package redis

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/nsyszr/admincenter/pkg/controlchannel/manager"
	"github.com/nsyszr/admincenter/pkg/util/rand"
)

// SessionManager reflects a Redis based session management
type SessionManager struct {
	redisClient *redis.Client
}

// NewSessionManager returns a Redis based SessionManager instance
func NewSessionManager(redisClient *redis.Client) *SessionManager {
	return &SessionManager{
		redisClient: redisClient,
	}
}

// ----------------------------------------------------------------------------
// SessionManager interface implementation
// ----------------------------------------------------------------------------

// GetSessionConfig returns the session config for the given realm
func (mgr *SessionManager) GetSessionConfig(realm string) (*manager.SessionConfig, error) {
	key := keyFromRealm(realm, "clients", "")
	if key == "" {
		return nil, manager.ErrInvalidRealm
	}

	exists, err := mgr.existsClientConfig(realm)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, manager.ErrNoSuchRealm
	}

	res, err := mgr.redisClient.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
		return nil, err
	}

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

	cfg := &manager.SessionConfig{
		SessionTimeout: sessionTimeout,
		PingInterval:   pingInterval,
		PongTimeout:    pongTimeout,
		EventsTopic:    res[3].(string),
	}

	return cfg, nil
}

// CreateSession tells the session manager to create a new session for the client
func (mgr *SessionManager) CreateSession(realm string, cfg *manager.SessionConfig) (int32, error) {
	for {
		sessID := rand.GenerateRandomInt32()
		key := fmt.Sprintf("sessions:%d", sessID)

		// HSETNX ensures that the key doesn't exists before it's added
		ok, err := mgr.redisClient.HSetNX(key, "realm", realm).Result()
		if err != nil {
			return 0, err
		}

		// We're added a new unique session key
		if ok {
			fields := make(map[string]interface{})
			fields["realm"] = realm
			fields["connected_since"] = time.Now().Unix()
			fields["last_message"] = time.Now().Unix()

			// Set all additional fields to the key
			_, err := mgr.redisClient.HMSet(key, fields).Result()
			if err != nil {
				// Remove the previously added key since we have an error
				mgr.redisClient.Del(key).Result()
				return 0, err
			}

			// Magic happens here! The key (session) exists only during the
			// specified session timeout. If the key expires, the session
			// is dead and the client has to restart the session.
			mgr.redisClient.Expire(key,
				time.Duration(cfg.SessionTimeout)*time.Second)

			return sessID, nil
		}
	}
}

// ExistsSession checks if a session for the given session ID exists
func (mgr *SessionManager) ExistsSession(sessionID int32) (bool, error) {
	key := fmt.Sprintf("sessions:%d", sessionID)
	res, err := mgr.redisClient.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

// ExistsSessionForRealm checks if a session for the given realm exists
func (mgr *SessionManager) ExistsSessionForRealm(realm string) (bool, error) {
	var cursor uint64

	for {
		var keys []string
		var err error
		keys, cursor, err = mgr.redisClient.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			res, err := mgr.redisClient.HGet(key, "realm").Result()
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

// RemoveSession tells the session manager to remove the session with given ID
func (mgr *SessionManager) RemoveSession(sessionID int32) error {
	exists, err := mgr.ExistsSession(sessionID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	key := fmt.Sprintf("sessions:%d", sessionID)
	_, err = mgr.redisClient.Del(key).Result()
	if err != nil {
		return err
	}

	return nil
}

// RenewSession tells the session manager to renew the session with given ID.
// This function updates the last message field with the current datetime.
func (mgr *SessionManager) RenewSession(sessionID int32) error {
	key := fmt.Sprintf("sessions:%d", sessionID)

	// Set last message unix time
	_, err := mgr.redisClient.HSet(key, "last_message", time.Now().Unix()).Result()
	if err != nil {
		return err
	}

	// Get the sessions realm
	realm, err := mgr.getSessionRealm(sessionID)
	if err != nil {
		return err
	}

	// Get the session config for the realm
	cfg, err := mgr.GetSessionConfig(realm)
	if err != nil {
		return err
	}

	// Reset expire time
	mgr.redisClient.Expire(key, time.Duration(cfg.SessionTimeout)*time.Second)

	return nil
}

// CleanupSessions removes all sessions from the session store
func (mgr *SessionManager) CleanupSessions() error {
	var cursor uint64

	for {
		var keys []string
		var err error
		keys, cursor, err = mgr.redisClient.Scan(cursor, "sessions:*", 1000).Result()
		if err != nil {
			return err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			_, err := mgr.redisClient.Del(key).Result()
			if err != nil {
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// Private functions
// ----------------------------------------------------------------------------

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

func (mgr *SessionManager) existsClientConfig(realm string) (bool, error) {
	key := keyFromRealm(realm, "clients", "")
	if key == "" {
		return false, nil
	}

	res, err := mgr.redisClient.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return (res == 1), nil
}

func (mgr *SessionManager) getSessionRealm(sessionID int32) (string, error) {
	exists, err := mgr.ExistsSession(sessionID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", manager.ErrNoSuchSession
	}

	key := fmt.Sprintf("sessions:%d", sessionID)
	res, err := mgr.redisClient.HGet(key, "realm").Result()
	if err != nil {
		return "", err
	}

	return res, err
}
