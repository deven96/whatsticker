package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"

	"google.golang.org/protobuf/proto"
)

// CompletedMessage is the proto message sent when done
const CompletedMessage = "Done Stickerizing"

type StickerizationMetric struct {
	InitialMediaLength int
	FinalMediaLength   int
	MediaType          string
	IsGroupMessage     bool
	MessageSender      string
	TimeOfRequest      string
	Validated          bool
}
type ConvertTask struct {
	MediaPath     string
	ConvertedPath string
	DataLen       int
	MediaType     string
	Chat          []byte
	IsGroup       bool
	MessageSender string
	TimeOfRequest string
}
type StickerConsumer struct {
	Client        *whatsmeow.Client
	PushMetricsTo amqp.Queue
}

func (consumer *StickerConsumer) Execute(ch *amqp.Channel, delivery *amqp.Delivery) {
	var task ConvertTask
	if err := json.Unmarshal(delivery.Body, &task); err != nil {
		// handle reject error
		log.Printf("Error delivering Reject %s", err)
		return
	}
	stickerMetric := StickerizationMetric{
		InitialMediaLength: task.DataLen,
		FinalMediaLength:   0,
		MediaType:          task.MediaType,
		IsGroupMessage:     task.IsGroup,
		MessageSender:      task.MessageSender,
		TimeOfRequest:      task.TimeOfRequest,
		Validated:          false,
	}
	metricsBytes, _ := json.Marshal(&stickerMetric)
	// perform task
	log.Printf("performing task %#v", task)
	data, err := os.ReadFile(task.ConvertedPath)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", task.ConvertedPath, err)
		publishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
		return
	}
	stickerMetric.FinalMediaLength = len(data)
	// Upload WebP
	uploaded, err := consumer.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		fmt.Printf("Failed to upload file: %v\n", err)
		metricsBytes, _ = json.Marshal(&stickerMetric)
		publishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
		return
	}
	sticker := &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}
	if task.MediaType == "video" {
		sticker.StickerMessage.IsAnimated = proto.Bool(true)
	}
	chat := types.JID{}
	json.Unmarshal(task.Chat, &chat)
	completed := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(CompletedMessage),
		},
	}
	consumer.Client.SendMessage(chat, "", sticker)
	if task.IsGroup {
		consumer.Client.SendMessage(chat, "", completed)
	}
	os.Remove(task.ConvertedPath)
	stickerMetric.Validated = true
	metricsBytes, _ = json.Marshal(&stickerMetric)
	publishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
	delivery.Ack(false)
}

func publishBytesToQueue(ch *amqp.Channel, q amqp.Queue, bytes []byte) {
	err := ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/json",
			Body:         bytes,
		})
	if err != nil {
		log.Printf("Failed to publish to queue %s", q.Name)
	}
}
