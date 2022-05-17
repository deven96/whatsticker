package main

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var client *whatsmeow.Client
const command = "stickerize deven96"

func loginNewClient() {
	// No ID stored, new login
	qrChan, _ := client.GetQRChannel(context.Background())
	err := client.Connect()
	if err != nil {
		panic(err)
	}
	for evt := range qrChan {
		if evt.Event == "code" {
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			fmt.Println("QR code:", evt.Code)
		} else {
			fmt.Println("Login event:", evt.Event)
		}
	}
}

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func eventHandler(evt interface{}) {
	
	switch eventInfo := evt.(type) {
	case *events.Message:
		if strings.ToLower(eventInfo.Message.ImageMessage.GetCaption()) == command {
			
			if eventInfo.Info.MediaType == "image" {
				handleImageStickers(eventInfo)
			} else {
				responseMessage := &waProto.Message{Conversation: proto.String("Bot currently supports sticker creation from images only")}
				client.SendMessage(eventInfo.Info.Chat, "", responseMessage)
			}
		}
	}
}

func handleImageStickers(event *events.Message) {
	// Download Image
	image := event.Message.GetImageMessage()
	data, err := client.Download(image)
	if err != nil {
		fmt.Printf("Failed to download image: %v\n", err)
		return
	}
	exts, _ := mime.ExtensionsByType(image.GetMimetype())
	newpath := filepath.Join(".", "images/raw")
	os.MkdirAll(newpath, os.ModePerm)
	newpath = filepath.Join(".", "images/converted")
	os.MkdirAll(newpath, os.ModePerm)
	path := fmt.Sprintf("images/raw/%s%s", event.Info.ID, exts[0])
	outputPath := fmt.Sprintf("images/converted/%s%s", event.Info.ID, ".webp")
	err = os.WriteFile(path, data, 0600)
	if err != nil {
		fmt.Printf("Failed to save image: %v", err)
		return
	}
	// Convert Image to WebP
	// Using https://developers.google.com/speed/webp/docs/cwebp
	// Image size must be 512x512 so make use of inbuilt cwebp resize
	cmd := exec.Command("cwebp", "-resize", "512", "512", path, "-o", outputPath)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Failed to Convert Image to WebP")
		return
	}

	data, err = os.ReadFile(outputPath)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", outputPath, err)
		return
	}
	// Upload WebP
	uploaded, err := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		fmt.Printf("Failed to upload file: %v\n", err)
		return
	}

	os.Remove(path)
	os.Remove(outputPath)

	// Send WebP as sticker
	msg := &waProto.Message{
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
	completed := &waProto.Message{Conversation: proto.String("Done Stickerizing")}
	client.SendMessage(event.Info.Chat, "", msg)
	client.SendMessage(event.Info.Chat, "", completed)
}

func main() {
	dbLog := waLog.Stdout("Database", "INFO", true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "INFO", true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		loginNewClient()
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	listenForCtrlC()
	client.Disconnect()
}
