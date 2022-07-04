package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	rmq "github.com/adjust/rmq/v4"
	"github.com/derhnyel/whatsticker/metrics"
)

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func main() {
	errChan := make(chan error)
	connectionString := os.Getenv("WAIT_HOSTS")
	connection, err := rmq.OpenConnection("logger connection", "tcp", connectionString, 1, errChan)
	if err != nil {
		log.Print("Failed to connect to redis queue")
		return
	}

	loggingQueue, _ := connection.OpenQueue(os.Getenv("LOG_METRIC_QUEUE"))
	gauges := metrics.NewGauges()
	registry := metrics.NewRegistry()

	register := &metrics.Initialize(&registry, gauges)

	loggingQueue.StartConsuming(10, time.Second)
	loggingQueue.AddConsumer("logging-consumer", register)
	log.Printf("Starting Queue on %s", connectionString)
	listenForCtrlC()
}
