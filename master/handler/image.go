package handler

import (
	"encoding/json"
	"errors"
	"fmt"

	// Import all possible image codecs
	// for getImageDimensions
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"mime"
	"os"
	"path/filepath"

	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Image : Logic for when image is received
type Image struct {
	Client        *whatsmeow.Client
	RawPath       string
	ConvertedPath string
	Format        whatsmeow.MediaType
	Event         *events.Message
	ToReply       *waProto.ContextInfo
}

func (handler *Image) SetUp(client *whatsmeow.Client, event *events.Message, replyTo bool) {
	handler.Client = client
	handler.Format = whatsmeow.MediaImage
	handler.Event = event
	if replyTo {
		handler.ToReply = &waProto.ContextInfo{
			StanzaId:      &event.Info.ID,
			Participant:   proto.String(event.Info.Sender.String()),
			QuotedMessage: event.Message,
		}
	}
	newpath := filepath.Join(".", "images/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "images/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Image) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	event := handler.Event
	image := event.Message.GetImageMessage()
	if image.GetFileLength() > ImageFileSizeLimit {
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String("Your file is larger than 2MB"),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		return errors.New("File too large")
	}
	return nil
}

func (handler *Image) Handle(ch *amqp.Channel, pushTo *amqp.Queue) error {
	if handler == nil {
		return errors.New("No Handler")
	}
	// Download Image
	event := handler.Event
	image := event.Message.GetImageMessage()
	data, err := handler.Client.Download(image)
	if err != nil {
		log.Errorf("Failed to download image: %v\n", err)
		return err
	}
	exts, _ := mime.ExtensionsByType(image.GetMimetype())
	handler.RawPath = fmt.Sprintf("images/raw/%s%s", event.Info.ID, exts[0])
	handler.ConvertedPath = fmt.Sprintf("images/converted/%s%s", event.Info.ID, WebPFormat)
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		log.Errorf("Failed to save image: %v", err)
		return err
	}
	chatBytes, _ := json.Marshal(event.Info.Chat)
	messageSender := event.Info.Sender.User
	requestTime := event.Info.Timestamp
	isgroupMessage := event.Info.IsGroup
	convertTask := &utils.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       len(data),
		MediaType:     "image",
		Chat:          chatBytes,
		IsGroup:       isgroupMessage,
		MessageSender: messageSender,
		TimeOfRequest: requestTime.String(),
	}
	taskBytes, _ := json.Marshal(convertTask)
	utils.PublishBytesToQueue(ch, pushTo, taskBytes)
	return nil
}
