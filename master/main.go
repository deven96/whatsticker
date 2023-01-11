package main

import (
	"flag"
	"fmt"
	"net/http"

	"os"
	"path/filepath"
	"strings"

	"github.com/deven96/whatsticker/master/handler"
	"github.com/deven96/whatsticker/master/task"
	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	_ "github.com/mattn/go-sqlite3"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

var ch *amqp.Channel
var convertQueue *amqp.Queue
var loggingQueue *amqp.Queue

type incomingMessageHandler struct {
}

func (i *incomingMessageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		fmt.Println(r.URL.Query())
		verifyToken := r.URL.Query().Get("hub.verify_token")
		mode := r.URL.Query().Get("hub.mode")
		challenge := r.URL.Query().Get("hub.challenge")
		if verifyToken == os.Getenv("VERIFY_TOKEN") && mode == "subscribe" {
			w.Write([]byte(challenge))
		} else {
			http.Error(w, "Could not verify challenge", http.StatusBadRequest)
		}
		return
	}
	parsed, err := whatsapp.UnmarshalIncomingMessage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	go handler.Run(parsed, ch, convertQueue, loggingQueue)
	fmt.Printf("%#v", parsed.Entry)
}

func main() {
	masterDir, _ := filepath.Abs("./master")
	logLevel := flag.String("log-level", "INFO", "Set log level to one of (INFO/DEBUG)")
	port := flag.String("port", "9000", "Set port to start incoming streaming server")
	flag.Parse()

	if ll := os.Getenv("LOG_LEVEL"); ll != "" {
		llg := strings.ToUpper(ll)
		logLevel = &llg
	}

	log.SetLevel(utils.GetLogLevel(*logLevel))
	fmt.Println(masterDir)

	amqpConfig := utils.GetAMQPConfig()
	conn, err := amqp.Dial(amqpConfig.Uri)
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err = conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	// RabbitMQ not to give more than one message to a worker at a time
	// don't dispatch a new message to a worker until it has processed and acknowledged the previous one.
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)

	convertQueue = utils.GetQueue(ch, os.Getenv("CONVERT_TO_WEBP_QUEUE"), true)
	completeQueue := utils.GetQueue(ch, os.Getenv("SEND_WEBP_TO_WHATSAPP_QUEUE"), true)
	loggingQueue = utils.GetQueue(ch, os.Getenv("LOG_METRIC_QUEUE"), false)
	complete := &task.StickerConsumer{
		PushMetricsTo: loggingQueue,
	}

	completeQueueMsgs, err := ch.Consume(
		completeQueue.Name, // queue
		"",                 // consumer
		false,              // auto-ack, so we can ack it ourself after processing
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	utils.FailOnError(err, "Failed to register a consumer")
	go func() {
		for d := range completeQueueMsgs {
			complete.Execute(ch, &d)
		}
	}()
	http.Handle("/incoming", &incomingMessageHandler{})
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Errorf("Could not start server on %s", *port)
	} else {
		log.Infof("Started Server on %s", *port)
	}
}
