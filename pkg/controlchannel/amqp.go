package controlchannel

import (
	"fmt"
	"log"
	"strconv"

	"github.com/streadway/amqp"
)

func (s *Server) publishMessageToAMQP(topic, body string) (int, error) {
	if err := s.ch.ExchangeDeclare(
		topic,    // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	); err != nil {
		return 0, err
	}

	if err := s.ch.Publish(
		topic, // exchange
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		}); err != nil {
		return 0, err
	}

	// TODO: get a message ID during publish and return it
	return 1234, nil
}

func (s *Server) configureRPCQueue(rpcQueueName string) error {
	_, err := s.ch.QueueDeclare(
		rpcQueueName, // name
		false,        // durable
		false,        // delete when usused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	err = s.ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) listenToRPCQueue(rpcQueueName string) error {
	if rpcQueueName == "" {
		// TODO: Better error handling
		return fmt.Errorf("No rpc queue name set")
	}

	msgs, err := s.ch.Consume(
		rpcQueueName, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	for msg := range msgs {
		n, _ := strconv.Atoi(string(msg.Body))
		// failOnError(err, "Failed to convert body to integer")

		log.Printf(" [.] fib(%d)", n)
		response := 1 // fib(n)

		err = s.ch.Publish(
			"",          // exchange
			msg.ReplyTo, // routing key
			false,       // mandatory
			false,       // immediate
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: msg.CorrelationId,
				Body:          []byte(strconv.Itoa(response)),
			})
		// failOnError(err, "Failed to publish a message")

		msg.Ack(false)
	}

	return nil
}
