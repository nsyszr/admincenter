package main

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"

	log "github.com/Sirupsen/logrus"
	"github.com/go-redis/redis"
	"github.com/nsyszr/admincenter/pkg/controlchannel"
)

func init() {
	// flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
	flag.Parse()
}

func main() {
	// log.SetFormatter(&logmatic.JSONFormatter{})
	log.SetLevel(log.DebugLevel)

	log.Info("Starting OAM Control Channel WebSocket Server")

	// Setup connection to Redis
	db := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer db.Close()

	// Setup connection to AMQP
	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Error("Failed to connect to AMQP:", err)
		return
	}
	defer amqpConn.Close()

	// Create new control channel server
	r := mux.NewRouter()
	_, err = controlchannel.NewServer(db, amqpConn, r)
	if err != nil {
		log.Error("Failed to create new control channel server:", err)
		return
	}

	// Start server
	if err := http.ListenAndServeTLS(":9443", "server.crt", "server.key", r); err != nil {
		log.Error("Failed to start HTTP server:", err)
	}
}
