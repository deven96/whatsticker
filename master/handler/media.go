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

const whatsappErrorResponse = "Your %s size %d beyond conversion size %d"

type Media struct {
	RawPath       string
	ConvertedPath string
	MetadataPath  string
	Message       *whatsapp.Message
	PhoneNumberID string
	MediaURL      string
	Len           int
	MediaType     string
}

func (handler *Media) SetUp(message *whatsapp.Message, phoneNumberID string) {
	handler.Message = message
	handler.PhoneNumberID = phoneNumberID
	handler.MediaType = message.Type

	newpath := filepath.Join(".", fmt.Sprintf("%ss/raw", handler.MediaType))
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", fmt.Sprintf("%ss/converted", handler.MediaType))
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Media) sizeLimit() int {
	// we dealing with just images and videos so we good
	if handler.MediaType == "image" {
		return ImageFileSizeLimit
	} else {
		return VideoFileSizeLimit
	}
}

func (handler *Media) Validate() error {
	if handler == nil {
		return errors.New("please initialize handler")
	}
	message := handler.Message
	meta, err := message.ContentLength()
	if err != nil {
		return err
	}
	if meta.FileSize > handler.sizeLimit() {
		length := meta.FileSize / 1024
		failed := whatsapp.TextResponse{
			Response: whatsapp.Response{
				To:      handler.Message.From,
				Type:    "text",
				Context: whatsapp.Context{MessageID: message.ID},
			},
			Text: whatsapp.Text{
				Body: fmt.Sprintf(whatsappErrorResponse, handler.MediaType, length, meta.FileSize),
			},
		}
		textbytes, _ := json.Marshal(&failed)
		whatsapp.SendMessage(textbytes, handler.PhoneNumberID)
		return fmt.Errorf("%s too large", handler.MediaType)
	}
	handler.MediaURL = meta.URL
	handler.Len = meta.FileSize
	return nil
}

func (handler *Media) Handle(ch *amqp.Channel, pushTo *amqp.Queue) error {
	if handler == nil {
		return errors.New("no Handler")
	}
	// Download Media
	message := handler.Message
	exts, _ := mime.ExtensionsByType(message.MediaType())
	handler.RawPath = fmt.Sprintf("%ss/raw/%s%s", handler.MediaType, message.MediaID(), exts[0])
	handler.ConvertedPath = fmt.Sprintf("%ss/converted/%s%s", handler.MediaType, message.MediaID(), WebPFormat)
	err := message.DownloadMedia(handler.RawPath, handler.MediaURL)
	if err != nil {
		log.Errorf("Failed to download %ss: %v\n", handler.MediaType, err)
		return err
	}
	messageSender := message.From
	requestTime := message.Time()
	isgroupMessage := message.IsGroup()
	convertTask := &utils.ConvertTask{
		MediaPath:     handler.RawPath,
		ConvertedPath: handler.ConvertedPath,
		DataLen:       handler.Len,
		MediaType:     handler.MediaType,
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
