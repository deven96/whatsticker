package main

import (
	"context"
	"flag"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"github.com/deven96/whatsticker/master/handler"
	"github.com/deven96/whatsticker/master/task"
	"github.com/deven96/whatsticker/utils"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var client *whatsmeow.Client
var ch *amqp.Channel
var replyTo *bool
var sender *string
var convertQueue *amqp.Queue
var loggingQueue *amqp.Queue

var commands = map[string]struct{}{
	"stickerize deven96": {},
	"stickerize":         {},
}

type CompletedTask struct {
	MediaPath     string
	ConvertedPath string
	DataLen       int
	MediaType     string
	Chat          string
	IsGroup       bool
	MessageSender string
	TimeOfRequest string
}

func loginNewClient() {
	// No ID stored, new login
	qrChan, _ := client.GetQRChannel(context.Background())
	err := client.Connect()
	if err != nil {
		panic(err)
	}
	for evt := range qrChan {
		if evt.Event == "code" {
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			log.Info("QR code: ", evt.Code)
		} else {
			log.Info("Login event: ", evt.Event)
		}
	}
}

func captionIsCommand(caption string) bool {
	_, ok := commands[strings.TrimSpace(strings.ToLower(caption))]
	return ok
}

func eventHandler(evt interface{}) {
	switch eventInfo := evt.(type) {
	case *events.ConnectFailure, *events.ClientOutdated:
		log.Error("Killing due to client related issues")
		os.Exit(1)
	case *events.StreamReplaced:
		log.Info("Started another stream with the same device session")
		//    os.Exit(1)
	case *events.OfflineSyncCompleted:
		log.Info("Offline sync completed")
	case *events.Message:
		extended := eventInfo.Message.GetExtendedTextMessage()
		quotedMsg := extended.GetContextInfo().GetQuotedMessage()
		quotedImage := quotedMsg.GetImageMessage()
		quotedVideo := quotedMsg.GetVideoMessage()
		quotedText := extended.GetText()
		messageSender := eventInfo.Info.Sender.User
		groupMessage := eventInfo.Info.IsGroup

		imageMatch := captionIsCommand(eventInfo.Message.GetImageMessage().GetCaption())
		videoMatch := captionIsCommand(eventInfo.Message.GetVideoMessage().GetCaption())
		// check if quoted message with correct caption references media
		quotedMatch := captionIsCommand(quotedText) &&
			(quotedImage != nil || quotedVideo != nil)
		isPrivateMedia := (eventInfo.Message.GetImageMessage() != nil || eventInfo.Message.GetVideoMessage() != nil) && !groupMessage
		if imageMatch || videoMatch || quotedMatch || isPrivateMedia {
			if quotedMatch {
				// replace the actual message struct with quoted media
				if quotedImage != nil {
					eventInfo.Info.MediaType = "image"
					eventInfo.Message.ImageMessage = quotedImage
				} else if quotedVideo != nil {
					// FIXME: gif quoted message just gets set as video
					// currently does not matter as much since both
					// use the same Video handler
					eventInfo.Info.MediaType = "video"
					eventInfo.Message.VideoMessage = quotedVideo
				}
			}
			log.Info("Stickerization Request from ", messageSender)
			log.Debug("message Sender ", messageSender, " *sender ", *sender)
			if *sender != "" && messageSender != *sender {
				return
			}
			go handler.Run(client, eventInfo, *replyTo, ch, convertQueue, loggingQueue)
		}
	}
}

func main() {
	masterDir, _ := filepath.Abs("./master")
	logLevel := flag.String("log-level", "INFO", "Set log level to one of (INFO/DEBUG)")
	replyTo = flag.Bool("reply-to", false, "Set to true to highlight messages to respond to")
	sender = flag.String("sender", "", "Set to number jid that you want to restrict responses to")
	flag.Parse()

	if ll := os.Getenv("LOG_LEVEL"); ll != "" {
		llg := strings.ToUpper(ll)
		logLevel = &llg
	}

	log.SetLevel(utils.GetLogLevel(*logLevel))
	dbLog := waLog.Stdout("Database", *logLevel, true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("sqlite3", fmt.Sprintf("file:%s/db/examplestore.db?_foreign_keys=on", masterDir), dbLog)
	if err != nil {
		panic(err)
	}

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
	utils.FailOnError(err, "Failed to set QoS")

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", *logLevel, true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true
	defer client.Disconnect()

	convertQueue = utils.GetQueue(ch, os.Getenv("CONVERT_TO_WEBP_QUEUE"), true)
	completeQueue := utils.GetQueue(ch, os.Getenv("SEND_WEBP_TO_WHATSAPP_QUEUE"), true)
	loggingQueue = utils.GetQueue(ch, os.Getenv("LOG_METRIC_QUEUE"), false)

	complete := &task.StickerConsumer{
		Client:        client,
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

	if client.Store.ID == nil {
		loginNewClient()
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	utils.ListenForCtrlC("master")
}
