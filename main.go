package main

import (
	"TwitchLanStats/client"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var channels []string
var streamsData = map[string][]prometheus.Gauge{}
var lastScrape, tokenRemaining prometheus.Gauge

func setupVars(users []client.UserData) {
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

func scrapeStreams(twitch *client.Client) {
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

func main() {
	twitch := client.NewClient(os.Getenv("CLIENT_KEY"))

	fd, err := os.Open("channels.json")
	if err != nil {
		panic("Error opening file")
	}
	content, err := ioutil.ReadAll(fd)
	fd.Close()

	json.Unmarshal(content, &channels)

	users, err := twitch.GetUsers(channels)
	if err != nil {
		panic("Error opening file")
	}
	setupVars(users)
	scrapeStreams(twitch)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
