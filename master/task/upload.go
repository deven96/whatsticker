package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	rmq "github.com/adjust/rmq/v4"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"

	"google.golang.org/protobuf/proto"
)

// CompletedMessage is the proto message sent when done
const CompletedMessage = "Done Stickerizing"

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
	Client *whatsmeow.Client
}

func (consumer *StickerConsumer) Consume(delivery rmq.Delivery) {
	var task ConvertTask
	if err := json.Unmarshal([]byte(delivery.Payload()), &task); err != nil {
		// handle json error
		if err := delivery.Reject(); err != nil {
			// handle reject error
			log.Printf("Error delivering Reject %s", err)
		}
		return
	}
	// perform task
	log.Printf("performing task %#v", task)
	data, err := os.ReadFile(task.ConvertedPath)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", task.ConvertedPath, err)
		return
	}

	// Upload WebP
	uploaded, err := consumer.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		fmt.Printf("Failed to upload file: %v\n", err)
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
}
