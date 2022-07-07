package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// Video : Logic for when video is received
type Video struct {
	Client        *whatsmeow.Client
	RawPath       string
	ConvertedPath string
	MetadataPath  string
	Format        whatsmeow.MediaType
	Event         *events.Message
	ToReply       *waProto.ContextInfo
}

func (handler *Video) SetUp(client *whatsmeow.Client, event *events.Message, replyTo bool) {
	handler.Client = client
	handler.Format = whatsmeow.MediaVideo
	handler.Event = event
	if replyTo {
		handler.ToReply = &waProto.ContextInfo{
			StanzaId:      &event.Info.ID,
			Participant:   proto.String(event.Info.Sender.String()),
			QuotedMessage: event.Message,
		}
	}
	newpath := filepath.Join(".", "videos/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "videos/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Video) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	event := handler.Event
	video := event.Message.GetVideoMessage()
	if video.GetSeconds() > VideoFileSecondsLimit {
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String("Your video is longer than 7 seconds"),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		return errors.New("Video too long")
	}
	if video.GetFileLength() > VideoFileSizeLimit {
		length := video.GetFileLength() / 1024
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(fmt.Sprintf("Your video size %dKb is greater than 1000Kb", length)),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		fmt.Printf("File size %d beyond conversion size", video.GetFileLength())
		return errors.New("Video size too large")
	}
	return nil
}

func (handler *Video) Handle(pushTo rmq.Queue) error {
	if handler == nil {
		return errors.New("No Handler")
	}
	// Download Video
	event := handler.Event
	video := event.Message.GetVideoMessage()
	data, err := handler.Client.Download(video)
	if err != nil {
		log.Printf("Failed to download videos: %v\n", err)
		return err
	}
	exts, _ := mime.ExtensionsByType(video.GetMimetype())
	handler.RawPath = fmt.Sprintf("videos/raw/%s%s", event.Info.ID, exts[0])
	handler.ConvertedPath = fmt.Sprintf("videos/converted/%s%s", event.Info.ID, WebPFormat)
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		log.Printf("Failed to save video: %v", err)
		return err
	}
	chatBytes, _ := json.Marshal(event.Info.Chat)
	messageSender := event.Info.Sender.User
	requestTime := event.Info.Timestamp
	isgroupMessage := event.Info.IsGroup
	convertTask := &task.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       len(data),
		MediaType:     "video",
		Chat:          chatBytes,
		IsGroup:       isgroupMessage,
		MessageSender: messageSender,
		TimeOfRequest: requestTime.String(),
	}
	taskBytes, _ := json.Marshal(convertTask)
	pushTo.PublishBytes(taskBytes)
	return nil
}
