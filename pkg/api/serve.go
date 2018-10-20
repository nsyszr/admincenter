package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/nsyszr/admincenter/pkg/middleware"
	"github.com/streadway/amqp"
)

// Server runs the RESTful API
type Server struct {
	RedisClient *redis.Client
	AMQPConn    *amqp.Connection
}

// NewServer creates a new RESTful API server
func NewServer(redisClient *redis.Client, amqpConn *amqp.Connection) *Server {
	return &Server{
		RedisClient: redisClient,
		AMQPConn:    amqpConn,
	}
}

// Serve the HTTP endpoints
func (s *Server) Serve(r *mux.Router) {
	withCORS := middleware.CORSHandler()

	r.Handle("/sessions/active",
		withCORS(s.handleGetSessionsActive())).Methods("GET", "OPTIONS")
	r.Handle("/sessions/run",
		withCORS(s.handlePostSessionsRun())).Methods("POST", "OPTIONS")
}

func respondWithJSON(w http.ResponseWriter, resp interface{}, statusCode int) {
	js, err := json.Marshal(resp)
	if err != nil {
		respondWithError(w, 1000, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(js)
}

func respondWithStatusCode(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
}

func respondWithError(w http.ResponseWriter, errorCode int, errorMessage string, statusCode int) {
	type errorResponse struct {
		ErrorCode    int    `json:"code"`
		ErrorMessage string `json:"message"`
		StatusCode   int    `json:"statusCode"`
	}

	resp := errorResponse{
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		StatusCode:   statusCode,
	}

	js, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(js)
}
