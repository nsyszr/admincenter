package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/streadway/amqp"
)

type clientConfig struct {
	sessionTimeout int
	pingInterval   int
	pongTimeout    int
	eventsTopic    string
}

type rpcRequest struct {
	Realm     string      `json:"realm"`
	Operation string      `json:"operation"`
	Arguments interface{} `json:"args"`
}

type rpcArgumentsM3CLI struct {
	Command string `json:"command"`
}

type rpcResultsM3CLI struct {
	Output []string `json:"output"`
}

func (s *Server) handleGetSessionsActive() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		/*err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			respondWithError(w, 1000, err.Error(), http.StatusInternalServerError)
			return
		}*/

		/*
				res, err := ctrl.redisDB.HMGet(key, "session_timeout", "ping_interval",
				"pong_max_wait_time", "events_topic").Result()
			if err != nil {
				log.Error("Failed to get client config:", err)
				return nil, false, err
			}*/

		var err error
		resp := &ActiveSessions{}

		resp.Active, err = s.scanSessions()
		if err != nil {
			respondWithError(w, 100, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, sess := range resp.Active {
			cfg, err := s.getClientConfig(sess.Realm)
			if err != nil {
				respondWithError(w, 101, err.Error(), http.StatusInternalServerError)
				return
			}

			sess.SessionTimeout = cfg.sessionTimeout
			sess.PingInterval = cfg.pingInterval
			sess.PongTimeout = cfg.pongTimeout
		}

		resp.Total = len(resp.Active)

		respondWithJSON(w, resp, http.StatusOK)
	}
}

// TODO: We need a DAO for the Redis database
func (s *Server) scanSessions() ([]*SessionDetails, error) {
	sessions := []*SessionDetails{}

	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = s.RedisClient.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return nil, err
		}

		// Fetch values
		for _, key := range keys {
			res, err := s.RedisClient.HMGet(key, "realm", "connected_since", "last_message").Result()
			if err != nil {
				return nil, err
			}

			sess := &SessionDetails{}
			sess.Realm = res[0].(string)

			connectedSinceUnix, err := strconv.ParseInt(res[1].(string), 10, 64)
			if err != nil {
				return nil, err
			}
			sess.ConnectedSince = time.Unix(connectedSinceUnix, 0)

			lastMessageUnix, err := strconv.ParseInt(res[2].(string), 10, 64)
			if err != nil {
				return nil, err
			}
			sess.LastMessage = time.Unix(lastMessageUnix, 0)

			sessions = append(sessions, sess)
		}

		if cursor == 0 {
			break
		}
	}

	return sessions, nil
}

// TODO: Duplicate code, see controlchannel
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

// TODO: Duplicate code, see controlchannel
func (s *Server) getClientConfig(realm string) (*clientConfig, error) {
	key := keyFromRealm(realm, "clients", "")
	if key == "" {
		return nil, fmt.Errorf("Invalid realm")
	}

	/*exists, err := ctrl.existsClientConfig(realm)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, true, errNoSuchRealm
	}*/

	res, err := s.RedisClient.HMGet(key, "session_timeout", "ping_interval",
		"pong_max_wait_time", "events_topic").Result()
	if err != nil {
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

func (s *Server) handlePostSessionsRun() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionRunRequest := SessionRunRequest{}
		err := json.NewDecoder(r.Body).Decode(&sessionRunRequest)
		if err != nil {
			respondWithError(w, 1000, err.Error(), http.StatusInternalServerError)
			return
		}

		sessionRunResponse := SessionRunResponse{
			Realms: make(map[string]SessionRunRealmResponse),
		}

		if len(sessionRunRequest.Realms) == 0 {
			respondWithJSON(w, sessionRunResponse, http.StatusOK)
			return
		}

		for _, realm := range sessionRunRequest.Realms {
			exists, err := s.existsSessionForRealm(realm)
			if err != nil {
				respondWithError(w, 1000, err.Error(), http.StatusInternalServerError)
				return
			}

			if exists {
				result := SessionRunRealmResponse{
					Success: false,
				}

				rpcResult, err := s.rpcM3CLI(realm, sessionRunRequest.Command)
				if err != nil {
					result.Error = &ErrorResponse{
						Code:    3000,
						Message: err.Error(),
					}
				} else {
					result.Success = true
					if rpcResult != nil {
						result.Results = rpcResult
					}
				}
				sessionRunResponse.Realms[realm] = result
			} else {
				result := SessionRunRealmResponse{
					Success: false,
					Error: &ErrorResponse{
						Code:    2000,
						Message: "No session",
					},
				}
				sessionRunResponse.Realms[realm] = result
			}
		}

		respondWithJSON(w, sessionRunResponse, http.StatusOK)
	}
}

// TODO: Duplicate code, see controlchannel
func (s *Server) existsSessionForRealm(realm string) (bool, error) {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = s.RedisClient.Scan(cursor, "sessions*", 1000).Result()
		if err != nil {
			return false, err
		}

		// Fetch each realm field of returend keys
		for _, key := range keys {
			res, err := s.RedisClient.HGet(key, "realm").Result()
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

func (s *Server) rpcM3CLI(realm, cmd string) (*rpcResultsM3CLI, error) {
	ch, err := s.AMQPConn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	corrID := generateRandomID()
	req := rpcRequest{
		Realm:     realm,
		Operation: "m3_cli",
		Arguments: rpcArgumentsM3CLI{Command: cmd},
	}

	js, _ := json.Marshal(req)

	err = ch.Publish(
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: strconv.FormatInt(int64(corrID), 10),
			ReplyTo:       q.Name,
			Body:          js,
		})
	if err != nil {
		return nil, err
	}

	for d := range msgs {
		if strconv.FormatInt(int64(corrID), 10) == d.CorrelationId {
			// res, err := strconv.Atoi(string(d.Body))
			// failOnError(err, "Failed to convert body to integer")
			if err != nil {
				log.Error(err)
			}

			log.Printf("rpc_queue result: corr_id=%d, res=%s\n", corrID, string(d.Body))

			result := &rpcResultsM3CLI{}
			err := json.Unmarshal(d.Body, result)
			if err != nil {
				return nil, fmt.Errorf("Type conversion failed")
			}
			return result, nil
		}
	}

	return nil, nil
}

// TODO: Place this method into a util package
func generateRandomID() int32 {
	rand.Seed(time.Now().UnixNano())
	return 1 + rand.Int31()
}
