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
		encoder = json.NewEncoder(w)
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

		var req []interface{}
		if err := decoder.Decode(&req); err != nil {
			log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
				Error("Failed to decode request:", err)
			return err
		}

		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "length": hdr.Length}).
			Debugf("Received request: %v", req)

		if err := sess.handleRequest(req, encoder); err != nil {
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

func (sess *Session) handleRequest(req []interface{}, resp *json.Encoder) error {
	if len(req) == 0 {
		return fmt.Errorf("empty message")
	}

	var messageType int
	/*switch req[0].(type) {
	case float64:
		messageType = int(req[0].(float64))
	default:
		return fmt.Errorf("invalid message type")
	}*/
	v, err := AnyToFloat64(req[0])
	if err != nil {
		return err
	}
	messageType = int(v)

	// We only accept hello message until the session isn't registered
	if messageType == messageTypeHello {
		return sess.handleHelloMessage(req, resp)
	}

	// If the session is not registered close the connection b/c it's not valid
	if !sess.registered {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid session")
		sess.Close()
		return fmt.Errorf("invalid session")
	}

	switch messageType {
	case messageTypeHello:
		return sess.abort(resp, errProtocolViolation, "After registration a new welcome message is not allowed.")
	case messageTypePing:
		return sess.handlePingMessage(req, resp)
	case messageTypePublish:
		return sess.handlePublishMessage(req, resp)
	}

	return nil
}

func (sess *Session) handleHelloMessage(req []interface{}, resp *json.Encoder) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Debug("Handle welcome message")

	if len(req) < 2 || req[1].(string) == "" {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("No or invalid realm given")
		return resp.Encode(buildAbortMessage(errInvalidRealm, "No or invalid realm given"))
	}

	sessID, cfg, ok, err := sess.ctrl.registerSession(req[1].(string), sess)
	if !ok && err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Error("Failed to register new session:", err)
		return err
	}
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Failed to register new session:", err)
		return resp.Encode(buildAbortMessage(err, "Failed to register new session"))
	}

	return resp.Encode(buildWelcomeMessage(sessID, cfg.sessionTimeout, cfg.pingInterval,
		cfg.pongTimeout, cfg.eventsTopic))
}

func (sess *Session) abort(resp *json.Encoder, err error, details string) error {
	if err := resp.Encode(buildAbortMessage(err, details)); err != nil {
		return err
	}

	// After abort message close the connection
	sess.Close()
	return nil
}
func (sess *Session) ensureRegistered(resp *json.Encoder) (bool, error) {
	ok, err := sess.ctrl.existsSession(sess.id)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Error("Failed to check if session exists:", err)
		return false, err
	}
	if !ok {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Debug("Session does not exists")
		return false, sess.abort(resp, errInvalidSession, "Invalid session")
	}

	if err := sess.ctrl.updateSession(sess.id, 1, 1); err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
			Error("Failed to update the session:", err)
		return false, err
	}

	return true, nil
}

func (sess *Session) handlePingMessage(req []interface{}, resp *json.Encoder) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("Handle ping message")

	ok, err := sess.ensureRegistered(resp)
	if !ok {
		return err
	}

	return resp.Encode(buildPongMessage())
}

func (sess *Session) handlePublishMessage(req []interface{}, resp *json.Encoder) error {
	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr(), "realm": sess.realm}).
		Debug("Handle publish message")

	ok, err := sess.ensureRegistered(resp)
	if !ok {
		return err
	}

	if len(req) < 4 {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid publish message payload")
		return resp.Encode(buildAbortMessage(errProtocolViolation, "Invalid publish message payload"))
	}

	msg, err := parsePublishMessage(req)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Invalid publish message payload: ", err)
		return resp.Encode(buildAbortMessage(errProtocolViolation, err.Error()))
	}

	pubID, err := sess.ctrl.publishMessage(msg.topic, msg.body)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Failed to publish message: ", err)
		return resp.Encode(buildErrorMessage(messageTypePublish, msg.requestID,
			errPublishFailed, err.Error()))
	}

	return resp.Encode(buildPublishedMessage(msg.requestID, pubID))
}

func parsePublishMessage(req []interface{}) (*publishMessage, error) {
	if len(req) < 4 {
		return nil, fmt.Errorf("Invalid payload")
	}

	msg := &publishMessage{}

	v, err := AnyToFloat64(req[0])
	if err != nil {
		return nil, fmt.Errorf("Invalid message type field")
	}
	msg.messageType = int(v)

	v, err = AnyToFloat64(req[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid request ID field")
	}
	msg.requestID = int(v)

	s, err := AnyToString(req[2])
	if err != nil {
		return nil, fmt.Errorf("Invalid topic field")
	}
	msg.topic = s

	js, err := AnyToJSON(req[3])
	if err != nil {
		return nil, fmt.Errorf("Invalid topic field")
	}
	msg.body = js

	return msg, nil
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
func buildErrorMessage(messageType, requestID int, err error, message string) []interface{} {
	type details struct {
		Error string `json:"error"`
	}

	var msg []interface{}
	msg = append(msg, messageTypeError)
	msg = append(msg, messageType)
	msg = append(msg, requestID)
	msg = append(msg, err.Error())
	msg = append(msg, details{Error: message})
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
	key := clientKeyFromRealm(realm)
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

func clientKeyFromRealm(realm string) string {
	s := strings.SplitN(realm, "@", 2)
	if len(s) == 2 {
		return fmt.Sprintf("clients:%s:%s", s[1], s[0])
	}

	return ""
}

func (ctrl *Controller) existsClientConfig(realm string) (bool, error) {
	key := clientKeyFromRealm(realm)
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
			fields["session_timeout"] = cfg.sessionTimeout
			fields["connected_since"] = time.Now().Unix()
			fields["msgs_send"] = 1
			fields["msgs_rcvd"] = 1

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

func (ctrl *Controller) updateSession(sessionID int32, incrMsgsSendBy, incrMsgsRcvdBy int64) error {
	sessionKey := fmt.Sprintf("sessions:%d", sessionID)

	// Fetch session timeout
	s, err := ctrl.redisDB.HGet(sessionKey, "session_timeout").Result()
	if err != nil {
		return err
	}
	sessionTimeout, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	// Increment the message counters
	_, err = ctrl.redisDB.HIncrBy(sessionKey, "msgs_send", incrMsgsSendBy).Result()
	if err != nil {
		return err
	}
	_, err = ctrl.redisDB.HIncrBy(sessionKey, "msgs_rcvd", incrMsgsRcvdBy).Result()
	if err != nil {
		return err
	}

	// Reset expire time
	ctrl.redisDB.Expire(sessionKey, time.Duration(sessionTimeout)*time.Second)

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
