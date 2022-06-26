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
	connectionString := os.Getenv("WAIT_HOSTS")
	connection, err := rmq.OpenConnection("worker connection", "tcp", connectionString, 1, errChan)
	if err != nil {
		log.Print("Failed to connect to redis queue")
		return
	}
	completeQueue, _ := connection.OpenQueue(os.Getenv("SEND_TO_WHATSAPP_QUEUE"))
	convertQueue, _ := connection.OpenQueue(os.Getenv("CONVERT_TO_WEBP_QUEUE"))
	convert := &convert.ConvertConsumer{
		// set to push to completeQueue when done
		PushTo: completeQueue,
	}
	convertQueue.StartConsuming(10, time.Second)
	convertQueue.AddConsumer("convert-consumer", convert)
	log.Printf("Starting Queue on %s", connectionString)
	listenForCtrlC()
}
