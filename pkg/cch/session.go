package cch

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/gorilla/websocket"
)

type session struct {
	//conn         *websocket.Conn
	conn         net.Conn
	ctrl         *sessionController
	startedAt    time.Time
	registeredCh chan bool
	registered   bool
}

func newSession(ctrl *sessionController, w http.ResponseWriter,
	r *http.Request, responseHeader http.Header) (*session, error) {
	/*var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}*/
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
			Error("Failed to upgrade HTTP websocket:", err)
		return nil, err
	}

	// TODO: Create a unique session ID, even if the session is not registered.
	//       This is interessting for Prometheus statistics! Add expires at
	//       to the session Redis key. If the session wont be registered, it
	//       disappears from the Prometheus statistics.
	// NOTES: Rethink that idea! Create seperate stats for initiated connections,
	//        for dropped onces, etc. A session should be only established
	//		  if both parties agree to each other.

	sess := &session{
		conn:         conn,
		ctrl:         ctrl,
		startedAt:    time.Now(),
		registeredCh: make(chan bool, 1),
	}

	log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
		Info("Control channel session started")

	// Start a background timer to invalidate session if not registered within
	// a specified time period.
	go func(sess *session) {
		log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
			Debug("Registration timeout routine started")
		for {
			select {
			case <-sess.registeredCh:
				log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
					Debug("Registration timeout routine stopped")
				return
			case <-time.After(60 * time.Second): // TODO: get timeout from config
				log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
					Debug("Registration timeout routine expired")
				return
			}
		}
	}(sess)

	// Start the websocket session
	go func() {
		defer conn.Close()

		var (
			r       = wsutil.NewReader(conn, ws.StateServerSide)
			w       = wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
			decoder = json.NewDecoder(r)
			//encoder = json.NewEncoder(w)
		)
		for {
			if _, err = r.NextFrame(); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to read the next frame:", err)
				return
			}

			var req []interface{}
			if err := decoder.Decode(&req); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to decode request:", err)
				return
			}

			if err = w.Flush(); err != nil {
				log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
					Error("Failed to flush response:", err)
				return
			}
		}
	}()

	return sess, nil
}

func (sess *session) close() {
	// Ensure the registration timeout routine is stopped.
	// TODO: Not the nicest way, b/c the session seems to be a valid and registered maybe we should a drop or similar channel!
	sess.registeredCh <- true

	if sess.conn != nil {
		sess.conn.Close()
	}

	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Info("Control channel session closed")
}

func (sess *session) listenAndServe() {
	/*
		for {
			mt, payload, err := sess.conn.ReadMessage()
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
					if len(message) < 2 || message[1].(string) == "" {
						writeAbortMessage(sess.conn, errInvalidRealm, "No or invalid realm given")
						return
					}

					sessionID, err := sess.ctrl.registerSession(sess.conn, message[1].(string))
					if err != nil {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(sess.conn, err, "Add a good error message...")
						return
					}

					// Add websocket connection to map with associated session ID
					sess.ctrl.sessions[sess.conn] = sessionID

					if err := writeWelcomeMessage(sess.conn, 1234, welcomeMessageDetails{
						SessionTimeout: 30,
						PingInterval:   28,
						PongTimeout:    16,
						EventsTopic:    "devices::events"}); err != nil {
						log.Error("Failed to write message:", err)
						return
					}

					// Quit the registration timeout routine
					sess.register()
				}
				break
			case messageTypePing, messageTypePublish:
				{
					exists, err := sess.ctrl.existsSession(sess.ctrl.sessions[sess.conn])
					if err != nil {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(sess.conn, err, "Add a good error message...")
						return
					}

					if !exists {
						// TODO: Ensure that we get only errors with valid reason! Eg. tech. exception, etc.
						writeAbortMessage(sess.conn, errInvalidSession, "Add a good error message...")
						return
					}

					if err := sess.handleIncomingMessage(sess.conn, message); err != nil {
						log.Error("Failed to handle incoming message:", err)
						return
					}

					// Update the session, otherwise it expires
					sess.ctrl.updateSession(sess.ctrl.sessions[sess.conn], 1, 1)
				}
				break
			default:
				{

				}
				break
			}
		}*/
}

func (sess *session) handleIncomingMessage(c *websocket.Conn, message []interface{}) error {
	/*messageType := int(message[0].(float64))
	switch messageType {
	case messageTypePing:
		{
			if err := writePongMessage(c, pongMessageDetails{}); err != nil {
				return err
			}
		}
		break
		case messageTypePublish:
		{
			requestID := int(message[1].(float64))
			topic := message[2].(string)
			//args := message[3].(publishMessageDetails)
			//log.Debug("args=", args)

			body, err := json.Marshal(message[3])
			if err != nil {
				log.Error("Failed to marshal body:", err)
				return err
			}
			log.Debug("body=", string(body))

			publicationID, err := sess.ctrl.publishMessageToAMQP(topic, string(body))
			if err != nil {
				writeErrorMessage(c, messageType, requestID,
					"ERR_UNKNOWN_EXCEPTION", errorMessageDetails{Error: err.Error()})

				// TODO: check if this okay? How to handle such errors in future?
				return nil
			}

			if err := writePublishedMessage(c, requestID, publicationID); err != nil {
				return err
			}
		}
	}*/

	return nil
}

func (sess *session) register() {
	sess.registered = true
	sess.registeredCh <- true
}
