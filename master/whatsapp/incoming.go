package whatsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func UnmarshalIncomingMessage(req *http.Request) (*WhatsappIncomingMessage, error) {
	var r WhatsappIncomingMessage
	err := json.NewDecoder(req.Body).Decode(&r)
	return &r, err
}

type WhatsappIncomingMessage struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Value ChangeValue `json:"value"`
	Field string      `json:"field"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
}

type Message struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
	Sticker struct {
		Media
		Animated string `json:"animated"`
	} `json:"sticker"`
	Image Media `json:"image"`
	Video Media `json:"video"`
}

func (m Message) Time() string {
	i, err := strconv.ParseInt(m.Timestamp, 10, 64)
	if err != nil {
		panic(err)
	}
	tm := time.Unix(i, 0)
	return tm.String()

}

type Media struct {
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	ID       string `json:"id"`
}

type MediaURLResponse struct {
	URL      string `json:"url"`
	FileSize int    `json:"file_size"`
}

type Contact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WhatsappID string `json:"wa_id"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

func (incoming Message) IsMedia() bool {
	return incoming.Type == "video" || incoming.Type == "image"
}

func (incoming Message) MediaType() string {
	switch incoming.Type {
	case "video":
		return incoming.Video.MimeType
	case "image":
		return incoming.Image.MimeType
	case "sticker":
		return incoming.Sticker.MimeType
	default:
		return "unknown"
	}
}

func (incoming Message) MediaID() string {
	switch incoming.Type {
	case "video":
		return incoming.Video.ID
	case "image":
		return incoming.Image.ID
	case "sticker":
		return incoming.Sticker.ID
	default:
		return "unknown"
	}
}

func (incoming Message) IsSticker() bool {
	return incoming.Type == "sticker"
}

func (incoming Message) IsVideo() bool {
	return incoming.Type == "video"
}

func (incoming Message) IsImage() bool {
	return incoming.Type == "image"
}

// IsGroup : Right now twilio doesn't allow adding whatsapp
// business to a group but we keep this here for now
func (incoming Message) IsGroup() bool {
	return false
}

func (incoming Message) ContentLength() (*MediaURLResponse, error) {
	if !incoming.IsMedia() {
		return nil, errors.New("Cannot get ContentLength for non media message")
	}
	id := incoming.MediaID()
	// Create a new request using http
	url := fmt.Sprintf("%s%s", FacebookGraphAPI, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r MediaURLResponse
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (incoming Message) DownloadMedia(path string, url string) error {
	if !incoming.IsMedia() {
		return errors.New("Cannot DownloadMedia for non media message")
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
