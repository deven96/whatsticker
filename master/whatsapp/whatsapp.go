package whatsapp

import (
	"os"
)

const FacebookGraphAPI = "https://graph.facebook.com/v15.0/"

var (
	BearerToken = "Bearer " + os.Getenv("BEARER_ACCESS_TOKEN")
)
