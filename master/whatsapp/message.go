package whatsapp

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

func SendMessage(message []byte) {
}
