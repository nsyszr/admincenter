package controlchannel

import (
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gobwas/ws"
	"github.com/streadway/amqp"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
)

// Server handles the Control Channel WebSocket connections
type Server struct {
	db           *redis.Client
	ch           *amqp.Channel
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
	// Setup a AMQP channel
	ch, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:     db,
		ch:     ch,
		router: router,
		ctrl:   NewController(db),
	}

	s.configureRoutes()

	return s, nil
}

func (s *Server) configureRoutes() {
	s.router.HandleFunc("/cch", s.handleControlChannel())
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
