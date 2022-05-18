package handler

import (
	"bytes"
	"context"
	"fmt"
	"image"
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

// Video : Logic for when video is received
type Video struct {
	Client        *whatsmeow.Client
	RawPath       string
	ConvertedPath string
	Format        whatsmeow.MediaType
	Event         *events.Message
}

func (handler *Video) SetUp(client *whatsmeow.Client, event *events.Message) {
	handler.Client = client
	handler.Format = whatsmeow.MediaVideo
	handler.Event = event
	newpath := filepath.Join(".", "videos/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "videos/converted")
	os.MkdirAll(newpath, os.ModePerm)
}

func (handler *Video) Handle() *waProto.Message {
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
	fmt.Println(len(data))
	err = os.WriteFile(handler.RawPath, data, 0600)
	if err != nil {
		fmt.Printf("Failed to save video")
		return nil
	}
	// Convert Video (.mp4) to WebP using ffmpeg
	// https://gist.github.com/witmin/1edf926c2886d5c8d9b264d70baf7379
	// Make use of lossy (-lossless 0) mode and compression_level set to 0 for fastest
	// encoding time
	// http://ffmpeg.org/ffmpeg-all.html#libwebp
	commandString := fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -filter:v fps=fps=20 -compression_level 0 -loop 0 -preset default -an -vsync 0 -s 800:600 %s", handler.RawPath, handler.ConvertedPath)
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

	fileStat, err := os.Stat(handler.RawPath)
	if err != nil {
		fmt.Printf("Unable to stat video %s: %s\n", handler.ConvertedPath, err)
	}
	if fileStat.Size() > VideoFileSizeLimit {
		failed := &waProto.Message{Conversation: proto.String("Your video size is greater than 600Kb")}
		handler.Client.SendMessage(event.Info.Chat, "", failed)
		fmt.Printf("File size %d beyond conversion size", fileStat.Size())
		return nil
	}

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
		},
	}
}

func (handler *Video) SendResponse(message *waProto.Message) {
	event := handler.Event
	completed := &waProto.Message{Conversation: proto.String(CompletedMessage)}
	handler.Client.SendMessage(event.Info.Chat, "", message)
	handler.Client.SendMessage(event.Info.Chat, "", completed)
}

func (handler *Video) CleanUp() {
	os.Remove(handler.RawPath)
	os.Remove(handler.ConvertedPath)

}

func getVideoDimensions(path string) (int, int) {
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
