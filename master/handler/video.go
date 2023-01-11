package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

// Video : Logic for when video is received
type Video struct {
	RawPath       string
	ConvertedPath string
	MetadataPath  string
	Message       *whatsapp.Message
	PhoneNumberID string
	VideoURL      string
	Len           int
}

func (handler *Video) SetUp(message *whatsapp.Message, phoneNumberID string) {
	handler.Message = message
	handler.PhoneNumberID = phoneNumberID
	newpath := filepath.Join(".", "videos/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "videos/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Video) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	message := handler.Message
	meta, err := message.ContentLength()
	if err != nil {
		return err
	}
	if meta.FileSize > VideoFileSizeLimit {
		length := meta.FileSize / 1024
		failed := whatsapp.TextResponse{
			Response: whatsapp.Response{
				To:      handler.Message.From,
				Type:    "text",
				Context: whatsapp.Context{MessageID: message.ID},
			},
			Text: whatsapp.Text{
				Body: fmt.Sprintf("File size %d beyond conversion size %d", length, meta.FileSize),
			},
		}
		textbytes, _ := json.Marshal(&failed)
		whatsapp.SendMessage(textbytes, handler.PhoneNumberID)

		log.Warnf("File size %d beyond conversion size", meta.FileSize)
		return errors.New("Video size too large")
	}
	handler.VideoURL = meta.URL
	handler.Len = meta.FileSize
	return nil
}

func (handler *Video) Handle(ch *amqp.Channel, pushTo *amqp.Queue) error {
	if handler == nil {
		return errors.New("No Handler")
	}
	// Download Video
	message := handler.Message
	exts, _ := mime.ExtensionsByType(message.MediaType())
	handler.RawPath = fmt.Sprintf("videos/raw/%s%s", message.MediaID(), exts[0])
	handler.ConvertedPath = fmt.Sprintf("videos/converted/%s%s", message.MediaID(), WebPFormat)
	err := message.DownloadMedia(handler.RawPath, handler.VideoURL)
	if err != nil {
		log.Errorf("Failed to download videos: %v\n", err)
		return err
	}
	messageSender := message.From
	requestTime := message.Time()
	isgroupMessage := message.IsGroup()
	convertTask := &utils.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       handler.Len,
		MediaType:     "video",
		MessageID:     message.ID,
		From:          message.From,
		PhoneNumberID: handler.PhoneNumberID,
		IsGroup:       isgroupMessage,
		MessageSender: messageSender,
		TimeOfRequest: requestTime,
	}
	taskBytes, _ := json.Marshal(convertTask)
	utils.PublishBytesToQueue(ch, pushTo, taskBytes)
	return nil
}
