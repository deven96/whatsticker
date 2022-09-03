// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    incomingMessage, err := UnmarshalIncomingMessage(bytes)
//    bytes, err = incomingMessage.Marshal()

package whatsapp

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

func UnmarshalIncomingMessage(req *http.Request) (*IncomingMessage, error) {
	var r IncomingMessage
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, errors.New("Could not read request body")
	}
	intermediate := map[string]*string{}
	parsed, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, errors.New("Could not parse request body")
	}
	for key, value := range parsed {
		fmt.Println(key, value)
		if len(value) == 1 {
			intermediate[key] = &value[0]
		} else {
			intermediate[key] = nil
		}
	}

	mapstructure.Decode(intermediate, &r)
	return &r, err
}

type IncomingMessage struct {
	ProfileName       *string
	APIVersion        *string
	SmsSid            *string
	WaId              *string
	SmsStatus         *string
	Body              *string
	To                *string
	MediaContentType0 *string
	SmSMessageSid     *string
	MessageSid        *string
	AccountSid        *string
	MediaUrl0         *string
	From              *string
	NumMedia          *string
	NumSegments       *string
	ReferralNumMedia  *string
	Latitude          *float64
	Longitude         *float64
	Address           *string
	Label             *string
}

func (incoming IncomingMessage) IsMedia() bool {
	return incoming.MediaContentType0 != nil
}

func (incoming IncomingMessage) IsStickerizable() bool {
	return incoming.IsMedia() && incoming.IsSticker()
}

func (incoming IncomingMessage) IsSticker() bool {
	return strings.HasSuffix(incoming.MediaType(), "webp")
}

func (incoming IncomingMessage) IsVideo() bool {
	if incoming.IsMedia() && !incoming.IsSticker() && strings.HasPrefix(incoming.MediaType(), "video") {
		return true
	}
	return false
}

func (incoming IncomingMessage) IsImage() bool {
	if incoming.IsMedia() && strings.HasPrefix(incoming.MediaType(), "image") {
		return true
	}
	return false
}

func (incoming IncomingMessage) IsContact() bool {
	if incoming.IsMedia() && strings.HasSuffix(incoming.MediaType(), "vcard") {
		return true
	}
	return false
}

func (incoming IncomingMessage) IsLocation() bool {
	return incoming.Latitude != nil && incoming.Longitude != nil
}

func (incoming IncomingMessage) MediaType() string {
	if incoming.MediaContentType0 != nil {
		return *incoming.MediaContentType0
	}
	return ""
}

// IsGroup : Right now twilio doesn't allow adding whatsapp
// business to a group but we keep this here for now
func (incoming IncomingMessage) IsGroup() bool {
	return false
}

func (incoming IncomingMessage) ContentLength() (int64, error) {
	if !incoming.IsMedia() {
		return 0, errors.New("Cannot get ContentLength for non media message")
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Head(*incoming.MediaUrl0)

	if err != nil {
		return 0, err
	}
	return resp.ContentLength, nil
}

func (incoming IncomingMessage) DownloadMedia(path string) error {
	if !incoming.IsMedia() {
		return errors.New("Cannot DownloadMedia for non media message")
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get(*incoming.MediaUrl0)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
