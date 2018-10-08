package cch

import (
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

type session struct {
	conn       *websocket.Conn
	ctrl       *sessionController
	startedAt  time.Time
	registered bool
}

func newSession(ctrl *sessionController, w http.ResponseWriter,
	r *http.Request, responseHeader http.Header) (*session, error) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}

	// TODO: Create a unique session ID, even if the session is not registered.
	//       This is interessting for Prometheus statistics! Add expires at
	//       to the sessions Redis key. If the session wont be registered, it
	//       disappears from the Prometheus statistics.

	sess := &session{
		conn:      conn,
		ctrl:      ctrl,
		startedAt: time.Now(),
	}

	log.WithFields(log.Fields{"remoteAddr": conn.RemoteAddr()}).
		Info("Control channel session started")

	return sess, nil
}

func (sess *session) close() {
	if sess.conn != nil {
		sess.conn.Close()
	}

	log.WithFields(log.Fields{"remoteAddr": sess.conn.RemoteAddr()}).
		Info("Control channel session closed")
}
