package cch

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/streadway/amqp"

	log "github.com/Sirupsen/logrus"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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

var errInvalidSession = errors.New("ERR_INVALID_SESSION")

// Server handles the Control Channel WebSocket connections
type Server struct {
	db           *redis.Client
	ch           *amqp.Channel
	router       *mux.Router
	sessionCtrl  *sessionController
	sessions     map[*websocket.Conn]int32
	rpcQueueName string
}

type abortMessageDetails struct {
	Message string `json:"message"`
}

type welcomeMessageDetails struct {
	SessionTimeout int    `json:"session_timeout"`
	PingInterval   int    `json:"ping_interval"`
	PongTimeout    int    `json:"pong_max_wait_time"`
	EventsTopic    string `json:"events_topic"`
}

type pongMessageDetails struct {
}

type errorMessageDetails struct {
	Error string `json:"error"`
}

type publishMessageDetails struct {
	DeviceID  string    `json:"device_id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// NewServer creates a new Control Channel WebSocket server handler
func NewServer(db *redis.Client, amqpConn *amqp.Connection, router *mux.Router) (*Server, error) {
	// Setup a AMQP channel
	ch, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:          db,
		ch:          ch,
		router:      router,
		sessionCtrl: newSessionController(db),
		sessions:    make(map[*websocket.Conn]int32),
	}

	s.configureRoutes()

	return s, nil
}

func (s *Server) configureRoutes() {
	s.router.HandleFunc("/cch", s.handleControlChannel())
}

func (s *Server) handleControlChannel() http.HandlerFunc {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("New control channel connection opened")

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("Failed to run websocket upgrade for incoming connection:", err)
			return
		}
		defer c.Close()

		for {
			mt, payload, err := c.ReadMessage()
			if err != nil {
				log.Error("Failed to read message:", err)
				return
			}

			log.WithFields(log.Fields{"mt": mt, "payload": string(payload)}).Debug("Received message")

			var message []interface{}
			if err := json.Unmarshal(payload, &message); err != nil {
				log.Error("Failed to unmarshal message:", err)
				return
			}

			messageType := message[0].(float64)
			switch int(messageType) {
			case messageTypeHello:
				{
					if len(message) < 2 || message[1].(string) == "" {
						writeAbortMessage(c, errInvalidRealm, "No or invalid realm given")
						return
					}

					sessionID, err := s.sessionCtrl.registerSession(c, message[1].(string))
					if err != nil {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(c, err, "Add a good error message...")
						return
					}

					// Add websocket connection to map with associated session ID
					s.sessions[c] = sessionID

					if err := writeWelcomeMessage(c, 1234, welcomeMessageDetails{
						SessionTimeout: 30,
						PingInterval:   28,
						PongTimeout:    16,
						EventsTopic:    "devices::events"}); err != nil {
						log.Error("Failed to write message:", err)
						return
					}
				}
				break
			case messageTypePing, messageTypePublish:
				{
					exists, err := s.sessionCtrl.existsSession(s.sessions[c])
					if err != nil {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(c, err, "Add a good error message...")
						return
					}

					if !exists {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(c, errInvalidSession, "Add a good error message...")
						return
					}

					if err := s.handleIncomingMessage(c, message); err != nil {
						log.Error("Failed to handle incoming message:", err)
						return
					}

					// Update the session, otherwise it expires
					s.sessionCtrl.updateSession(s.sessions[c], 1, 1)
				}
				break
			default:
				{

				}
				break
			}
		}
	}
}

func (s *Server) handleIncomingMessage(c *websocket.Conn, message []interface{}) error {
	messageType := int(message[0].(float64))
	switch messageType {
	case messageTypePing:
		{
			if err := writePongMessage(c, pongMessageDetails{}); err != nil {
				return err
			}
		}
		break
	case messageTypePublish:
		{
			requestID := int(message[1].(float64))
			topic := message[2].(string)
			//args := message[3].(publishMessageDetails)
			//log.Debug("args=", args)

			body, err := json.Marshal(message[3])
			if err != nil {
				log.Error("Failed to marshal body:", err)
				return err
			}
			log.Debug("body=", string(body))

			publicationID, err := s.publishMessageToAMQP(topic, string(body))
			if err != nil {
				writeErrorMessage(c, messageType, requestID,
					"ERR_UNKNOWN_EXCEPTION", errorMessageDetails{Error: err.Error()})

				// TODO: check if this okay? How to handle such errors in future?
				return nil
			}

			if err := writePublishedMessage(c, requestID, publicationID); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeJSONArrayTextMessage(c *websocket.Conn, msg []interface{}) error {
	js, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{"mt": websocket.TextMessage, "payload": string(js)}).Debug("Sending message")

	err = c.WriteMessage(websocket.TextMessage, js)
	if err != nil {
		return err
	}

	return nil
}

func writeAbortMessage(c *websocket.Conn, reason error, details string) error {
	var msg []interface{}
	msg = append(msg, messageTypeAbort)
	msg = append(msg, reason.Error())
	msg = append(msg, abortMessageDetails{Message: details})

	return writeJSONArrayTextMessage(c, msg)
}

func writeWelcomeMessage(c *websocket.Conn, sessionID int, details interface{}) error {
	var msg []interface{}
	msg = append(msg, messageTypeWelcome)
	msg = append(msg, sessionID)
	msg = append(msg, details)

	return writeJSONArrayTextMessage(c, msg)
}

func writePongMessage(c *websocket.Conn, details interface{}) error {
	var msg []interface{}
	msg = append(msg, messageTypePong)
	msg = append(msg, details)

	return writeJSONArrayTextMessage(c, msg)
}

func writeErrorMessage(c *websocket.Conn, messageType, requestID int, err string, details interface{}) error {
	var msg []interface{}
	msg = append(msg, messageTypeError)
	msg = append(msg, messageType)
	msg = append(msg, requestID)
	msg = append(msg, err)
	msg = append(msg, details)

	return writeJSONArrayTextMessage(c, msg)
}

func writePublishedMessage(c *websocket.Conn, requestID, publicationID int) error {
	var msg []interface{}
	msg = append(msg, messageTypePublished)
	msg = append(msg, requestID)
	msg = append(msg, publicationID)

	return writeJSONArrayTextMessage(c, msg)
}
