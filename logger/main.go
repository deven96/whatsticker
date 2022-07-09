package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/deven96/whatsticker/logger/metrics"
	"github.com/deven96/whatsticker/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

func main() {
	var (
		port = flag.String("listen-port", ":9091", "The address to listen on for HTTP requests.")
	)
	counters := metrics.NewCounters()
	registry := metrics.NewRegistry()
	metric := metrics.Initialize(registry, counters)

	log.SetLevel(utils.GetLogLevelFromEnv())

	log.Infof("Initialized Metrics SideCar %#v", metric)

	amqpConfig := utils.GetAMQPConfig()
	conn, err := amqp.Dial(amqpConfig.Uri)
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	loggingQueue := utils.GetQueue(ch, os.Getenv("LOG_METRIC_QUEUE"), false)

	loggingQueueMsgs, err := ch.Consume(
		loggingQueue.Name, // queue
		"",                // consumer
		true,              // auto-ack true so that we don't resend/consume queue message
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	utils.FailOnError(err, "Failed to register a consumer")

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
