package utils

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// PublishBytesToQueue : Send bytes to a queue on a channel
func PublishBytesToQueue(ch *amqp.Channel, q *amqp.Queue, bytes []byte) {
	err := ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/json",
			Body:         bytes,
		})
	if err != nil {
		log.Errorf("Failed to publish to queue %s", q.Name)
	}
}

// ListenForCtrlC
// quit if Ctrl+C is pressed
func ListenForCtrlC(service string) {
	log.Infof("Started %s. Waiting for Ctrl+C", service)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

// GetQueue : Returns an AMQP Queue
func GetQueue(ch *amqp.Channel, queueName string, durable bool) *amqp.Queue {
	q, err := ch.QueueDeclare(
		queueName,
		durable, // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	FailOnError(err, "Failed to connect to RabbitMQ")
	return &q
}

func GetLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "trace":
		return log.TraceLevel
	case "debug":
		return log.DebugLevel
	case "info":
		return log.InfoLevel
	case "error":
		return log.ErrorLevel
	case "warn":
		return log.WarnLevel
	case "fatal":
		return log.FatalLevel
	case "panic":
		return log.PanicLevel
	default:
		return log.InfoLevel
	}
}

// Returns a LogLevel type from Environment variable
func GetLogLevelFromEnv() log.Level {
	return GetLogLevel(os.Getenv("LOG_LEVEL"))
}
