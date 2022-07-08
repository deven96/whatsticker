package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"

	"github.com/deven96/whatsticker/metadata"
	amqp "github.com/rabbitmq/amqp091-go"
)

const videoQuality = 35

// 975kb
const maxFileSize = 975000

const animatableVideoLen = 1000000

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

type ConvertConsumer struct {
	PushTo amqp.Queue
}

func (consumer *ConvertConsumer) Consume(ch *amqp.Channel, delivery *amqp.Delivery) {
	var task ConvertTask
	if err := json.Unmarshal([]byte(delivery.Body), &task); err != nil {
		log.Printf("Error unmarshaling delivered body %s", err)
		return
	}

	// perform task
	log.Printf("performing task %#v", task)
	var err error
	switch task.MediaType {
	case "image":
		err = convertImage(task)
	case "video":
		err = convertVideo(task)
	default:
		return
	}
	log.Printf("%s", err)

	if err != nil {
		fmt.Printf("Failed to Convert %s to WebP %s", task.MediaType, err)
		return
	}

	//Function to Get file lenght

	metadata.GenerateMetadata(task.ConvertedPath)
	log.Print("Metadata generation complete")
	publishBytesToQueue(ch, consumer.PushTo, delivery.Body)

	log.Print("Published to %s queue", consumer.PushTo.Name)
	os.Remove(task.MediaPath)
	delivery.Ack(false)
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

func isAnimateable(path string) bool {

	file, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("can not check if video length is animatable: %v", err)
	}
	if len(file) > animatableVideoLen {
		return false
	}
	return true
}

func convertImage(task ConvertTask) error {

	// FIXME: converting to webp's 512x512 skews aspect ratio
	// So Find a way to convert to 512x512 while maintaining perspective before cwebp convertion

	// Convert Image to WebP
	// Using https://developers.google.com/speed/webp/docs/cwebp
	log.Print("Converting Image")
	cmd := *exec.Command("cwebp", task.MediaPath, "-resize", "0", "600", "-o", task.ConvertedPath)
	err := cmd.Run()

	if err == nil {
		log.Print("Conversion Completed")
	}
	return err
}

func convertVideo(task ConvertTask) error {

	var qValue int
	switch {
	case task.DataLen < 350000:
		qValue = 20
	case task.DataLen < 650000:
		qValue = 15
	default:
		qValue = 12
	}
	fmt.Printf("Q value is %d\n", qValue)
	commandString := fmt.Sprintf("ffmpeg -i %s  -filter:v fps=fps=20 -compression_level 0 -q:v %d -loop 0 -preset picture -an -vsync 0 -s 800:800  %s", task.MediaPath, qValue, task.ConvertedPath)
	cmd := *exec.Command("bash", "-c", commandString)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()

	// validate converted video is the right size
	if !(isAnimateable(task.ConvertedPath)) {
		fmt.Println("Reconverting video..\n")
		commandString = fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -fs %d -preset default -loop 0 -an -vsync 0 -vf 'fps=20, scale=800:800' -quality %d -y %s", task.MediaPath, maxFileSize, videoQuality, task.ConvertedPath)
		cmd := *exec.Command("bash", "-c", commandString)
		var outb, errb bytes.Buffer
		cmd.Stdout = &outb
		cmd.Stderr = &errb

		err = cmd.Run()

	}

	return err

}

func publishBytesToQueue(ch *amqp.Channel, q amqp.Queue, bytes []byte) {
	err := ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/json",
			Body:         bytes,
		})
	if err != nil {
		log.Printf("Failed to publish to queue %s", q.Name)
	}
}
