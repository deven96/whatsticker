package task

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// CompletedMessage is the proto message sent when done
const CompletedMessage = "Done Stickerizing"

type StickerConsumer struct {
	Client        *whatsmeow.Client
	PushMetricsTo *amqp.Queue
}

func (consumer *StickerConsumer) Execute(ch *amqp.Channel, delivery *amqp.Delivery) {
	var task utils.ConvertTask
	if err := json.Unmarshal(delivery.Body, &task); err != nil {
		// handle reject error
		log.Errorf("Error delivering Reject %s", err)
		return
	}
	stickerMetric := utils.StickerizationMetric{
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
	log.Debugf("performing task %#v", task)
	data, err := os.ReadFile(task.ConvertedPath)
	if err != nil {
		log.Errorf("Failed to read %s: %s\n", task.ConvertedPath, err)
		utils.PublishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
		return
	}
	stickerMetric.FinalMediaLength = len(data)
	// Upload WebP
	uploaded, err := consumer.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		log.Errorf("Failed to upload file: %v\n", err)
		metricsBytes, _ = json.Marshal(&stickerMetric)
		utils.PublishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
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
	utils.PublishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
	delivery.Ack(false)
}
