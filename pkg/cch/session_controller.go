package cch

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var errInvalidRealm = errors.New("ERR_INVALID_REALM")
var errNoSuchRealm = errors.New("ERR_NO_SUCH_REALM")
var errSessionExists = errors.New("ERR_SESSION_EXISTS")

// sessionController handles the websocket client sessions
type sessionController struct {
	// TODO: Abstract db to interface
	db       *redis.Client
	sessions map[*websocket.Conn]int32
	// sessions map[string]*session
}

type clientConfig struct {
	sessionTimeout int
	pingInterval   int
	pongTimeout    int
	eventsTopic    string
}

// newSessionController creates a new session controller instance
func newSessionController(db *redis.Client) *sessionController {
	return &sessionController{
		db:       db,
		sessions: make(map[*websocket.Conn]int32),
		// sessions:    make(map[string]*session),
	}
}

func (ctrl *sessionController) handleSession(w http.ResponseWriter, r *http.Request) (*session, error) {
	/*var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}*/
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
			Error("Failed to upgrade HTTP websocket:", err)
		return nil, err
	}

	// TODO: Create a unique session ID, even if the session is not registered.
	//       This is interessting for Prometheus statistics! Add expires at
	//       to the session Redis key. If the session wont be registered, it
	//       disappears from the Prometheus statistics.
	// NOTES: Rethink that idea! Create seperate stats for initiated connections,
	//        for dropped onces, etc. A session should be only established
	//		  if both parties agree to each other.

	sess := &session{
		conn:         conn,
		ctrl:         ctrl,
		startedAt:    time.Now(),
		registeredCh: make(chan bool, 1),
	}

	log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
		Info("Control channel session started")

	// Start a background timer to invalidate session if not registered within
	// a specified time period.
	go func(sess *session) {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Registration timeout routine started")
		for {
			select {
			case <-sess.registeredCh:
				log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
					Debug("Registration timeout routine stopped")
				return
			case <-time.After(60 * time.Second): // TODO: get timeout from config
				log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
					Debug("Registration timeout routine expired")
				return
			}
		}
	}(sess)

	// Start the websocket session
	go func() {
		defer conn.Close()

		var (
			r       = wsutil.NewReader(conn, ws.StateServerSide)
			w       = wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
			decoder = json.NewDecoder(r)
			//encoder = json.NewEncoder(w)
		)
		for {
			if _, err = r.NextFrame(); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to read the next frame:", err)
				return
			}

			var req []interface{}
			if err := decoder.Decode(&req); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to decode request:", err)
				return
			}

			log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
				Debugf("Received message: %v", req)

			if err = w.Flush(); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to flush response:", err)
				return
			}
		}
	}()

	return sess, nil
}

// registerSession registers a new websocket session
func (ctrl *sessionController) registerSession(conn *websocket.Conn, realm string) (int32, error) {
	cfg, err := ctrl.getClientConfig(realm)
	if err != nil {
		return 0, err
	}

	exists, err := ctrl.existsSessionForRealm(realm)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errSessionExists
	}

	sessionID, err := ctrl.createSession(realm, cfg)
	if err != nil {
		return 0, err
	}

	return sessionID, nil
}

func (ctrl *sessionController) getClientConfig(realm string) (*clientConfig, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return nil, errInvalidRealm
	}

	exists, err := ctrl.existsClientConfig(realm)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errNoSuchRealm
	}

	res, err := ctrl.db.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
		log.Error("Failed to get client config:", err)
		return nil, err
	}

	log.Debug("Client config values:", res)

	sessionTimeout, err := strconv.Atoi(res[0].(string))
	if err != nil {
		return nil, err
	}
	pingInterval, err := strconv.Atoi(res[1].(string))
	if err != nil {
		return nil, err
	}
	pongTimeout, err := strconv.Atoi(res[2].(string))
	if err != nil {
		return nil, err
	}

	cfg := &clientConfig{
		sessionTimeout: sessionTimeout,
		pingInterval:   pingInterval,
		pongTimeout:    pongTimeout,
		eventsTopic:    res[3].(string),
	}

	return cfg, nil
}

func clientKeyFromRealm(realm string) string {
	s := strings.SplitN(realm, "@", 2)
	if len(s) == 2 {
		return fmt.Sprintf("clients:%s:%s", s[1], s[0])
	}

	return ""
}

func (ctrl *sessionController) existsClientConfig(realm string) (bool, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return false, errInvalidRealm
	}

	res, err := ctrl.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (ctrl *sessionController) existsSessionForRealm(realm string) (bool, error) {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = ctrl.db.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			res, err := ctrl.db.HGet(key, "realm").Result()
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

func (ctrl *sessionController) createSession(realm string, cfg *clientConfig) (int32, error) {
	for {
		sessionID := generateRandomSessionID()
		sessionKey := fmt.Sprintf("sessions:%d", sessionID)

		// HSETNX ensures that the key doesn't exists before it's added
		success, err := ctrl.db.HSetNX(sessionKey, "realm", realm).Result()
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
			_, err := ctrl.db.HMSet(sessionKey, fields).Result()
			if err != nil {
				ctrl.db.Del(sessionKey).Result()
				return 0, err
			}

			// Magic happens here! The key (session) exists only during the
			// specified session timeout. If the key expires, the session
			// is dead and the client has to restart the session.
			ctrl.db.Expire(sessionKey,
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
func (ctrl *sessionController) existsSession(id int32) (bool, error) {
	key := fmt.Sprintf("sessions:%d", id)
	res, err := ctrl.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (ctrl *sessionController) updateSession(sessionID int32, incrMsgsSendBy, incrMsgsRcvdBy int64) error {
	sessionKey := fmt.Sprintf("sessions:%d", sessionID)

	// Fetch session timeout
	s, err := ctrl.db.HGet(sessionKey, "session_timeout").Result()
	if err != nil {
		return err
	}
	sessionTimeout, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	// Increment the message counters
	_, err = ctrl.db.HIncrBy(sessionKey, "msgs_send", incrMsgsSendBy).Result()
	if err != nil {
		return err
	}
	_, err = ctrl.db.HIncrBy(sessionKey, "msgs_rcvd", incrMsgsRcvdBy).Result()
	if err != nil {
		return err
	}

	// Reset expire time
	ctrl.db.Expire(sessionKey, time.Duration(sessionTimeout)*time.Second)

	return nil
}
