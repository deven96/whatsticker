package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/deven96/whatsticker/metadata"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Video : Logic for when video is received
type Video struct {
	Client        *whatsmeow.Client
	RawPath       string
	ConvertedPath string
	MetadataPath  string
	Format        whatsmeow.MediaType
	Event         *events.Message
	ToReply       *waProto.ContextInfo
}

func (handler *Video) SetUp(client *whatsmeow.Client, event *events.Message, replyTo bool) {
	handler.Client = client
	handler.Format = whatsmeow.MediaVideo
	handler.Event = event
	if replyTo {
		handler.ToReply = &waProto.ContextInfo{
			StanzaId:      &event.Info.ID,
			Participant:   proto.String(event.Info.Sender.String()),
			QuotedMessage: event.Message,
		}
	}
	newpath := filepath.Join(".", "videos/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "videos/converted")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "videos/metadata")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Video) Validate() error {
	if handler == nil {
		return errors.New("Please initialize handler")
	}
	event := handler.Event
	video := event.Message.GetVideoMessage()
	if video.GetSeconds() > VideoFileSecondsLimit {
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String("Your video is longer than 7 seconds"),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		return errors.New("Video too long")
	}
	if video.GetFileLength() > VideoFileSizeLimit {
		length := video.GetFileLength() / 1024
		failed := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(fmt.Sprintf("Your video size %dKb is greater than 1000Kb", length)),
				ContextInfo: handler.ToReply,
			},
		}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		fmt.Printf("File size %d beyond conversion size", video.GetFileLength())
		return errors.New("Video size too large")
	}
	return nil
}

func (handler *Video) Handle() *waProto.Message {
	if handler == nil {
		return nil
	}
	// Download Video
	event := handler.Event
	video := event.Message.GetVideoMessage()
	data, err := handler.Client.Download(video)
	if err != nil {
		fmt.Printf("Failed to download videos: %v\n", err)
		return nil
	}
	exts, _ := mime.ExtensionsByType(video.GetMimetype())
	handler.RawPath = fmt.Sprintf("videos/raw/%s%s", event.Info.ID, exts[0])
	handler.ConvertedPath = fmt.Sprintf("videos/converted/%s%s", event.Info.ID, WebPFormat)
	handler.MetadataPath = fmt.Sprintf("videos/metadata/%s.exif", event.Info.ID)
	fmt.Println(len(data))
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		fmt.Printf("Failed to save video")
		return nil
	}
	// Convert Video (.mp4) to WebP using ffmpeg
	// https://gist.github.com/witmin/1edf926c2886d5c8d9b264d70baf7379
	// http://ffmpeg.org/ffmpeg-all.html#libwebp
	// -an disable audio
	// -q:v (reduce for larger videos)
	// -compression_level 0 for smallest size
	// -lossless 0 sets up for lossy compression
	var qValue int
	switch dataLen := len(data); {
	case dataLen < 300000:
		qValue = 20
	case dataLen < 400000:
		qValue = 10
	default:
		qValue = 5
	}
	fmt.Printf("Q value is %d\n", qValue)
	commandString := fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -filter:v fps=fps=20 -compression_level 0 -q:v %d -loop 0 -preset picture -an -vsync 0 -s 800:800 %s", handler.RawPath, qValue, handler.ConvertedPath)
	cmd := exec.Command("bash", "-c", commandString)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err = cmd.Run()
	if err != nil {
		fmt.Println(outb.String(), "*****", errb.String())
		fmt.Printf("Failed to Convert Video to WebP %s", err)
		return nil
	}

	metadata.GenerateMetadata(handler.ConvertedPath)

	data, err = os.ReadFile(handler.ConvertedPath)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", handler.ConvertedPath, err)
		return nil
	}
	// Upload WebP
	uploaded, err := handler.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
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
			IsAnimated:    proto.Bool(true),
			ContextInfo:   handler.ToReply,
		},
	}
}

func (handler *Video) SendResponse(message *waProto.Message) {
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

func (handler *Video) CleanUp() {
	if handler == nil {
		return
	}
	os.Remove(handler.RawPath)
	os.Remove(handler.ConvertedPath)

}
