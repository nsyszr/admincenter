package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/nsyszr/admincenter/pkg/api"
	"github.com/nsyszr/admincenter/pkg/middleware"
	"github.com/streadway/amqp"
)

func main() {
	port := 8080

	// log.SetFormatter(&logmatic.JSONFormatter{})
	log.SetLevel(log.DebugLevel)

	log.Info("Starting OAM API-Server")

	// Setup connection to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer redisClient.Close()

	// Setup connection to AMQP
	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Error("Failed to connect to AMQP:", err)
		return
	}
	defer amqpConn.Close()

	// Create a new HTTP router
	r := mux.NewRouter()

	// Handle API
	apiServer := api.NewServer(redisClient, amqpConn)
	apiServer.Serve(r.PathPrefix("/api/v1").Subrouter())

	// Catch SIGINT
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Create the http server and start it in the background
	h := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: middleware.WithLogging(r)}

	go func() {
		log.Infof("Listening on http://0.0.0.0%s\n", fmt.Sprintf(":%d", port))

		if err := h.ListenAndServe(); err != nil {
			log.Error(err)
			os.Exit(1)
		}
	}()

	<-stop

	// App received SIGINT signal. Shutdown now!
	log.Info("Shutting down the server...")
	h.Shutdown(context.Background())
	log.Info("Server gracefully stopped")
}
