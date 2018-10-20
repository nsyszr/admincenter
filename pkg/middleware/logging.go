package middleware

import (
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
)

func WithLogging(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		lrw := newLoggingResponseWriter(w)
		wrappedHandler.ServeHTTP(lrw, req)

		statusCode := lrw.statusCode
		s := fmt.Sprintf("%s %s %d", req.Method, req.URL.Path, statusCode) // , http.StatusText(statusCode)
		if statusCode < 400 {
			log.WithFields(log.Fields{"remoteAddr": req.RemoteAddr, "hostAddr": req.Host,
				"statusCode": statusCode, "path": req.URL.Path, "method": req.Method}).
				Info(s)
		} else if statusCode < 500 {
			log.WithFields(log.Fields{"remoteAddr": req.RemoteAddr, "hostAddr": req.Host,
				"statusCode": statusCode, "path": req.URL.Path, "method": req.Method}).
				Warn(s)
		} else {
			log.WithFields(log.Fields{"remoteAddr": req.RemoteAddr, "hostAddr": req.Host,
				"statusCode": statusCode, "path": req.URL.Path, "method": req.Method}).
				Error(s)
		}
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	// WriteHeader(int) is not called if our response implicitly returns 200 OK, so
	// we default to that status code.
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
