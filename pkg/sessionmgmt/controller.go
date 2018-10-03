package sessionmgmt

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var ErrInvalidRealm = errors.New("sessionmgmt: invalid realm")
var ErrNoSuchRealm = errors.New("sessionmgmt: realm not found")
var ErrSessionExistsAlready = errors.New("sessionmgmt: session for realm already exists")

// Controller handles the websocket client sessions
type Controller struct {
	redisClient *redis.Client
}

type clientConfig struct {
	sessionTimeout int
	pingInterval   int
	pongTimeout    int
	eventsTopic    string
}

// NewController creates a new session controller instance
func NewController(redisClient *redis.Client) *Controller {
	return &Controller{
		redisClient: redisClient,
	}
}

// RegisterSession registers a new websocket session
func (c *Controller) RegisterSession(conn *websocket.Conn, realm string) (int32, error) {
	cfg, err := c.getClientConfig(realm)
	if err != nil {
		return 0, err
	}

	exists, err := c.existsSessionForRealm(realm)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrSessionExistsAlready
	}

	sessionID, err := c.createSession(realm, cfg)
	if err != nil {
		return 0, err
	}

	return sessionID, nil
}

func (c *Controller) getClientConfig(realm string) (*clientConfig, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return nil, ErrInvalidRealm
	}

	exists, err := c.existsClientConfig(realm)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNoSuchRealm
	}

	values, err := c.redisClient.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
		log.Println("getClientConfig error: ", err)
		return nil, err
	}

	log.Println("getClientConfig values: ", values)

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

func (c *Controller) existsClientConfig(realm string) (bool, error) {
	key := clientKeyFromRealm(realm)
	if key == "" {
		return false, ErrInvalidRealm
	}

	val, err := c.redisClient.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (val == 1), nil
}

func (c *Controller) existsSessionForRealm(realm string) (bool, error) {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = c.redisClient.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			val, err := c.redisClient.HGet(key, "realm").Result()
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

func (c *Controller) createSession(realm string, cfg *clientConfig) (int32, error) {
	for {
		sessionID := generateRandomSessionID()
		sessionKey := fmt.Sprintf("sessions:%d", sessionID)

		// HSETNX ensures that the key doesn't exists before it's added
		success, err := c.redisClient.HSetNX(sessionKey, "realm", realm).Result()
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
			_, err := c.redisClient.HMSet(sessionKey, values).Result()
			if err != nil {
				c.redisClient.Del(sessionKey).Result()
				return 0, err
			}

			// Magic happens here! The key (session) exists only during the
			// specified session timeout. If the key expires, the session
			// is dead and the client has to restart the session.
			c.redisClient.Expire(sessionKey,
				time.Duration(cfg.sessionTimeout)*time.Second)

			return sessionID, nil
		}
	}
}

func generateRandomSessionID() int32 {
	rand.Seed(time.Now().UnixNano())
	return 1 + rand.Int31()
}

// ExistsSession returns if a session with given ID exists
func (c *Controller) ExistsSession(id int32) (bool, error) {
	key := fmt.Sprintf("sessions:%d", id)
	val, err := c.redisClient.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (val == 1), nil
}
