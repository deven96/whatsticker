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

	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

// Image : Logic for when image is received
type Image struct {
	RawPath       string
	ConvertedPath string
	Message       *whatsapp.Message
	PhoneNumberID string
	ImageURL      string
	Len           int
}

func (handler *Image) SetUp(message *whatsapp.Message, phoneNumberID string) {
	handler.Message = message
	handler.PhoneNumberID = phoneNumberID
	newpath := filepath.Join(".", "images/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "images/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Image) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	message := handler.Message
	meta, err := message.ContentLength()
	if err != nil {
		return err
	}
	if meta.FileSize > ImageFileSizeLimit {
		failed := whatsapp.TextResponse{
			Response: whatsapp.Response{
				To:      handler.Message.From,
				Type:    "text",
				Context: whatsapp.Context{MessageID: message.ID},
			},
			Text: whatsapp.Text{
				Body: "Your image is larger than 2MB",
			},
		}
		textbytes, _ := json.Marshal(&failed)
		whatsapp.SendMessage(textbytes, handler.PhoneNumberID)
		return errors.New("File too large")
	}
	handler.ImageURL = meta.URL
	handler.Len = meta.FileSize
	return nil
}

func (handler *Image) Handle(ch *amqp.Channel, pushTo *amqp.Queue) error {
	if handler == nil {
		return errors.New("No Handler")
	}
	// Download Image
	message := handler.Message
	exts, _ := mime.ExtensionsByType(message.MediaType())
	handler.RawPath = fmt.Sprintf("images/raw/%s%s", message.MediaID(), exts[len(exts)-1])
	handler.ConvertedPath = fmt.Sprintf("images/converted/%s%s", message.MediaID(), WebPFormat)
	err := message.DownloadMedia(handler.RawPath, handler.ImageURL)
	if err != nil {
		log.Errorf("Failed to download images: %v\n", err)
		return err
	}
	messageSender := message.From
	requestTime := message.Time()
	isgroupMessage := message.IsGroup()
	convertTask := &utils.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       handler.Len,
		MediaType:     "image",
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
