package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/deven96/whatsticker/convert"

	amqp "github.com/rabbitmq/amqp091-go"
)

type amqpConfig struct {
	uri          string // AMQP URI
	exchange     string // Durable, non-auto-deleted AMQP exchange name
	exchangeType string // Exchange type - direct|fanout|topic|x-custom
	queue        string // Ephemeral AMQP queue name
	bindingKey   string // AMQP binding key
	consumerTag  string // AMQP consumer tag (should not be blank)
	lifetime     uint   // 5*time.Second, lifetime of process before shutdown (0s=infinite)"
	verbose      bool   // enable verbose output of message data
	autoAck      bool   // enable message auto-ack
}

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func getAMQPConfig(uri string) *amqpConfig {
	return &amqpConfig{
		uri:      uri,
		exchange: "",
		verbose:  false,
		autoAck:  true,
	}
}

func main() {
	amqpConfig := getAMQPConfig(os.Getenv("AMQP_URI"))
	conn, err := amqp.Dial(amqpConfig.uri)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// RabbitMQ not to give more than one message to a worker at a time
	// don't dispatch a new message to a worker until it has processed and acknowledged the previous one.
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	convertQueue, err := ch.QueueDeclare(
		os.Getenv("CONVERT_TO_WEBP_QUEUE"),
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to connect to RabbitMQ")

	completeQueue, err := ch.QueueDeclare(
		os.Getenv("SEND_WEBP_TO_WHATSAPP_QUEUE"),
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to connect to RabbitMQ")

	convertQueueMsgs, err := ch.Consume(
		convertQueue.Name, // queue
		"",                // consumer
		false,             // auto-ack, so we can ack it ourself after processing
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	failOnError(err, "Failed to register a consumer")

	convert := &convert.ConvertConsumer{
		// set to push to completeQueue when done
		//set to push metrics to loggingQueue when done
		PushTo: completeQueue,
	}

	go func() {
		for d := range convertQueueMsgs {
			convert.Consume(ch, &d)
		}
	}()

	listenForCtrlC()
}
