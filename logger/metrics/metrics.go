package metrics

import (
	"fmt"
	"strings"

	"encoding/json"
	"log"

	"github.com/dongri/phonenumber"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"

	rmq "github.com/adjust/rmq/v4"
)

type StickerizationMetric struct {
	InitialMediaLength int
	FinalMediaLength   int
	MediaType          string
	IsGroupMessage     bool
	MessageSender      string
	TimeOfRequest      string
	Validation         bool
}

type StickerizationGauges struct {
	GroupMessagesGauge   prometheus.Gauge
	PrivateMessagesGauge prometheus.Gauge
	ImageGauge           prometheus.Gauge
	VideoGauge           prometheus.Gauge
	InvalidMediaGauge    prometheus.Gauge
	CountryGauge         *prometheus.GaugeVec
	ValidGauge           prometheus.Gauge
	InvalidGauge         prometheus.Gauge
}

type MetricConsumer struct {
	Gauges   StickerizationGauges
	Registry *prometheus.Registry
	Pusher   *push.Pusher
}

func NewGauges() StickerizationGauges {

	isgroupQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "Source",
		Name:      "GroupMessages",
		Help:      "Stickerization Requests From Group Chats",
	})
	isprivateQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "Source",
		Name:      "PrivateMessages",
		Help:      "Stickerization Requests From Private Chats",
	})
	isimageQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "Image",
		Help:      "Stickerization Requests with Image as Media Type",
	})
	isvideoQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "Video",
		Help:      "Stickerization Requests with Video as Media Type",
	})
	isnomediaQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "MediaType",
		Name:      "NoMedia",
		Help:      "Stickerization Requests with Invalid Media Type ",
	})
	isvalidQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "Validation",
		Name:      "Valid",
		Help:      "Valid Stickerization Requests ",
	})
	isinvalidQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "Validation",
		Name:      "Invalid",
		Help:      "Invalid Stickerization Requests ",
	})

	countryQueued := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "Whatsticker",
			Subsystem: "SenderCountry",
			Name:      "Country",
			Help:      "Stickerization Request Sender's Country",
		},
		[]string{
			"country",
		},
	)
	return StickerizationGauges{
		GroupMessagesGauge:   isgroupQueued,
		PrivateMessagesGauge: isprivateQueued,
		ImageGauge:           isimageQueued,
		VideoGauge:           isvideoQueued,
		InvalidMediaGauge:    isnomediaQueued,
		CountryGauge:         countryQueued,
		ValidGauge:           isvalidQueued,
		InvalidGauge:         isinvalidQueued,
	}
}

func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func Initialize(registry *prometheus.Registry, gauges StickerizationGauges) MetricConsumer {
	registry.MustRegister(
		gauges.CountryGauge,
		gauges.GroupMessagesGauge,
		gauges.PrivateMessagesGauge,
		gauges.ImageGauge,
		gauges.VideoGauge,
	)
	pusher := push.New("http://metrics:9091", "StickerizationRequestMetrics").Gatherer(registry)

	return MetricConsumer{
		Registry: registry,
		Gauges:   gauges,
		Pusher:   pusher,
	}

}

func PushToGateway(pusher *push.Pusher) {
	if err := pusher.Add(); err != nil {
		fmt.Println("Could not push to Pushgateway:", err)
	}
	fmt.Println("Metrics Pushed to Pushgateway:", pusher)
}

func CheckAndIncrementMetrics(stickerMetric StickerizationMetric, stickerGauges *StickerizationGauges) {
	senderCountry := extractCountry(stickerMetric.MessageSender)
	if senderCountry != "" {
		stickerGauges.CountryGauge.With(prometheus.Labels{"country": senderCountry}).Inc()
	}

	if stickerMetric.IsGroupMessage {
		stickerGauges.GroupMessagesGauge.Inc()
	} else {
		stickerGauges.PrivateMessagesGauge.Inc()
	}

	if stickerMetric.MediaType == "image" {
		stickerGauges.ImageGauge.Inc()
	} else if stickerMetric.MediaType == "video" {
		stickerGauges.VideoGauge.Inc()
	} else {
		stickerGauges.InvalidMediaGauge.Inc()
	}

	if stickerMetric.Validation {
		stickerGauges.ValidGauge.Inc()
	} else {
		stickerGauges.InvalidGauge.Inc()
	}
}

func extractCountry(number string) string {
	phoneNumber := strings.Trim(number, "+")
	country := phonenumber.GetISO3166ByNumber(phoneNumber, true)
	fmt.Println(country.CountryName)
	return country.CountryName

}

func (consumer *MetricConsumer) Consume(delivery rmq.Delivery) {
	var stickerMetrics StickerizationMetric
	if err := json.Unmarshal([]byte(delivery.Payload()), &stickerMetrics); err != nil {
		// handle json error
		if err := delivery.Reject(); err != nil {
			// handle reject error
			log.Printf("Error delivering Reject %s", err)
		}
		return
	}
	log.Printf("Incrementing Metrics %#v", stickerMetrics)

	CheckAndIncrementMetrics(stickerMetrics, &consumer.Gauges)
	PushToGateway(consumer.Pusher)
	if err := delivery.Ack(); err != nil {
		// handle ack error
		return
	}

}
