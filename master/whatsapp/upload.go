package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type UploadMediaRequest struct {
	Path             string `json:"file"`
	Type             string `json:"type"`
	MessagingProduct string `json:"messaging_product"`
}

type UploadMediaResponse struct {
	ID string `json:"id"`
}

// UploadSticker returns the ID
func UploadSticker(path string, phoneNumberID string) (string, error) {
	// Create a new request using http
	url := fmt.Sprintf("%s/%s/media", FacebookGraphAPI, phoneNumberID)
	upload := UploadMediaRequest{
		Type:             "image/webp",
		Path:             path,
		MessagingProduct: "whatsapp",
	}
	// Marshal it into JSON prior to requesting
	uploadJSON, err := json.Marshal(upload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(uploadJSON))
	if err != nil {
		return "", err
	}
	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	var r UploadMediaResponse
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return "", err
	}
	return r.ID, nil
}
