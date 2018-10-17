package controlchannel

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-redis/redis"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/streadway/amqp"
)

const (
	messageTypeHello     int = 1
	messageTypeWelcome   int = 2
	messageTypeAbort     int = 3
	messageTypePing      int = 4
	messageTypePong      int = 5
	messageTypeError     int = 9
	messageTypeCall      int = 10
	messageTypeResult    int = 11
	messageTypePublish   int = 20
	messageTypePublished int = 21
)

var errInvalidRealm = errors.New("ERR_INVALID_REALM")
var errInvalidSession = errors.New("ERR_INVALID_SESSION")
var errNoSuchRealm = errors.New("ERR_NO_SUCH_REALM")
var errSessionExists = errors.New("ERR_SESSION_EXISTS")
var errProtocolViolation = errors.New("ERR_PROTOCOL_VIOLATION")
var errPublishFailed = errors.New("ERR_PUBLISH_FAILED")

// Session contains logic for a control channel session
type Session struct {
	conn         net.Conn
	realm        string
	id           int32
	startedAt    time.Time
	timeout      int
	ctrl         *Controller
	registered   bool
	registeredCh chan bool
	quitCh       chan bool
}

// Controller contains logic for managing control channel sessions
type Controller struct {
	redisDB  *redis.Client
	mu       sync.RWMutex
	sessions map[string]*Session
	ch       *amqp.Channel
}

type clientConfig struct {
	sessionTimeout int
	pingInterval   int
	pongTimeout    int
	eventsTopic    string
}

type publishMessage struct {
	messageType int
	requestID   int
	topic       string
	body        []byte
}

// NewSession returns an instance of control channel session
func NewSession(conn net.Conn, ctrl *Controller) *Session {
	return &Session{
		conn:         conn,
		ctrl:         ctrl,
		startedAt:    time.Now(),
		registeredCh: make(chan bool),
		quitCh:       make(chan bool),
	}
}

// NewController returns an instance of control channel controller
func NewController(redisDB *redis.Client, amqpConn *amqp.Connection) (*Controller, error) {
	// Setup a AMQP channel
	ch, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	// Create new controller
	ctrl := &Controller{
		redisDB:  redisDB,
		sessions: make(map[string]*Session), // Key is realm
		ch:       ch,
	}

	// Remove existing session entries
	ctrl.cleanupSessions()

	// Create and start RPC queue
	if err := ctrl.configureRPCQueue("rpc_queue"); err != nil {
		return nil, err
	}
	go ctrl.listenRPCQueue("rpc_queue")

	return ctrl, nil
}

// Close sends a quit signal and closes the network connection
func (sess *Session) Close() {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Debug("Closing session")

	// Unregister the session before closing the network connection
	sess.ctrl.unregisterSession(sess)

	// Send quit signal to all session relying routines
	sess.quitCh <- true

	// Close the network connection
	if sess.conn != nil {
		sess.conn.Close()
	}
}

func (sess *Session) ensureWelcomeMessage() {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Debug("Ensure welcome message timeout routine started")
	for {
		select {
		case <-sess.registeredCh:
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Debug("Ensure welcome message timeout routine stopped")
			return
		case <-sess.quitCh:
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Debug("Ensure welcome message timeout routine quitted")
			return
		case <-time.After(60 * time.Second): // TODO: get timeout from config
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Debug("Ensure welcome message timeout routine expired")
			// Close the session, since it's not registered within time
			sess.Close()
			return
		}
	}
}

func (sess *Session) listen() error {
	var (
		r       = wsutil.NewReader(sess.conn, ws.StateServerSide)
		w       = wsutil.NewWriter(sess.conn, ws.StateServerSide, ws.OpText)
		decoder = json.NewDecoder(r)
		// encoder = json.NewEncoder(w)
	)

	defer sess.Close()

	// Start listening for next frames
	for {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Wait for the next frame")

		hdr, err := r.NextFrame()
		if err != nil {
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Error("Failed to read the next frame:", err)
			return err
		}

		var msg []interface{}
		if err := decoder.Decode(&msg); err != nil {
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Error("Failed to decode request:", err)
			return err
		}

		debugJs, _ := json.Marshal(msg)
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "length": hdr.Length}).
			Debugf("Received message: %s", string(debugJs))

		// Update IO statistics
		sess.ctrl.updateIOStats(sess, 1, 0, int64(hdr.Length), 0)

		if err := sess.handleMessage(msg, w); err != nil {
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Error("Failed to handle request:", err)
			return err
		}

		if err := w.Flush(); err != nil {
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Error("Failed to flush response:", err)
			return err
		}
	}
}

func (sess *Session) writeMessage(w *wsutil.Writer, msg []interface{}) error {
	payload, err := json.Marshal(msg)
	// TODO: Add proper error handling
	if err != nil {
		return err
	}

	n, err := w.Write(payload)
	// TODO: Add proper error handling
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "length": n}).
		Debugf("Send JSON message: %s", string(payload))

	//log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "length": n}).
	//	Debugf("Send message: %v", msg)

	// Update IO statistics
	sess.ctrl.updateIOStats(sess, 0, 1, 0, int64(n))

	return nil

}

// AnyToFloat64 returns a float64 for given empty interface
func AnyToFloat64(v interface{}) (float64, error) {
	switch v.(type) {
	case float64:
		return v.(float64), nil
	default:
		return 0, fmt.Errorf("Type conversion failed")
	}
}

// AnyToString returns a string for given empty interface
func AnyToString(v interface{}) (string, error) {
	switch v.(type) {
	case string:
		return v.(string), nil
	default:
		return "", fmt.Errorf("Type conversion failed")
	}
}

// AnyToJSON returns a JSON byte array
func AnyToJSON(v interface{}) ([]byte, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("Type conversion failed")
	}
	return js, nil
}

func (sess *Session) handleMessage(msg []interface{}, w *wsutil.Writer) error {
	if len(msg) == 0 {
		return fmt.Errorf("empty message")
	}

	// Resolve the message type
	var msgType int
	v, err := AnyToFloat64(msg[0])
	if err != nil {
		return err
	}
	msgType = int(v)

	// We only accept hello message until the session isn't registered
	if msgType == messageTypeHello {
		return sess.handleHelloMessage(msg, w)
	}

	// If the session is not registered close the connection b/c it's not valid
	if !sess.registered {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid session")
		sess.Close()
		return fmt.Errorf("invalid session")
	}

	// Handle the incoming message
	switch msgType {
	case messageTypeHello:
		return sess.abort(w, errProtocolViolation, "After registration a new welcome message is not allowed.")
	case messageTypePing:
		return sess.handlePingMessage(msg, w)
	case messageTypePublish:
		return sess.handlePublishMessage(msg, w)
	}

	return nil
}

func (sess *Session) handleHelloMessage(msg []interface{}, w *wsutil.Writer) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Debug("Handle welcome message")

	if len(msg) < 2 || msg[1].(string) == "" {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("No or invalid realm given")
		//return resp.Encode(buildAbortMessage(errInvalidRealm, "No or invalid realm given"))
		return sess.writeMessage(w, buildAbortMessage(errInvalidRealm, "No or invalid realm given"))
	}

	sessID, cfg, ok, err := sess.ctrl.registerSession(msg[1].(string), sess)
	if !ok && err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Error("Failed to register new session:", err)
		return err
	}
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Failed to register new session:", err)
		// return resp.Encode(buildAbortMessage(err, "Failed to register new session"))
		return sess.writeMessage(w, buildAbortMessage(err, "Failed to register new session"))
	}

	return sess.writeMessage(w, buildWelcomeMessage(sessID, cfg.sessionTimeout,
		cfg.pingInterval, cfg.pongTimeout, cfg.eventsTopic))
}

func (sess *Session) abort(w *wsutil.Writer, err error, details string) error {
	//if err := resp.Encode(buildAbortMessage(err, details)); err != nil {
	if err := sess.writeMessage(w, buildAbortMessage(err, details)); err != nil {
		return err
	}

	// After abort message close the connection
	sess.Close()
	return nil
}
func (sess *Session) ensureRegistered(w *wsutil.Writer) (bool, error) {
	ok, err := sess.ctrl.existsSession(sess.id)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Error("Failed to check if session exists:", err)
		return false, err
	}
	if !ok {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Debug("Session does not exists")
		return false, sess.abort(w, errInvalidSession, "Invalid session")
	}

	if err := sess.ctrl.updateSession(sess); err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Error("Failed to update the session:", err)
		return false, err
	}

	return true, nil
}

func (sess *Session) handlePingMessage(msg []interface{}, w *wsutil.Writer) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("Handle ping message")

	ok, err := sess.ensureRegistered(w)
	if !ok {
		return err
	}

	// return resp.Encode(buildPongMessage())
	return sess.writeMessage(w, buildPongMessage())
}

func (sess *Session) handlePublishMessage(msg []interface{}, w *wsutil.Writer) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("Handle publish message")

	ok, err := sess.ensureRegistered(w)
	if !ok {
		return err
	}

	if len(msg) < 4 {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid publish message payload")
		// return resp.Encode(buildAbortMessage(errProtocolViolation, "Invalid publish message payload"))
		return sess.writeMessage(w, buildAbortMessage(errProtocolViolation, "Invalid publish message payload"))
	}

	publishMsg, err := parsePublishMessage(msg)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid publish message payload: ", err)
		// return resp.Encode(buildAbortMessage(errProtocolViolation, err.Error()))
		return sess.writeMessage(w, buildAbortMessage(errProtocolViolation, err.Error()))
	}

	publishID, err := sess.ctrl.publishMessage(publishMsg.topic, publishMsg.body)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Failed to publish message: ", err)
		// return resp.Encode(buildErrorMessage(messageTypePublish, publishMsg.requestID,
		//	errPublishFailed, err.Error()))
		return sess.writeMessage(w, buildErrorMessage(messageTypePublish,
			publishMsg.requestID, errPublishFailed, err.Error()))
	}

	// return resp.Encode(buildPublishedMessage(publishMsg.requestID, publishID))
	return sess.writeMessage(w, buildPublishedMessage(publishMsg.requestID, publishID))
}

func parsePublishMessage(msg []interface{}) (*publishMessage, error) {
	if len(msg) < 4 {
		return nil, fmt.Errorf("Invalid payload")
	}

	publishMsg := &publishMessage{}

	v, err := AnyToFloat64(msg[0])
	if err != nil {
		return nil, fmt.Errorf("Invalid message type field")
	}
	publishMsg.messageType = int(v)

	v, err = AnyToFloat64(msg[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid request ID field")
	}
	publishMsg.requestID = int(v)

	s, err := AnyToString(msg[2])
	if err != nil {
		return nil, fmt.Errorf("Invalid topic field")
	}
	publishMsg.topic = s

	js, err := AnyToJSON(msg[3])
	if err != nil {
		return nil, fmt.Errorf("Invalid topic field")
	}
	publishMsg.body = js

	return publishMsg, nil
}

func buildAbortMessage(reason error, details string) []interface{} {
	var msg []interface{}
	msg = append(msg, messageTypeAbort)
	msg = append(msg, reason.Error())
	msg = append(msg, abortMessageDetails{Message: details})
	return msg
}

func buildWelcomeMessage(sessionID int32, sessionTimeout, pingInterval, pongTimeout int, eventsTopic string) []interface{} {
	type details struct {
		SessionTimeout int    `json:"session_timeout"`
		PingInterval   int    `json:"ping_interval"`
		PongTimeout    int    `json:"pong_max_wait_time"`
		EventsTopic    string `json:"events_topic"`
	}

	var msg []interface{}
	msg = append(msg, messageTypeWelcome)
	msg = append(msg, sessionID)
	msg = append(msg, details{
		SessionTimeout: sessionTimeout,
		PingInterval:   pingInterval,
		PongTimeout:    pongTimeout,
		EventsTopic:    eventsTopic,
	})
	return msg
}

func buildPongMessage() []interface{} {
	type details struct{}
	var msg []interface{}
	msg = append(msg, messageTypePong)
	msg = append(msg, details{})
	return msg

}

// [ERROR, MessageType|integer, Request|id, Error|string, Details|dict]
func buildErrorMessage(msgType, requestID int, err error, message string) []interface{} {
	type details struct {
		Error string `json:"error"`
	}

	var msg []interface{}
	msg = append(msg, messageTypeError)
	msg = append(msg, msgType)
	msg = append(msg, requestID)
	msg = append(msg, err.Error())
	msg = append(msg, details{Error: message})
	return msg
}

// [CALL, Request|id, Operation|string, Arguments|dict]
func buildCallMessage(requestID int, operation string, args interface{}) []interface{} {
	var msg []interface{}
	msg = append(msg, messageTypeCall)
	msg = append(msg, requestID)
	msg = append(msg, operation)
	msg = append(msg, args)
	return msg
}

// [PUBLISHED, Request|id, Publication|id]
func buildPublishedMessage(requestID, publicationID int) []interface{} {
	var msg []interface{}
	msg = append(msg, messageTypePublished)
	msg = append(msg, requestID)
	msg = append(msg, publicationID)
	return msg
}

// registerSession registers a new websocket session
func (ctrl *Controller) registerSession(realm string, sess *Session) (int32, *clientConfig, bool, error) {
	cfg, ok, err := ctrl.getClientConfig(realm)
	if err != nil {
		return 0, nil, ok, err
	}

	exists, err := ctrl.existsSessionForRealm(realm)
	if err != nil {
		return 0, nil, false, err
	}
	if exists {
		return 0, nil, true, errSessionExists
	}

	sessionID, err := ctrl.createSession(realm, cfg)
	if err != nil {
		return 0, nil, false, err
	}

	// Add session to session map
	// Tell the sesssion that it's registered
	// TODO: rethink the updates of the session in this way
	sess.realm = realm
	sess.id = sessionID
	sess.timeout = cfg.sessionTimeout
	sess.registeredCh <- true
	sess.registered = true
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("New session registered")

	ctrl.mu.Lock()
	ctrl.sessions[realm] = sess
	ctrl.mu.Unlock()

	return sessionID, cfg, true, nil
}

func (ctrl *Controller) unregisterSession(sess *Session) {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("Unregistering session")

	if sess.realm == "" {
		return
	}

	// Remove the session from the session map
	ctrl.mu.Lock()
	delete(ctrl.sessions, sess.realm)
	ctrl.mu.Unlock()

	exists, err := ctrl.existsSessionForRealm(sess.realm)
	if err != nil {
		return
	}
	if !exists {
		return
	}

	// Remove the session from the database to ensure that the client can
	// reconnect again immediately.
	sessionKey := fmt.Sprintf("sessions:%d", sess.id)
	_, err = ctrl.redisDB.Del(sessionKey).Result()
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Error("Failed to remove session: ", err)
	}
}

func (ctrl *Controller) getClientConfig(realm string) (*clientConfig, bool, error) {
	key := keyFromRealm(realm, "clients", "")
	if key == "" {
		return nil, true, errInvalidRealm
	}

	exists, err := ctrl.existsClientConfig(realm)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, true, errNoSuchRealm
	}

	res, err := ctrl.redisDB.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
		log.Error("Failed to get client config:", err)
		return nil, false, err
	}

	log.Debug("Client config values:", res)

	sessionTimeout, err := strconv.Atoi(res[0].(string))
	if err != nil {
		return nil, false, err
	}
	pingInterval, err := strconv.Atoi(res[1].(string))
	if err != nil {
		return nil, false, err
	}
	pongTimeout, err := strconv.Atoi(res[2].(string))
	if err != nil {
		return nil, false, err
	}

	cfg := &clientConfig{
		sessionTimeout: sessionTimeout,
		pingInterval:   pingInterval,
		pongTimeout:    pongTimeout,
		eventsTopic:    res[3].(string),
	}

	return cfg, true, nil
}

func keyFromRealm(realm, prefix, suffix string) string {
	s := strings.SplitN(realm, "@", 2)
	if len(s) == 2 {
		key := fmt.Sprintf("%s:%s:%s", prefix, s[1], s[0])
		if suffix != "" {
			key = fmt.Sprintf("%s:%s", key, suffix)
		}
		return key
	}

	return ""
}

func (ctrl *Controller) existsClientConfig(realm string) (bool, error) {
	key := keyFromRealm(realm, "clients", "")
	if key == "" {
		return false, nil
	}

	res, err := ctrl.redisDB.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (ctrl *Controller) existsSessionForRealm(realm string) (bool, error) {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = ctrl.redisDB.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			res, err := ctrl.redisDB.HGet(key, "realm").Result()
			if err != nil {
				return false, err
			}
			if res == realm {
				return true, nil
			}
		}

		if cursor == 0 {
			break
		}
	}

	return false, nil
}

func (ctrl *Controller) createSession(realm string, cfg *clientConfig) (int32, error) {
	for {
		sessionID := generateRandomSessionID()
		sessionKey := fmt.Sprintf("sessions:%d", sessionID)

		// HSETNX ensures that the key doesn't exists before it's added
		success, err := ctrl.redisDB.HSetNX(sessionKey, "realm", realm).Result()
		if err != nil {
			return 0, err
		}

		// We're added a new unique session key
		if success {
			fields := make(map[string]interface{})
			fields["realm"] = realm
			fields["connected_since"] = time.Now().Unix()
			fields["last_message"] = time.Now().Unix()

			// Set all additional fields to the key
			_, err := ctrl.redisDB.HMSet(sessionKey, fields).Result()
			if err != nil {
				ctrl.redisDB.Del(sessionKey).Result()
				return 0, err
			}

			// Magic happens here! The key (session) exists only during the
			// specified session timeout. If the key expires, the session
			// is dead and the client has to restart the session.
			ctrl.redisDB.Expire(sessionKey,
				time.Duration(cfg.sessionTimeout)*time.Second)

			return sessionID, nil
		}
	}
}

func generateRandomSessionID() int32 {
	rand.Seed(time.Now().UnixNano())
	return 1 + rand.Int31()
}

// existsSession returns if a session with the given ID exists
func (ctrl *Controller) existsSession(id int32) (bool, error) {
	key := fmt.Sprintf("sessions:%d", id)
	res, err := ctrl.redisDB.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (ctrl *Controller) updateSession(sess *Session) error {
	sessionKey := fmt.Sprintf("sessions:%d", sess.id)

	// Set last message unix time
	_, err := ctrl.redisDB.HSet(sessionKey, "last_message", time.Now().Unix()).Result()
	if err != nil {
		return err
	}

	// Reset expire time
	ctrl.redisDB.Expire(sessionKey, time.Duration(sess.timeout)*time.Second)

	return nil
}

// Close cleans up a controller
func (ctrl *Controller) Close() {
	ctrl.cleanupSessions()
	ctrl.ch.Close()
}

func (ctrl *Controller) cleanupSessions() {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = ctrl.redisDB.Scan(cursor, "sessions:*", 1000).Result()
		if err != nil {
			log.Error("Failed to scan session keys: ", err)
			return
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			_, err := ctrl.redisDB.Del(key).Result()
			if err != nil {
				log.Error("Failed to delete session key: ", err)
			}
		}

		if cursor == 0 {
			break
		}
	}
}

func (ctrl *Controller) publishMessage(topic string, body []byte) (int, error) {
	if err := ctrl.ch.ExchangeDeclare(
		topic,    // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	); err != nil {
		return 0, err
	}

	if err := ctrl.ch.Publish(
		topic, // exchange
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		}); err != nil {
		return 0, err
	}

	// TODO: get a message ID during publish and return it
	return 1234, nil
}

func (ctrl *Controller) updateIOStats(sess *Session, rxMsgs, txMsgs, rxBytes, txBytes int64) error {
	// key := keyFromRealm(realm, "stats", "io")
	key := fmt.Sprintf("stats:%s:io", sess.conn.RemoteAddr())

	keys, err := ctrl.redisDB.Keys(key).Result()
	if err != nil {
		log.Error("Failed to update stats: ", err)
		return err
	}

	// Stats entry for realm doesnt exists. Add a new one.
	if len(keys) == 0 {
		fields := make(map[string]interface{})
		fields["realm"] = ""
		fields["rx_msgs"] = 0
		fields["tx_msgs"] = 0
		fields["rx_bytes"] = 0
		fields["tx_bytes"] = 0

		_, err := ctrl.redisDB.HMSet(key, fields).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
			return err
		}
	}

	// Update realm
	if sess.realm != "" {
		_, err := ctrl.redisDB.HSet(key, "realm", sess.realm).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	// Update stats
	if rxMsgs != 0 {
		_, err := ctrl.redisDB.HIncrBy(key, "rx_msgs", rxMsgs).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if txMsgs != 0 {
		_, err := ctrl.redisDB.HIncrBy(key, "tx_msgs", txMsgs).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if rxBytes != 0 {
		_, err := ctrl.redisDB.HIncrBy(key, "rx_bytes", rxBytes).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if txBytes != 0 {
		_, err := ctrl.redisDB.HIncrBy(key, "tx_bytes", txBytes).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	return nil
}

func (ctrl *Controller) configureRPCQueue(rpcQueueName string) error {
	_, err := ctrl.ch.QueueDeclare(
		rpcQueueName, // name
		false,        // durable
		false,        // delete when usused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	err = ctrl.ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return err
	}

	return nil
}

func (ctrl *Controller) listenRPCQueue(rpcQueueName string) error {
	type rpcRequest struct {
		Realm     string      `json:"realm"`
		Operation string      `json:"operation"`
		Arguments interface{} `json:"args"`
	}

	type rpcError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	type rpcResponse struct {
		Error  rpcError    `json:"error,omitempty"`
		Result interface{} `json:"result,omitempty"`
	}

	if rpcQueueName == "" {
		// TODO: Better error handling
		return fmt.Errorf("No rpc queue name set")
	}

	msgs, err := ctrl.ch.Consume(
		rpcQueueName, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	for msg := range msgs {

		req := rpcRequest{}
		resp := rpcResponse{Error: rpcError{}}
		hasError := false

		if err := json.Unmarshal(msg.Body, &req); err != nil {
			resp.Error.Code = 1000
			resp.Error.Message = err.Error()
			hasError = true
		}

		//n, _ := strconv.Atoi(string(msg.Body))
		// failOnError(err, "Failed to convert body to integer")
		if !hasError {
			log.Printf("RPC request received: ", req)
			// response := 1 // fib(n)
			exists, err := ctrl.existsSessionForRealm(req.Realm)
			if err != nil {
				hasError = true
				resp.Error.Code = 1001
				resp.Error.Message = err.Error()
			} else if !exists {
				hasError = true
				resp.Error.Code = 2000
				resp.Error.Message = "No session for given devices realm"
			} else {
				ctrl.mu.Lock()
				sess := ctrl.sessions[req.Realm]
				ctrl.mu.Unlock()

				w := wsutil.NewWriter(sess.conn, ws.StateServerSide, ws.OpText)
				if err := sess.writeMessage(w, buildCallMessage(1234, req.Operation, req.Arguments)); err != nil {
					log.Printf("listen to rpc error: %s", err.Error())
				}
				if err := w.Flush(); err != nil {
					log.Printf("listen to rpc error: %s", err.Error())
				}
			}
		} else {
			log.Printf("Error: ", resp.Error.Message)
		}

		js, _ := json.Marshal(resp)

		err = ctrl.ch.Publish(
			"",          // exchange
			msg.ReplyTo, // routing key
			false,       // mandatory
			false,       // immediate
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: msg.CorrelationId,
				Body:          js, // []byte(strconv.Itoa(response)),
			})
		// failOnError(err, "Failed to publish a message")

		msg.Ack(false)
	}

	return nil
}
