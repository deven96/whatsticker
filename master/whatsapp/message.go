package whatsapp

import (
	"bytes"
	"fmt"
	"net/http"
)

type Response struct {
	Type    string  `json:"type"`
	Context Context `json:"context"`
	To      string  `json:"to"`
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
	Body string `json:"body"`
}

func SendMessage(message []byte, phoneNumberID string) error {
	// Create a new request using http
	url := fmt.Sprintf("%s/%s/messages", FacebookGraphAPI, phoneNumberID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)

	// Send req using http Client
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	return nil
}
