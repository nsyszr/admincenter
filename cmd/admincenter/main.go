package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/nsyszr/admincenter/pkg/sessionmgmt"
)

const (
	messageTypeHello   int = 1
	messageTypeWelcome int = 2
	messageTypeAbort   int = 3
	messageTypePing    int = 4
	messageTypePong    int = 5
)

const (
	errNoSuchRealm        string = "ERR_NO_SUCH_REALM"
	errProtocolViolation  string = "ERR_PROTOCOL_VIOLATION"
	errInvalidSession     string = "ERR_INVALID_SESSION"
	errUnknownException   string = "ERR_UNKNOWN_EXCEPTION"
	errTechnicalException string = "ERR_TECHNICAL_EXCEPTION"
)

type session struct {
	c              *websocket.Conn
	timeout        int
	connectedSince time.Time
}

type controlChannel struct {
	sessionsMutex sync.Mutex
	sessions      map[string]*session
}

type welcomeMessageDetails struct {
	SessionTimeout int    `json:"session_timeout"`
	PingInterval   int    `json:"ping_interval"`
	PongTimeout    int    `json:"pong_max_wait_time"`
	EventsTopic    string `json:"events_topic"`
}

type pongDetails struct {
}

type abortMessageDetails struct {
	Message string `json:"message"`
}

var addr = flag.String("addr", "192.168.209.136:9012", "http service address")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func writeJSONArrayTextMessage(c *websocket.Conn, msg []interface{}) error {
	js, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	log.Printf("writing message: mt=%v, payload=%s", websocket.TextMessage, js)
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

func (cc *controlChannel) registerSession(c *websocket.Conn, message []interface{}) error {
	log.Println("registering new session")

	if len(message) < 2 {
		return fmt.Errorf("Invalid HELLO message")
	}

	realm := message[1].(string)
	if realm == "" || !strings.Contains(realm, "@") {
		log.Println("failed to register session: invalid realm")
		return fmt.Errorf("invalid realm")
	}

	if cc.sessionExists(realm) {
		log.Println("failed to register session: client registered already")
		return fmt.Errorf("client registered already")
	}

	s := &session{
		c:              c,
		timeout:        30,
		connectedSince: time.Now(),
	}
	cc.createSession(realm, s)

	log.Printf("session for '%s' registered\n", realm)

	return nil
}

func (cc *controlChannel) sessionExists(realm string) bool {
	exists := false

	cc.sessionsMutex.Lock()
	_, exists = cc.sessions[realm]
	cc.sessionsMutex.Unlock()

	return exists
}

func (cc *controlChannel) createSession(realm string, s *session) {
	cc.sessionsMutex.Lock()
	cc.sessions[realm] = s
	cc.sessionsMutex.Unlock()
}

func (cc *controlChannel) sessionForConnectionExists(c *websocket.Conn) bool {
	exists := false

	cc.sessionsMutex.Lock()
	for _, s := range cc.sessions {
		if s.c == c {
			exists = true
			break
		}
	}
	cc.sessionsMutex.Unlock()

	return exists
}

func (cc *controlChannel) handleClientConnection(w http.ResponseWriter, r *http.Request) {
	log.Println("handle new incoming connection")

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("failed to run websocket upgrade for incoming connection: ", err)
		return
	}
	defer c.Close()

	for {
		mt, payload, err := c.ReadMessage()
		if err != nil {
			log.Println("failed to read message: ", err)
			return
		}

		log.Printf("received message: mt=%v, payload=%s", mt, payload)

		var message []interface{}
		if err := json.Unmarshal(payload, &message); err != nil {
			log.Println("failed to unmarshal message: ", err)
			return
		}

		messageType := message[0].(float64)
		switch int(messageType) {
		case messageTypeHello:
			{
				log.Println("processing HELLO message")

				if err := cc.registerSession(c, message); err != nil {
					writeAbortMessage(c, errTechnicalException, err.Error())
					return
				}

				if err := writeWelcomeMessage(c, 1234, welcomeMessageDetails{
					SessionTimeout: 30,
					PingInterval:   28,
					PongTimeout:    16,
					EventsTopic:    "devices::events"}); err != nil {
					log.Println("failed to write message: ", err)
					return
				}
			}
			break
		case messageTypePing:
			{
				log.Println("processing PING message")

				if !cc.sessionForConnectionExists(c) {
					log.Println("session for client connection doesn't exists")
					writeAbortMessage(c, errInvalidSession, "session for client connection doesn't exists")
					return
				}

				var resp []interface{}
				resp = append(resp, messageTypePong)
				resp = append(resp, pongDetails{})

				if err := writeJSONArrayTextMessage(c, resp); err != nil {
					log.Println("failed to write message: ", err)
					return
				}
			}
			break
		}
	}
}

func main() {
	fmt.Println("Starting OAM Control Channel WebSocket Server...")

	flag.Parse()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	ctrl := sessionmgmt.NewController(redisClient)
	_, err := ctrl.RegisterSession(nil, "1234@devices.iot.insys-icom.com")
	if err == sessionmgmt.ErrNoSuchRealm {
		log.Println("Could not find client!")
	}

	r := http.NewServeMux()

	cc := controlChannel{
		sessions: make(map[string]*session),
	}
	r.HandleFunc("/control", cc.handleClientConnection)

	log.Fatal(http.ListenAndServeTLS(":9443", "server.crt", "server.key", r))
}
