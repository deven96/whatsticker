package handler

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// WebPFormat is the extension of webp
const WebPFormat = ".webp"

// CompletedMessage is the proto message sent when done
const CompletedMessage = "Done Stickerizing"

// FileSizeLimit limits file sizes to be converted to 5MB(Mibibytes) in Bytes
const FileSizeLimit = 5243000

// Handler interface for multiple message types
type Handler interface {
	SetUp(client *whatsmeow.Client, event *events.Message)
	Handle() *waProto.Message
	SendResponse(message *waProto.Message)
	CleanUp()
}

func commonHandle(handler *Handler, name string, dataLimit int) {
}

// Run : the appropriate handler using the event type
func Run(client *whatsmeow.Client, event *events.Message) {
	var handle Handler
	switch event.Info.MediaType {
	case "image":
		fmt.Println("Using Image Handler")
		handle = &Image{}
	case "video":
		fmt.Println("Using Video Handler")
		handle = &Video{}
	default:
		responseMessage := &waProto.Message{Conversation: proto.String("Bot currently supports sticker creation from (video/images) only")}
		client.SendMessage(event.Info.Chat, "", responseMessage)
		return
	}
	defer handle.CleanUp()
	handle.SetUp(client, event)
	message := handle.Handle()
	if message == nil {
		fmt.Println("Could not get sticker message to send")
		responseMessage := &waProto.Message{Conversation: proto.String("Sorry I ran into troubles stickerizing that")}
		client.SendMessage(event.Info.Chat, "", responseMessage)
		return
	}
	handle.SendResponse(message)
}
