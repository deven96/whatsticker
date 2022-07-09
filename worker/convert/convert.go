package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/exec"

	"github.com/deven96/whatsticker/utils"
	"github.com/deven96/whatsticker/worker/metadata"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

const videoQuality = 35

// 975kb
const maxFileSize = 975000

const animatableVideoLen = 1000000

type ConvertConsumer struct {
	PushTo *amqp.Queue
}

func (consumer *ConvertConsumer) Consume(ch *amqp.Channel, delivery *amqp.Delivery) {
	var task utils.ConvertTask
	if err := json.Unmarshal([]byte(delivery.Body), &task); err != nil {
		log.Errorf("Error unmarshaling delivered body %s", err)
		return
	}

	// perform task
	log.Infof("performing task %#v", task)
	var err error
	switch task.MediaType {
	case "image":
		err = convertImage(task)
	case "video":
		err = convertVideo(task)
	default:
		return
	}

	if err != nil {
		log.Errorf("Failed to Convert %s to WebP %s", task.MediaType, err)
		return
	}

	//Function to Get file lenght

	metadata.GenerateMetadata(task.ConvertedPath)
	utils.PublishBytesToQueue(ch, consumer.PushTo, delivery.Body)

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
		log.Debugf("%s %d %d\n", path, im.Width, im.Height)
		width = im.Width
		height = im.Height
	} else {
		log.Error("Impossible to open the file: ", err)
	}
	return width, height
}

func isAnimateable(path string) bool {

	file, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("can not check if video length is animatable: %v", err)
	}
	if len(file) > animatableVideoLen {
		return false
	}
	return true
}

func convertImage(task utils.ConvertTask) error {

	// FIXME: converting to webp's 512x512 skews aspect ratio
	// So Find a way to convert to 512x512 while maintaining perspective before cwebp convertion

	// Convert Image to WebP
	// Using https://developers.google.com/speed/webp/docs/cwebp
	cmd := *exec.Command("cwebp", task.MediaPath, "-resize", "0", "600", "-o", task.ConvertedPath)
	err := cmd.Run()

	return err
}

func convertVideo(task utils.ConvertTask) error {

	var qValue int
	switch {
	case task.DataLen < 350000:
		qValue = 20
	case task.DataLen < 650000:
		qValue = 15
	default:
		qValue = 12
	}
	log.Debugf("Q value is %d\n", qValue)
	commandString := fmt.Sprintf("ffmpeg -i %s  -filter:v fps=fps=20 -compression_level 0 -q:v %d -loop 0 -preset picture -an -vsync 0 -s 800:800  %s", task.MediaPath, qValue, task.ConvertedPath)
	cmd := *exec.Command("bash", "-c", commandString)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()

	// validate converted video is the right size
	if !(isAnimateable(task.ConvertedPath)) {
		log.Debugf("Reconverting video..\n")
		commandString = fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -fs %d -preset default -loop 0 -an -vsync 0 -vf 'fps=20, scale=800:800' -quality %d -y %s", task.MediaPath, maxFileSize, videoQuality, task.ConvertedPath)
		cmd := *exec.Command("bash", "-c", commandString)
		var outb, errb bytes.Buffer
		cmd.Stdout = &outb
		cmd.Stderr = &errb

		err = cmd.Run()

	}

	return err

}
