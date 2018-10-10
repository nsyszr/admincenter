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
	sessCtrl     *sessionController
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
		db:       db,
		ch:       ch,
		router:   router,
		sessCtrl: newSessionController(db),
	}

	s.configureRoutes()

	return s, nil
}

func (s *Server) configureRoutes() {
	s.router.HandleFunc("/cch", s.handleControlChannel())
}

func (s *Server) handleControlChannel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Enter handleControlChannel()")

		/*sess, err := newSession(s.sessCtrl, w, r, nil)
		if err != nil {
			log.Error("Failed to start control channel session:", err)
			return
		}
		defer sess.close()

		sess.listenAndServe()*/
		s.sessCtrl.handleSession(w, r)
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
