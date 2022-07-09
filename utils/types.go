package utils

import "os"

type AMQPConfig struct {
	Uri     string // AMQP URI
	Verbose bool   // enable verbose output of message data
}

func GetAMQPConfig() *AMQPConfig {
	return &AMQPConfig{
		Uri:     os.Getenv("AMQP_URI"),
		Verbose: false,
	}
}

// ConvertTask
type ConvertTask struct {
	MediaPath     string
	ConvertedPath string
	DataLen       int
	MediaType     string
	Chat          []byte
	IsGroup       bool
	MessageSender string
	TimeOfRequest string //time.Time
}

// StickerizationMetric
type StickerizationMetric struct {
	InitialMediaLength int
	FinalMediaLength   int
	MediaType          string
	IsGroupMessage     bool
	MessageSender      string
	TimeOfRequest      string
	Validated          bool
}
