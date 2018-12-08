package main

import (
	"encoding/json"
	"flag"
	"math/rand"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type rpcRequest struct {
	Realm     string      `json:"realm"`
	Operation string      `json:"operation"`
	Arguments interface{} `json:"args"`
}

type rpcArgumentsM3CLI struct {
	Command string `json:"command"`
}

func init() {
	// flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
	flag.Parse()
}

func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func main() {
	// log.SetFormatter(&logmatic.JSONFormatter{})
	log.SetLevel(log.DebugLevel)

	log.Info("Starting OAM RPC-Client")

	// Setup connection to AMQP
	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Error("Failed to connect to AMQP:", err)
		return
	}
	defer amqpConn.Close()

	ch, err := amqpConn.Channel()
	if err != nil {
		log.Error(err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		log.Error(err)
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	corrID := randomString(32)
	//cmd := `-----LUA-----\nres=cli("administration.hostnames.hostname")\nprint(res)\n-----LUA-----\n`
	/*cmd := `-----LUA-----
	print("Hello from LUA")
	-----LUA-----
	`*/
	cmd := "status.sysdetail.system.mac"
	req := rpcRequest{
		Realm:     "1234@devices.iot.insys-icom.com",
		Operation: "m3_cli",
		Arguments: rpcArgumentsM3CLI{Command: cmd}, // status.device_info
	}

	js, _ := json.Marshal(req)

	err = ch.Publish(
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrID,
			ReplyTo:       q.Name,
			Body:          js, // []byte(strconv.Itoa(123)),
		})
	if err != nil {
		log.Error(err)
		return
	}

	for d := range msgs {
		if corrID == d.CorrelationId {
			// res, err := strconv.Atoi(string(d.Body))
			// failOnError(err, "Failed to convert body to integer")
			if err != nil {
				log.Error(err)
			}
			log.Printf("rpc_queue result: corr_id=%s, res=%s\n", corrID, string(d.Body))
			break
		}
	}
}
