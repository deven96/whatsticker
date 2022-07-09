package main

import (
	"os"

	"github.com/deven96/whatsticker/utils"
	"github.com/deven96/whatsticker/worker/convert"
	log "github.com/sirupsen/logrus"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	log.SetLevel(utils.GetLogLevelFromEnv())
	amqpConfig := utils.GetAMQPConfig()
	conn, err := amqp.Dial(amqpConfig.Uri)
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	// RabbitMQ not to give more than one message to a worker at a time
	// don't dispatch a new message to a worker until it has processed and acknowledged the previous one.
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	utils.FailOnError(err, "Failed to set QoS")
	convertQueue := utils.GetQueue(ch, os.Getenv("CONVERT_TO_WEBP_QUEUE"), true)
	completeQueue := utils.GetQueue(ch, os.Getenv("SEND_WEBP_TO_WHATSAPP_QUEUE"), true)

	convertQueueMsgs, err := ch.Consume(
		convertQueue.Name, // queue
		"",                // consumer
		false,             // auto-ack, so we can ack it ourself after processing
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	utils.FailOnError(err, "Failed to register a consumer")

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

	utils.ListenForCtrlC("worker")
}
