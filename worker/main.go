package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deven96/whatsticker/convert"

	rmq "github.com/adjust/rmq/v4"
)

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func main() {
	errChan := make(chan error)
	connectionString := "redis:6379"
	connection, err := rmq.OpenConnection("worker connection", "tcp", connectionString, 1, errChan)
	if err != nil {
		log.Print("Failed to connect to redis queue")
		return
	}
	completeQueue, _ := connection.OpenQueue("complete")
	convertQueue, _ := connection.OpenQueue("convert")
	convert := &convert.ConvertConsumer{
		PushTo: completeQueue,
	}
	convertQueue.StartConsuming(10, time.Second)
	convertQueue.AddConsumer("convert-consumer", convert)
	log.Printf("Starting Queue on %s", connectionString)
	listenForCtrlC()
}
