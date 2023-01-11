package whatsapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
)

type UploadMediaRequest struct {
	Path             string `json:"file"`
	Type             string `json:"type"`
	MessagingProduct string `json:"messaging_product"`
}

type UploadMediaResponse struct {
	ID string `json:"id"`
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// UploadSticker returns the ID
func UploadSticker(path string, phoneNumberID string) (string, error) {
	// Create a new request using http
	url := fmt.Sprintf("%s%s/media", FacebookGraphAPI, phoneNumberID)
	data, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer data.Close()
	form := new(bytes.Buffer)
	writer := multipart.NewWriter(form)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(data.Name())))
	h.Set("Content-Type", "image/webp")
	fw, err := writer.CreatePart(h)
	if err != nil {
		return "", nil
	}
	_, err = io.Copy(fw, data)
	formField, err := writer.CreateFormField("type")
	_, err = formField.Write([]byte(`image/webp`))

	formField, err = writer.CreateFormField("messaging_product")
	_, err = formField.Write([]byte(`whatsapp`))
	defer writer.Close()

	req, err := http.NewRequest("POST", url, form)
	if err != nil {
		return "", err
	}

	// add authorization header to the req
	req.Header.Add("Authorization", BearerToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		bodyText, _ := ioutil.ReadAll(resp.Body)
		return "", errors.New(string(bodyText))
	}
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
