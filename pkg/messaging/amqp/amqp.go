package amqp

import (
	"fmt"
	"time"

	"github.com/nsyszr/admincenter/pkg/util/rand"
	"github.com/streadway/amqp"
)

type Broker struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewBroker(conn *amqp.Connection) (*Broker, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	return &Broker{
		Conn:    conn,
		Channel: ch,
	}, nil
}

func (broker *Broker) Close() {
	if broker.Channel != nil {
		broker.Channel.Close()
	}
}

// Request sends a message to broker according to the request-reply pattern
func (broker *Broker) Request(route string, body []byte, timeout int) ([]byte, error) {
	q, err := broker.Channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	msgs, err := broker.Channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	corrID := rand.GenerateRandomString(32)
	err = broker.Channel.Publish(
		"",    // exchange
		route, // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrID,
			ReplyTo:       q.Name,
			Body:          body,
		})
	if err != nil {
		return nil, err
	}

	/*for msg := range msgs {
		if corrID == msg.CorrelationId {
			return msg.Body, nil
		}
	}*/

	// Loop for message with corresponding ID until timeout reached
	for {
		select {
		case msg := <-msgs:
			if corrID == msg.CorrelationId {
				return msg.Body, nil
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, fmt.Errorf("No reply within timeout")
		}
	}
}

// ReplyTo replies a message to broker according to the request-reply pattern
func (broker *Broker) ReplyTo(route, correlationID string, body []byte) error {
	if err := broker.Channel.Publish(
		"",    // exchange
		route, // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: correlationID,
			Body:          body,
		}); err != nil {
		return err
	}

	return nil
}
