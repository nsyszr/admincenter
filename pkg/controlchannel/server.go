package controlchannel

import (
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobwas/ws"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

// Server handles the Control Channel WebSocket connections
type Server struct {
	db           *redis.Client
	router       *mux.Router
	ctrl         *Controller
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
	ctrl, err := NewController(db, amqpConn)
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:     db,
		router: router,
		ctrl:   ctrl,
	}

	s.configureRoutes()

	return s, nil
}

func (s *Server) configureRoutes() {
	s.router.HandleFunc("/cch", s.handleControlChannel())
	s.router.HandleFunc("/health", s.handleHealth())
}

func (s *Server) handleControlChannel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{"remoteAddr": r.RemoteAddr}).
			Debug("New Control Channel request")

		// Upgrade to a WebSocket connection
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
				Error("Failed to upgrade HTTP websocket:", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sess := NewSession(conn, s.ctrl)
		go sess.ensureWelcomeMessage()
		go sess.listen()
	}
}

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{"remoteAddr": r.RemoteAddr}).
			Debug("New health request")
		w.WriteHeader(200)
		w.Write([]byte("OK\n"))
	}
}
