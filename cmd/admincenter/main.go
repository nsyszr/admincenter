package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/go-redis/redis"
	"github.com/nsyszr/admincenter/pkg/cch"
)

/*const (
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
	sessionCtrl   *sessionmgmt.Controller
}

var addr = flag.String("addr", "192.168.209.136:9012", "http service address")


func (cc *controlChannel) registerSession(c *websocket.Conn, message []interface{}) error {
	log.Println("registering new session")

	if len(message) < 2 {
		return fmt.Errorf("control channel: invalid HELLO message")
	}

	realm := message[1].(string)
	if realm == "" || !strings.Contains(realm, "@") {
		log.Println("control channel: invalid realm")
		return fmt.Errorf("control channel: invalid realm")
	}

	if cc.sessionExists(realm) {
		log.Println("control channel: failed to register session, client registered already")
		return fmt.Errorf("control channel: client registered already")
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
}*/

func init() {
	// flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
	flag.Parse()
}

func main() {
	fmt.Println("Starting OAM Control Channel WebSocket Server...")

	db := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer db.Close()

	/*ctrl := sessionmgmt.NewController(redisClient)

	r := http.NewServeMux()

	cc := controlChannel{
		sessions:    make(map[string]*session),
		sessionCtrl: ctrl,
	}
	r.HandleFunc("/control", cc.handleClientConnection)*/

	r := mux.NewRouter()
	cch.NewServer(db, r)

	log.Fatal(http.ListenAndServeTLS(":9443", "server.crt", "server.key", r))
}
