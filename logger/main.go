package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/deven96/whatsticker/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func getAMQPConfig(uri string) *amqpConfig {
	return &amqpConfig{
		uri:      uri,
		exchange: "",
		verbose:  false,
		autoAck:  true,
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	var (
		port = flag.String("listen-port", ":9091", "The address to listen on for HTTP requests.")
	)
	gauges := metrics.NewGauges()
	registry := metrics.NewRegistry()
	metric := metrics.Initialize(registry, gauges)

	log.Printf("Initialized Metrics SideCar %#v", metric)

	amqpConfig := getAMQPConfig(os.Getenv("AMQP_URI"))
	conn, err := amqp.Dial(amqpConfig.uri)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	loggingQueue, err := ch.QueueDeclare(
		os.Getenv("LOG_METRIC_QUEUE"),
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to connect to RabbitMQ")

	loggingQueueMsgs, err := ch.Consume(
		loggingQueue.Name, // queue
		"",                // consumer
		true,              // auto-ack true so that we don't resend/consume queue message
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for d := range loggingQueueMsgs {
			metric.Consume(ch, &d)
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))
	log.Fatal(http.ListenAndServe(*port, nil))
}
