package handler

import (
	"encoding/json"

	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

// WebPFormat is the extension of webp
const WebPFormat = ".webp"

// ImageFileSizeLimit limits file sizes to be converted to 2MB(Mebibytes) in Bytes
const ImageFileSizeLimit = 2097000

// VideoFileSizeLimit limits video to be much smaller cuz over 1000KiB(1MiB)
// seems not to animate
const VideoFileSizeLimit = 1024000

// Handler interface for multiple message types
type Handler interface {
	// Setup the handler, event and context to reply
	SetUp(event *whatsapp.Message, phoneNumberID string)
	// Validate : ensures the media conforms to some standards
	// also sends message to client about issue
	Validate() error
	// Handle : obtains the message to be sent as response
	Handle(ch *amqp.Channel, pushTo *amqp.Queue) error
}

// Run : the appropriate handler using the event type
func Run(event *whatsapp.WhatsappIncomingMessage, ch *amqp.Channel, convertQueue *amqp.Queue, loggingQueue *amqp.Queue) {
	var handle Handler
	entry := event.Entry[0]
	for _, change := range entry.Changes {
		messages := change.Value.Messages
		for _, message := range messages {
			log.Debugf("Running for %s type\n", message.Type)
			messageSender := message.From
			requestTime := message.Time()
			isgroupMessage := message.IsGroup()
			metric := utils.StickerizationMetric{
				InitialMediaLength: 0,
				FinalMediaLength:   0,
				MediaType:          message.Type,
				IsGroupMessage:     isgroupMessage,
				MessageSender:      messageSender,
				TimeOfRequest:      requestTime,
				Validated:          false,
			}
			metricBytes, _ := json.Marshal(&metric)
			switch message.Type {
			case "image":
				log.Debug("Using Image Handler")
				handle = &Image{}
			case "video", "gif":
				log.Debug("Using Video Handler")
				handle = &Video{}
			default:
				failed := whatsapp.TextResponse{
					Response: whatsapp.Response{
						To:      message.From,
						Type:    "text",
						Context: whatsapp.Context{MessageID: message.ID},
					},
					Body: "Bot currently supports sticker creation from (video/images) only",
				}
				textbytes, _ := json.Marshal(&failed)
				whatsapp.SendMessage(textbytes, change.Value.Metadata.PhoneNumberID)

				utils.PublishBytesToQueue(ch, loggingQueue, metricBytes)
				return
			}
			handle.SetUp(&message, change.Value.Metadata.PhoneNumberID)
			invalid := handle.Validate()
			if invalid != nil {
				log.Debugf("Invalid event Data: %s\n", invalid)
				utils.PublishBytesToQueue(ch, loggingQueue, metricBytes)
				return
			}

			if handle.Handle(ch, convertQueue) != nil {
				utils.PublishBytesToQueue(ch, loggingQueue, metricBytes)
			}
		}
	}

}
