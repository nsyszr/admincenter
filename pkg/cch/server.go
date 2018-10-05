package cch

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/Sirupsen/logrus"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	messageTypeHello   int = 1
	messageTypeWelcome int = 2
	messageTypeAbort   int = 3
	messageTypePing    int = 4
	messageTypePong    int = 5
)

var errInvalidSession = errors.New("ERR_INVALID_SESSION")

// Server handles the Control Channel WebSocket connections
type Server struct {
	db          *redis.Client
	router      *mux.Router
	sessionCtrl *sessionController
	sessions    map[*websocket.Conn]int32
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

// NewServer creates a new Control Channel WebSocket server handler
func NewServer(db *redis.Client, router *mux.Router) *Server {
	s := &Server{
		db:          db,
		router:      router,
		sessionCtrl: newSessionController(db),
		sessions:    make(map[*websocket.Conn]int32),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.HandleFunc("/cch", s.handleControlChannel())
}

func (s *Server) handleControlChannel() http.HandlerFunc {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Handle new incoming connection")

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
					log.Debug("Processing HELLO message")

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
			case messageTypePing:
				{
					log.Debug("Processing PING message")

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

					if err := writePongMessage(c, pongMessageDetails{}); err != nil {
						log.Error("Failed to write message:", err)
						return
					}

					// Update the session, otherwise it expires
					s.sessionCtrl.updateSession(s.sessions[c], 1, 1)
				}
				break
			default:
				// TODO: risk!
				s.sessionCtrl.updateSession(s.sessions[c], 1, 1)
			}
		}
	}
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

func writeWelcomeMessage(c *websocket.Conn, sessionID int, details welcomeMessageDetails) error {
	var msg []interface{}
	msg = append(msg, messageTypeWelcome)
	msg = append(msg, sessionID)
	msg = append(msg, details)

	return writeJSONArrayTextMessage(c, msg)
}

func writePongMessage(c *websocket.Conn, details pongMessageDetails) error {
	var msg []interface{}
	msg = append(msg, messageTypePong)
	msg = append(msg, details)

	return writeJSONArrayTextMessage(c, msg)
}
