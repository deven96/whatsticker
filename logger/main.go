package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	rmq "github.com/adjust/rmq/v4"
	"github.com/deven96/whatsticker/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var (
		port = flag.String("listen-port", ":9091", "The address to listen on for HTTP requests.")
	)
	gauges := metrics.NewGauges()
	registry := metrics.NewRegistry()
	metric := metrics.Initialize(registry, gauges)

	log.Printf("Initialized Metrics SideCar %#v", metric)

	errChan := make(chan error)
	connectionString := os.Getenv("WAIT_HOSTS")
	connection, err := rmq.OpenConnection("logger connection", "tcp", connectionString, 1, errChan)
	if err != nil {
		log.Print("Failed to connect to redis queue")
		return
	}
	loggingQueue, _ := connection.OpenQueue(os.Getenv("LOG_METRIC_QUEUE"))
	loggingQueue.StartConsuming(10, time.Second)
	loggingQueue.AddConsumer("logging-consumer", &metric)
	log.Printf("Starting Queue on %s", connectionString)

	http.Handle("/metrics", promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))
	log.Fatal(http.ListenAndServe(*port, nil))
}
