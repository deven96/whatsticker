package task

import (
	"encoding/json"
	"os"

	"github.com/deven96/whatsticker/master/whatsapp"
	"github.com/deven96/whatsticker/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

// CompletedMessage is the proto message sent when done
const CompletedMessage = "Done Stickerizing"

type StickerConsumer struct {
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
	id, err := whatsapp.UploadSticker(task.ConvertedPath, task.PhoneNumberID)
	if err != nil {
		log.Errorf("Failed to upload file: %v\n", err)
		metricsBytes, _ = json.Marshal(&stickerMetric)
		utils.PublishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
		return
	}
	sticker := whatsapp.StickerResponse{
		Response: whatsapp.Response{
			To:      task.From,
			Type:    "text",
			Context: whatsapp.Context{MessageID: task.MessageID},
		},
		Sticker: whatsapp.Sticker{
			ID: id,
		},
	}
	stickerBytes, _ := json.Marshal(&sticker)
	whatsapp.SendMessage(stickerBytes)

	os.Remove(task.ConvertedPath)
	stickerMetric.Validated = true
	metricsBytes, _ = json.Marshal(&stickerMetric)
	utils.PublishBytesToQueue(ch, consumer.PushMetricsTo, []byte(metricsBytes))
	delivery.Ack(false)
}
