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

	rmq "github.com/adjust/rmq/v4"
	"github.com/deven96/whatsticker/task"
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

func (handler *Image) Handle(pushTo rmq.Queue) {
	if handler == nil {
		return
	}
	// Download Image
	event := handler.Event
	image := event.Message.GetImageMessage()
	data, err := handler.Client.Download(image)
	if err != nil {
		fmt.Printf("Failed to download image: %v\n", err)
		return
	}
	exts, _ := mime.ExtensionsByType(image.GetMimetype())
	handler.RawPath = fmt.Sprintf("images/raw/%s%s", event.Info.ID, exts[0])
	handler.ConvertedPath = fmt.Sprintf("images/converted/%s%s", event.Info.ID, WebPFormat)
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		fmt.Printf("Failed to save image: %v", err)
		return
	}
	isgroupMessage := event.Info.IsGroup
	chatBytes, _ := json.Marshal(event.Info.Chat)
	convertTask := &task.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       len(data),
		MediaType:     "image",
		Chat:          chatBytes,
		IsGroup:       isgroupMessage,
	}
	taskBytes, _ := json.Marshal(convertTask)
	pushTo.PublishBytes(taskBytes)
}
