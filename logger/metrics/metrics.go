package metrics

import (
	"strings"

	"encoding/json"

	"github.com/deven96/whatsticker/utils"
	"github.com/dongri/phonenumber"
	"github.com/prometheus/client_golang/prometheus"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

type StickerizationCounters struct {
	GroupMessagesCounter   prometheus.Counter
	PrivateMessagesCounter prometheus.Counter
	ImageCounter           prometheus.Counter
	VideoCounter           prometheus.Counter
	InvalidMediaCounter    prometheus.Counter
	CountryCounter         *prometheus.CounterVec
	ValidCounter           prometheus.Counter
	InvalidCounter         prometheus.Counter
}

type MetricConsumer struct {
	Counters StickerizationCounters
	Registry *prometheus.Registry
}

func NewCounters() StickerizationCounters {
	isgroupQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "Source",
		Name:      "GroupMessages",
		Help:      "Stickerization Requests From Group Chats",
	})
	isprivateQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "Source",
		Name:      "PrivateMessages",
		Help:      "Stickerization Requests From Private Chats",
	})
	isimageQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "Image",
		Help:      "Stickerization Requests with Image as Media Type",
	})
	isvideoQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "Video",
		Help:      "Stickerization Requests with Video as Media Type",
	})
	isnomediaQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "NoMedia",
		Help:      "Stickerization Requests with Invalid Media Type ",
	})
	isvalidQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "Validated",
		Name:      "Valid",
		Help:      "Valid Stickerization Requests ",
	})
	isinvalidQueued := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "Whatsticker",
		Subsystem: "Validated",
		Name:      "Invalid",
		Help:      "Invalid Stickerization Requests ",
	})
	countryQueued := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "Whatsticker",
			Subsystem: "SenderCountry",
			Name:      "Country",
			Help:      "Stickerization Request Sender's Country",
		},
		[]string{
			"country",
		},
	)
	return StickerizationCounters{
		GroupMessagesCounter:   isgroupQueued,
		PrivateMessagesCounter: isprivateQueued,
		ImageCounter:           isimageQueued,
		VideoCounter:           isvideoQueued,
		InvalidMediaCounter:    isnomediaQueued,
		CountryCounter:         countryQueued,
		ValidCounter:           isvalidQueued,
		InvalidCounter:         isinvalidQueued,
	}
}

func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func Initialize(registry *prometheus.Registry, counters StickerizationCounters) MetricConsumer {
	registry.MustRegister(
		counters.CountryCounter,
		counters.GroupMessagesCounter,
		counters.PrivateMessagesCounter,
		counters.ImageCounter,
		counters.VideoCounter,
		counters.InvalidMediaCounter,
		counters.ValidCounter,
		counters.InvalidCounter,
	)
	return MetricConsumer{
		Registry: registry,
		Counters: counters,
	}

}

func CheckAndIncrementMetrics(stickerMetric utils.StickerizationMetric, stickerCounters *StickerizationCounters) {
	senderCountry := extractCountry(stickerMetric.MessageSender)
	if senderCountry != "" {
		stickerCounters.CountryCounter.With(prometheus.Labels{"country": senderCountry}).Inc()
	}
	if stickerMetric.IsGroupMessage {
		stickerCounters.GroupMessagesCounter.Inc()
	} else {
		stickerCounters.PrivateMessagesCounter.Inc()
	}
	if stickerMetric.MediaType == "image" {
		stickerCounters.ImageCounter.Inc()
	} else if stickerMetric.MediaType == "video" {
		stickerCounters.VideoCounter.Inc()
	} else {
		stickerCounters.InvalidMediaCounter.Inc()
	}
	if stickerMetric.Validated {
		stickerCounters.ValidCounter.Inc()
	} else {
		stickerCounters.InvalidCounter.Inc()
	}
}

func extractCountry(number string) string {
	phoneNumber := strings.Trim(number, "+")
	country := phonenumber.GetISO3166ByNumber(phoneNumber, true)
	log.Debugf("Origin Country: %s", country.CountryName)
	return country.CountryName
}

func (consumer *MetricConsumer) Consume(ch *amqp.Channel, delivery *amqp.Delivery) {
	var stickerMetrics utils.StickerizationMetric
	if err := json.Unmarshal(delivery.Body, &stickerMetrics); err != nil {
		log.Errorf("Error delivering Reject %s", err)
		return
	}
	log.Debugf("Incrementing Metrics %#v", stickerMetrics)
	CheckAndIncrementMetrics(stickerMetrics, &consumer.Counters)
}
