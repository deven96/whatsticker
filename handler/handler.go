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

// ImageFileSizeLimit limits file sizes to be converted to 2MB(Mebibytes) in Bytes
const ImageFileSizeLimit = 2097000

// VideoFileSizeLimit limits video to be much smaller cuz over 1000KiB(1MiB)
// seems not to animate
const VideoFileSizeLimit = 1024000

// VideoFileSecondsLimit sets video to be less than 7 seconds
// or it no longer animates
const VideoFileSecondsLimit = 7

// Handler interface for multiple message types
type Handler interface {
	// Setup the handler, event and context to reply
	SetUp(client *whatsmeow.Client, event *events.Message, replyTo bool)
	// Validate : ensures the media conforms to some standards
	// also sends message to client about issue
	Validate() error
	// Handle : obtains the message to be sent as response
	Handle() *waProto.Message
	// SendResponse : sends the response
	SendResponse(message *waProto.Message)
	// CleanUp : cleans up any saved files to prevent bloat
	CleanUp()
}

// Run : the appropriate handler using the event type
func Run(client *whatsmeow.Client, event *events.Message, replyTo bool) {
	var handle Handler
	fmt.Printf("Running for %s type\n", event.Info.MediaType)
	switch event.Info.MediaType {
	case "image":
		fmt.Println("Using Image Handler")
		handle = &Image{}
	case "video", "gif":
		fmt.Println("Using Video Handler")
		handle = &Video{}
	default:
		responseMessage := &waProto.Message{Conversation: proto.String("Bot currently supports sticker creation from (video/images) only")}
		client.SendMessage(event.Info.Chat, "", responseMessage)
		return
	}
	defer handle.CleanUp()
	handle.SetUp(client, event, replyTo)
	invalid := handle.Validate()
	if invalid != nil {
		fmt.Printf("%s\n", invalid)
		return
	}
	message := handle.Handle()
	if message == nil {
		fmt.Println("Could not get sticker message to send")
		responseMessage := &waProto.Message{Conversation: proto.String("Sorry I ran into troubles stickerizing that")}
		client.SendMessage(event.Info.Chat, "", responseMessage)
		return
	}
	handle.SendResponse(message)
}

//func RunTest() {
//  metadata.GenerateMetadata("images/converted/STK-20211123-WA0006.webp", "images/metadata/test.exif")
//}
