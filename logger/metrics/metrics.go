package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type StickerizationMetric struct {
	InitialMediaLength int
	FinalMediaLength   int
	MediaType          string
	IsGroupMessage     bool
	Country            string
	TimeOfRequest      string
}

type StickerizationGauges struct {
	GroupMessagesGauge   prometheus.Gauge
	PrivateMessagesGauge prometheus.Gauge
	ImageGauge           prometheus.Gauge
	VideoGauge           prometheus.Gauge
	CountryGauge         *prometheus.GaugeVec
}

type Register struct {
	Gauges   StickerizationGauges
	Registry *prometheus.Registry
	Pusher   *push.Pusher
}

func newGauges() StickerizationGauges {

	isgroupQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "RequestSource",
		Name:      "GroupMessages",
		Help:      "Stickerization Requests From Group Chats",
	})
	isprivateQueued := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "Whatsticker",
		Subsystem: "RequestSource",
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
		CountryGauge:         countryQueued,
	}
}

func newRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func initialize(registry *prometheus.Registry, gauges StickerizationGauges) Register {
	registry.MustRegister(
		gauges.CountryGauge,
		gauges.GroupMessagesGauge,
		gauges.PrivateMessagesGauge,
		gauges.ImageGauge,
		gauges.VideoGauge,
	)
	pusher := push.New("http://pushgateway:9091", "StickerizationRequestMetrics").Gatherer(registry)

	return Register{
		Registry: registry,
		Gauges:   gauges,
		Pusher:   pusher,
	}

}

func pushToGateway(pusher *push.Pusher) {
	if err := pusher.Add(); err != nil {
		fmt.Println("Could not push to Pushgateway:", err)
	}
}

func checkAndIncrementMetrics(stickerMetric StickerizationMetric, stickerGauges *StickerizationGauges) {
	if stickerMetric.IsGroupMessage {
		stickerGauges.GroupMessagesGauge.Inc()
	} else {
		stickerGauges.PrivateMessagesGauge.Inc()
	}

	if stickerMetric.MediaType == "image" {
		stickerGauges.ImageGauge.Inc()
	} else if stickerMetric.MediaType == "video" {
		stickerGauges.VideoGauge.Inc()
	}

	if stickerMetric.Country != "" {
		stickerGauges.CountryGauge.With(prometheus.Labels{"country": stickerMetric.Country}).Inc()
	}

}
