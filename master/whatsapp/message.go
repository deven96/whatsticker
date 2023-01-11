package whatsapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Response struct {
	Type             string           `json:"type"`
	Context          Context          `json:"context"`
	To               string           `json:"to"`
	MessagingProduct MessagingProduct `json:"messaging_product"`
}
type MessagingProduct string

// implement the Unmarshaler interface on MessagingProduct
func (e MessagingProduct) MarshalJSON() ([]byte, error) {
	if e == "" {
		return json.Marshal("whatsapp")
	}
	return json.Marshal(e)
}

type Context struct {
	MessageID string `json:"message_id"`
}

type StickerResponse struct {
	Response
	Sticker `json:"sticker"`
}

type Sticker struct {
	ID string `json:"id"`
}

type TextResponse struct {
	Response
	Text Text `json:"text"`
}

type Text struct {
	Body string `json:"body"`
}

func SendMessage(message []byte, phoneNumberID string) error {
	// Create a new request using http
	url := fmt.Sprintf("%s%s/messages", FacebookGraphAPI, phoneNumberID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)
	req.Header.Set("Content-Type", "application/json")

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		bodyText, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("%s\n", bodyText)
		return errors.New(string(bodyText))
	}
	return nil
}
