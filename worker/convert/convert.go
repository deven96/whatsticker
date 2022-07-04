package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"strings"

	rmq "github.com/adjust/rmq/v4"
	"github.com/deven96/whatsticker/metadata"
	"github.com/dongri/phonenumber"
)

type ConvertTask struct {
	MediaPath     string
	ConvertedPath string
	DataLen       int
	MediaType     string
	Chat          []byte
	IsGroup       bool
	MessageSender string
	TimeOfRequest string //time.Time
}

type StickerizationMetric struct {
	InitialMediaLength int
	FinalMediaLength   int
	MediaType          string
	IsGroupMessage     bool
	Country            string
	TimeOfRequest      string
}

type ConvertConsumer struct {
	PushTo        rmq.Queue
	PushMetricsTo rmq.Queue
}

func extractCountry(number string) string {
	phoneNumber := strings.Trim(number, "+")
	country := phonenumber.GetISO3166ByNumber(phoneNumber, true)
	fmt.Println(country.CountryName)

	return country.CountryName

}

func getDataLength(filePath string) int {

	data, err := os.ReadFile(filePath)

	if err != nil {
		fmt.Println(err.Error())
	}

	return len(data)
}

func (consumer *ConvertConsumer) Consume(delivery rmq.Delivery) {
	var task ConvertTask
	var cmd *exec.Cmd
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
	switch task.MediaType {
	case "image":
		// FIXME: converting to webp's 512x512 skews aspect ratio
		// So Find a way to convert to 512x512 while maintaining perspective before cwebp convertion

		// Convert Image to WebP
		// Using https://developers.google.com/speed/webp/docs/cwebp
		cmd = exec.Command("cwebp", task.MediaPath, "-resize", "0", "600", "-o", task.ConvertedPath)
	case "video":
		var qValue int
		switch {
		case task.DataLen < 300000:
			qValue = 20
		case task.DataLen < 400000:
			qValue = 10
		default:
			qValue = 5
		}
		fmt.Printf("Q value is %d\n", qValue)
		commandString := fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -filter:v fps=fps=20 -compression_level 0 -q:v %d -loop 0 -preset picture -an -vsync 0 -s 800:800 %s", task.MediaPath, qValue, task.ConvertedPath)
		cmd = exec.Command("bash", "-c", commandString)
		var outb, errb bytes.Buffer
		cmd.Stdout = &outb
		cmd.Stderr = &errb
	default:
		return
	}

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to Convert %s to WebP %s", task.MediaType, err)
		return
	}

	//Function to Get file lenght
	convertedDataLength := getDataLength(task.ConvertedPath)
	senderCountry := extractCountry(task.MessageSender)

	metadata.GenerateMetadata(task.ConvertedPath)
	consumer.PushTo.PublishBytes([]byte(delivery.Payload()))

	os.Remove(task.MediaPath)

	if err := delivery.Ack(); err != nil {
		// handle ack error
		return
	}

	//Push Metrics
	stickerMetric := &StickerizationMetric{
		InitialMediaLength: task.DataLen,
		FinalMediaLength:   convertedDataLength,
		MediaType:          task.MediaType,
		IsGroupMessage:     task.IsGroup,
		Country:            senderCountry,
		TimeOfRequest:      task.TimeOfRequest,
	}
	metricsBytes, _ := json.Marshal(stickerMetric)
	consumer.PushMetricsTo.PublishBytes([]byte(metricsBytes))
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
