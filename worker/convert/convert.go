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

// 500kb
const maxVideoFileSize = 512000

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
		err = convertVideo(task, 60)
	default:
		return
	}
	if err != nil {
		log.Errorf("Failed to Convert %s to WebP %s", task.MediaType, err)
		return
	}
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

// https://imagemagick.org/script/command-line-options.php#resize
func resizeImage(task utils.ConvertTask) error {
	cmd := *exec.Command("convert", task.MediaPath, "-resize", "512x512", "-background", "black", "-compose", "Copy", "-gravity", "center", "-extent", "512x512", "-quality", "92", task.MediaPath)
	err := cmd.Run()
	return err
}

func convertImage(task utils.ConvertTask) error {
	err := resizeImage(task)
	if err != nil {
		return err
	}
	cmd := *exec.Command("cwebp", task.MediaPath, "-resize", "512", "512", "-o", task.ConvertedPath)
	err = cmd.Run()

	return err
}

func isTargetSize(path string) bool {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("cannot check if video length is animatable: %v", err)
	}
	if len(file) > maxVideoFileSize {
		return false
	}
	return true
}

func convertVideo(task utils.ConvertTask, qValue int) error {
	log.Infof("Q value is %d\n", qValue)
	commandString := fmt.Sprintf("ffmpeg -i %s -fs %d  -filter:v fps=fps=20 -compression_level 0 -q:v %d -loop 0 -preset picture -an -vsync 0 -s 512:512  %s", task.MediaPath, maxVideoFileSize, qValue, task.ConvertedPath)
	cmd := *exec.Command("bash", "-c", commandString)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()

	// validate converted video is the right size
	// FIXME: Use libwep codec and specify file size directly?
	//commandString = fmt.Sprintf("ffmpeg -i %s -vcodec libwebp -fs %d -preset default -loop 0 -an -vsync 0 -vf 'scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(512-iw)/2:(512-ih)/2:color=black' -quality %d -y %s", task.ConvertedPath, maxVideoFileSize, videoQuality, task.ConvertedPath)
	if err == nil && !(isTargetSize(task.ConvertedPath)) {
		log.Info("Reconverting video..\n")
		os.Remove(task.ConvertedPath)
		err = convertVideo(task, qValue-10)
	}

	return err

}
