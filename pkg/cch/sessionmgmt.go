package cch

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

var errInvalidRealm = errors.New("sessionmgmt: invalid realm")
var errNoSuchRealm = errors.New("sessionmgmt: realm not found")
var errSessionExistsAlready = errors.New("sessionmgmt: session for realm already exists")

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
		return 0, errSessionExistsAlready
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

	values, err := c.db.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
		glog.Errorln("Failed to get client config:", err)
		return nil, err
	}

	glog.V(2).Infoln("Client config values: ", values)

	sessionTimeout, err := strconv.Atoi(values[0].(string))
	if err != nil {
		return nil, err
	}
	pingInterval, err := strconv.Atoi(values[1].(string))
	if err != nil {
		return nil, err
	}
	pongTimeout, err := strconv.Atoi(values[2].(string))
	if err != nil {
		return nil, err
	}

	cfg := &clientConfig{
		sessionTimeout: sessionTimeout,
		pingInterval:   pingInterval,
		pongTimeout:    pongTimeout,
		eventsTopic:    values[3].(string),
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

	val, err := c.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (val == 1), nil
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
			val, err := c.db.HGet(key, "realm").Result()
			if err != nil {
				return false, err
			}
			if val == realm {
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
			var values map[string]interface{}
			values = make(map[string]interface{})
			values["realm"] = realm
			values["connected_since"] = time.Now().Unix()

			// Set all additional fields to the key
			_, err := c.db.HMSet(sessionKey, values).Result()
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
	val, err := c.db.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (val == 1), nil
}
