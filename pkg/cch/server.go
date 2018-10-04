package cch

import (
	"encoding/json"
	"net/http"

	"github.com/golang/glog"

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

// Server handles the Control Channel WebSocket connections
type Server struct {
	db          *redis.Client
	router      *mux.Router
	sessionCtrl *sessionController
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
		glog.V(2).Info("Handle new incoming connection")

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Errorln("Failed to run websocket upgrade for incoming connection:", err)
			return
		}
		defer c.Close()

		for {
			mt, payload, err := c.ReadMessage()
			if err != nil {
				glog.Errorln("Failed to read message:", err)
				return
			}

			glog.V(2).Infof("Received message: mt=%v, payload=%s\n", mt, payload)

			var message []interface{}
			if err := json.Unmarshal(payload, &message); err != nil {
				glog.Errorln("Failed to unmarshal message:", err)
				return
			}

			messageType := message[0].(float64)
			switch int(messageType) {
			case messageTypeHello:
				{
					glog.V(2).Infoln("Processing HELLO message")

					/*if err := cc.registerSession(c, message); err != nil {
						writeAbortMessage(c, errTechnicalException, err.Error())
						return
					}*/

					if err := writeWelcomeMessage(c, 1234, welcomeMessageDetails{
						SessionTimeout: 30,
						PingInterval:   28,
						PongTimeout:    16,
						EventsTopic:    "devices::events"}); err != nil {
						glog.Errorln("Failed to write message:", err)
						return
					}
				}
				break
			case messageTypePing:
				{
					glog.V(2).Infoln("Processing PING message")

					/*if !cc.sessionForConnectionExists(c) {
						log.Println("session for client connection doesn't exists")
						writeAbortMessage(c, errInvalidSession, "session for client connection doesn't exists")
						return
					}*/

					if err := writePongMessage(c, pongMessageDetails{}); err != nil {
						glog.Errorln("Failed to write message:", err)
						return
					}
				}
				break
			}
		}
	}
}

func writeJSONArrayTextMessage(c *websocket.Conn, msg []interface{}) error {
	js, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	glog.V(2).Infof("Writing message: mt=%v, payload=%s\n", websocket.TextMessage, js)

	err = c.WriteMessage(websocket.TextMessage, js)
	if err != nil {
		return err
	}

	return nil
}

func writeAbortMessage(c *websocket.Conn, reason, details string) error {
	var msg []interface{}
	msg = append(msg, messageTypeAbort)
	msg = append(msg, reason)
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
