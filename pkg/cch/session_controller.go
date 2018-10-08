package cch

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var errInvalidRealm = errors.New("ERR_INVALID_REALM")
var errNoSuchRealm = errors.New("ERR_NO_SUCH_REALM")
var errSessionExists = errors.New("ERR_SESSION_EXISTS")

// sessionController handles the websocket client sessions
type sessionController struct {
	db *redis.Client
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
		db: db,
		// sessions:    make(map[string]*session),
	}
}

// registerSession registers a new websocket session
func (c *sessionController) registerSession(conn *websocket.Conn, realm string) (int32, error) {
	cfg, err := c.getClientConfig(realm)
	if err != nil {
		return 0, err
	}

	exists, err := c.existsSessionForRealm(realm)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errSessionExists
	}

	sessionID, err := c.createSession(realm, cfg)
	if err != nil {
		return 0, err
	}

	return sessionID, nil
}

func (c *sessionController) getClientConfig(realm string) (*clientConfig, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return nil, errInvalidRealm
	}

	exists, err := c.existsClientConfig(realm)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errNoSuchRealm
	}

	res, err := c.db.HMGet(key, "session_timeout", "ping_interval",
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

func (c *sessionController) existsClientConfig(realm string) (bool, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return false, errInvalidRealm
	}

	res, err := c.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (c *sessionController) existsSessionForRealm(realm string) (bool, error) {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = c.db.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			res, err := c.db.HGet(key, "realm").Result()
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

func (c *sessionController) createSession(realm string, cfg *clientConfig) (int32, error) {
	for {
		sessionID := generateRandomSessionID()
		sessionKey := fmt.Sprintf("sessions:%d", sessionID)

		// HSETNX ensures that the key doesn't exists before it's added
		success, err := c.db.HSetNX(sessionKey, "realm", realm).Result()
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
			_, err := c.db.HMSet(sessionKey, fields).Result()
			if err != nil {
				c.db.Del(sessionKey).Result()
				return 0, err
			}

			// Magic happens here! The key (session) exists only during the
			// specified session timeout. If the key expires, the session
			// is dead and the client has to restart the session.
			c.db.Expire(sessionKey,
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
func (c *sessionController) existsSession(id int32) (bool, error) {
	key := fmt.Sprintf("sessions:%d", id)
	res, err := c.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (c *sessionController) updateSession(sessionID int32, incrMsgsSendBy, incrMsgsRcvdBy int64) error {
	sessionKey := fmt.Sprintf("sessions:%d", sessionID)

	// Fetch session timeout
	s, err := c.db.HGet(sessionKey, "session_timeout").Result()
	if err != nil {
		return err
	}
	sessionTimeout, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	// Increment the message counters
	_, err = c.db.HIncrBy(sessionKey, "msgs_send", incrMsgsSendBy).Result()
	if err != nil {
		return err
	}
	_, err = c.db.HIncrBy(sessionKey, "msgs_rcvd", incrMsgsRcvdBy).Result()
	if err != nil {
		return err
	}

	// Reset expire time
	c.db.Expire(sessionKey, time.Duration(sessionTimeout)*time.Second)

	return nil
}
