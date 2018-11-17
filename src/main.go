package main

import (
	"fmt"
	"github.com/alexsasharegan/dotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strings"
	"time"
)

var channels []string
var streamsData = map[string][]prometheus.Gauge{}
var lastScrape, tokenRemaining prometheus.Gauge

func setupVars(users []UserData) {
	// setup the vars for prometheus
	lastScrape = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "twitch",
		Name:      "last_scrape",
		Help:      "last scrape time",
	})

	tokenRemaining = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "twitch",
		Name:      "token_remaining",
		Help:      "Token remaining",
	})

	for _, user := range users {
		measures := []prometheus.Gauge{
			promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "twitch",
				Name:      "online",
				Help:      "Is the streamer is online",
				ConstLabels: prometheus.Labels{
					"name": user.DisplayName,
				},
			}),
			promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "twitch",
				Name:      "viewers",
				Help:      "Count the number of viewers",
				ConstLabels: prometheus.Labels{
					"name": user.DisplayName,
				},
			}),
			promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "twitch",
				Name:      "started_at",
				Help:      "Started at",
				ConstLabels: prometheus.Labels{
					"name": user.DisplayName,
				},
			}),
		}
		streamsData[user.ID] = measures
	}
}

func scrapeStreams(twitch *Client) {
	go func() {
		onlineStream := map[string]bool{}
		for {
			// reset online status
			for k := range streamsData {
				onlineStream[k] = false
			}

			// download data
			streamInfos, token, err := twitch.GetStreams(channels)
			if err != nil {
				fmt.Printf("Error getting twitch user: %v", err)
			}

			//infos
			tokenRemaining.Set(float64(token))
			lastScrape.Set(float64(time.Now().Unix()))

			// process online stream
			for _, streamInfo := range streamInfos {
				fmt.Printf("Stream %s: %s - %s, %d viewers\n", streamInfo.UserID, streamInfo.UserName, streamInfo.Title, streamInfo.ViewerCount)
				onlineStream[streamInfo.UserID] = true
				streamsData[streamInfo.UserID][0].Set(1)
				streamsData[streamInfo.UserID][1].Set(float64(streamInfo.ViewerCount))
				streamsData[streamInfo.UserID][2].Set(float64(streamInfo.StartedAt.Unix()))
			}

			// clean offline stream
			for userID, online := range onlineStream {
				if !online {
					streamsData[userID][0].Set(0)
					streamsData[userID][1].Set(0)
					streamsData[userID][2].Set(0)
				}
			}

			time.Sleep(30 * time.Second)
		}
	}()
}

type Config struct {
	LogLevel string
	ClientID string
	Channels []string
}

var cfg Config

func main() {
	_ = dotenv.Load()

	// Setup logging before anything else
	if len(os.Getenv("LOG_LEVEL")) == 0 {
		cfg.LogLevel = "info"
	} else {
		cfg.LogLevel = os.Getenv("LOG_LEVEL")
	}
	switch cfg.LogLevel {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}

	twitch := NewClient(os.Getenv("CLIENT_KEY"))
	channels = strings.Split(os.Getenv("CHANNELS"), ",")
	log.Debugf("Channels: %s", channels)

	users, err := twitch.GetUsers(channels)
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	setupVars(users)
	scrapeStreams(twitch)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
