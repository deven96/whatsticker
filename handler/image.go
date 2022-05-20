package handler

import (
	"context"
	"errors"
	"fmt"
	"image"

	// Import all possible image codecs
	// for getImageDimensions
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Image : Logic for when image is received
type Image struct {
	Client        *whatsmeow.Client
	RawPath       string
	ConvertedPath string
	Format        whatsmeow.MediaType
	Event         *events.Message
	ToReply       *waProto.ContextInfo
}

func (handler *Image) SetUp(client *whatsmeow.Client, event *events.Message, replyTo bool) {
	handler.Client = client
	handler.Format = whatsmeow.MediaImage
	handler.Event = event
	if replyTo {
		handler.ToReply = &waProto.ContextInfo{
			StanzaId:      &event.Info.ID,
			Participant:   proto.String(event.Info.Sender.String()),
			QuotedMessage: event.Message,
		}
	}
	newpath := filepath.Join(".", "images/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "images/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Image) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	event := handler.Event
	image := event.Message.GetImageMessage()
	if image.GetFileLength() > ImageFileSizeLimit {
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String("Your file is larger than 2MB"),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		return errors.New("File too large")
	}
	return nil
}

func (handler *Image) Handle() *waProto.Message {
	if handler == nil {
		return nil
	}
	// Download Image
	event := handler.Event
	image := event.Message.GetImageMessage()
	data, err := handler.Client.Download(image)
	if err != nil {
		fmt.Printf("Failed to download image: %v\n", err)
		return nil
	}
	exts, _ := mime.ExtensionsByType(image.GetMimetype())
	handler.RawPath = fmt.Sprintf("images/raw/%s%s", event.Info.ID, exts[0])
	handler.ConvertedPath = fmt.Sprintf("images/converted/%s%s", event.Info.ID, WebPFormat)
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		fmt.Printf("Failed to save image: %v", err)
		return nil
	}
	// FIXME: converting to webp's 512x512 skews aspect ratio
	// So Find a way to convert to 512x512 while maintaining perspective before cwebp convertion

	// Convert Image to WebP
	// Using https://developers.google.com/speed/webp/docs/cwebp
	cmd := exec.Command("cwebp", handler.RawPath, "-o", handler.ConvertedPath)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Failed to Convert Image to WebP")
		return nil
	}

	data, err = os.ReadFile(handler.ConvertedPath)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", handler.ConvertedPath, err)
		return nil
	}

	// Upload WebP
	uploaded, err := handler.Client.Upload(context.Background(), data, handler.Format)
	if err != nil {
		fmt.Printf("Failed to upload file: %v\n", err)
		return nil
	}

	// Send WebP as sticker
	return &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   handler.ToReply,
		},
	}
}

func (handler *Image) SendResponse(message *waProto.Message) {
	if handler == nil {
		return
	}
	event := handler.Event
	completed := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String(CompletedMessage),
			ContextInfo: handler.ToReply,
		},
	}
	handler.Client.SendMessage(event.Info.Chat, "", message)
	handler.Client.SendMessage(event.Info.Chat, "", completed)
}

func (handler *Image) CleanUp() {
	if handler == nil {
		return
	}
	os.Remove(handler.RawPath)
	os.Remove(handler.ConvertedPath)

}

// FIXME: Probably use this image dimensions to find a way
// to maintain aspect ratio on an image before uploading as sticker
func getImageDimensions(path string) (int, int) {
	width, height := 0, 0
	if reader, err := os.Open(path); err == nil {
		defer reader.Close()
		im, _, _ := image.DecodeConfig(reader)
		fmt.Printf("%s %d %d\n", path, im.Width, im.Height)
		width = im.Width
		height = im.Height
	} else {
		fmt.Println("Impossible to open the file:", err)
	}
	return width, height
}
