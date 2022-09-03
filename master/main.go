package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"os"
	"path/filepath"
	"strings"

	"github.com/deven96/whatsticker/master/handler"
	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
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

type incomingMessageHandler struct {
	Client *twilio.RestClient
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

	params := &openapi.CreateMessageParams{}
	parsed, err := whatsapp.UnmarshalIncomingMessage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	log.Printf("%#v", parsed.MediaType())
	log.Print(parsed.ContentLength())
	params.SetFrom("whatsapp:+14155238886")
	params.SetTo(*parsed.From)
	params.SetBody("I see you comrade")

	_, err = i.Client.Api.CreateMessage(params)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Message sent successfully!")
	}
}

func main() {
	masterDir, _ := filepath.Abs("./master")
	logLevel := flag.String("log-level", "INFO", "Set log level to one of (INFO/DEBUG)")
	replyTo = flag.Bool("reply-to", false, "Set to true to highlight messages to respond to")
	sender = flag.String("sender", "", "Set to number jid that you want to restrict responses to")
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
	loggingQueue = utils.GetQueue(ch, os.Getenv("LOG_METRIC_QUEUE"), false)
	http.Handle("/incoming", &incomingMessageHandler{
		Client: twilio.NewRestClient(),
	})
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Errorf("Could not start server on %s", *port)
	} else {
		log.Infof("Started Server on %s", *port)
	}

	//  if client.Store.ID == nil {
	//    loginNewClient()
	//  } else {
	//    // Already logged in, just connect
	//    err = client.Connect()
	//    if err != nil {
	//      panic(err)
	//    }
	//  }

	//  utils.ListenForCtrlC("master")
}
