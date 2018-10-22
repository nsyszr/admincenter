package messaging

type Broker interface {
	Request(route string, body []byte, timeout int) ([]byte, error)
	ReplyTo(route, correlationID string, body []byte) errors
}
