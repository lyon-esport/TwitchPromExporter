package main

import (
	"encoding/json"
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

type Stream struct {
	Name      string `json:"name"`
	Online    bool   `json:"online"`
	Uptime    int64  `json:"uptime"`
	Followers int    `json:"followers"`
	Viewers   int    `json:"viewers"`
	Views     int    `json:"views"`
}

var (
	version                                         = "development"
	channels                                        []string
	channelsData                                    = make(map[string]*Stream)
	channelsID                                      = make(map[string]string)
	streamsUp                                       = map[string]prometheus.Gauge{}
	streamsViewers, streamsUptime, views, followers *prometheus.GaugeVec
	lastScrape, tokenRemaining                      prometheus.Gauge
)

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

	streamsViewers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "twitch",
			Name:      "viewers",
			Help:      "Count the number of viewers",
		},
		[]string{"name"},
	)

	streamsUptime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "twitch",
			Name:      "started_at",
			Help:      "Started at",
		},
		[]string{"name"},
	)

	// use dynamic label
	views = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "twitch",
			Name:      "views",
			Help:      "Number of views",
		},
		[]string{"name"},
	)

	followers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "twitch",
			Name:      "followers",
			Help:      "Number of followers",
		},
		[]string{"name"},
	)

	for _, user := range users {
		channelsData[user.ID] = &Stream{
			Name:    user.DisplayName,
			Online:  false,
			Viewers: 0,
			Views:   user.ViewCount,
		}
		log.Debug(user)
		channelsID[user.ID] = user.DisplayName
		streamsUp[user.ID] = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "twitch",
			Name:      "online",
			Help:      "Is the streamer is online",
			ConstLabels: prometheus.Labels{
				"name": user.DisplayName,
			},
		})
		views.With(prometheus.Labels{"name": user.DisplayName}).Set(float64(user.ViewCount))
	}
}

func scrapeStreams(twitch *Client) {
	go func() {
		var streamsID []string
		for id := range channelsID {
			streamsID = append(streamsID, id)
		}

		var streamScraped = 0
		for {

			// download data
			streamInfos, token, err := twitch.GetStreams(channels)
			log.Debug("Stream info", streamInfos)
			if err != nil {
				fmt.Printf("Error getting twitch user: %v", err)
			}

			//infos
			tokenRemaining.Set(float64(token))
			lastScrape.Set(float64(time.Now().Unix()))

			// generate a map to index the values
			streamTable := make(map[string]StreamData)
			for _, streamInfo := range streamInfos {
				streamTable[streamInfo.UserID] = streamInfo
			}

			// sync stream info with twitch state
			for channelId, _ := range channelsData {
				streamData, ok := streamTable[channelId]
				if ok {
					channelsData[channelId].Online = true
					channelsData[channelId].Viewers = streamData.ViewerCount
					channelsData[channelId].Uptime = streamData.StartedAt.Unix()

					// setting prometheus var
					streamsUp[streamData.UserID].Set(1)
					streamsViewers.With(prometheus.Labels{"name": channelsID[streamData.UserID]}).Set(float64(streamData.ViewerCount))
					streamsUptime.With(prometheus.Labels{"name": channelsID[streamData.UserID]}).Set(float64(streamData.StartedAt.Unix()))

				} else {

					// updating json
					channelsData[channelId].Online = false
					channelsData[channelId].Uptime = 0
					channelsData[channelId].Viewers = 0

					//setting prometheus var
					streamsUp[channelId].Set(0)
					streamsViewers.Delete(prometheus.Labels{"name": channelsID[channelId]})
					streamsUptime.Delete(prometheus.Labels{"name": channelsID[channelId]})
				}
			}

			// Update the view count
			token--
			users, err := twitch.GetUsers(channels)
			if err != nil {
				log.Fatal(err)
				panic(err)
			}
			// Fill the information
			for _, user := range users {
				views.With(prometheus.Labels{"name": user.DisplayName}).Set(float64(user.ViewCount))
				channelsData[user.ID].Views = user.ViewCount
			}

			// Update the follow count with the remaining tokens
			// Prevent exhausting all the tokens
			for token > 5 {
				token--
				var sid = streamsID[streamScraped]

				followersCount, err := twitch.GetFollows(sid)
				if err != nil {
					fmt.Printf("Error getting twitch user: %v", err)
					break
				}
				followers.With(prometheus.Labels{"name": channelsID[sid]}).Set(float64(followersCount))
				channelsData[sid].Followers = followersCount
				streamScraped++
				if streamScraped >= len(streamsID) {
					streamScraped = 0
					break
				}
			}
			log.Debug("Scraped ", streamScraped, "/", len(streamsID), " stream")

			time.Sleep(30 * time.Second)
		}
	}()
}

func jsonStats(w http.ResponseWriter, r *http.Request) {
	log.Debug(len(channelsData), " Streams to serialize")
	streamList := make([]Stream, len(channelsData))
	var pos = 0
	for _, stream := range channelsData {
		streamList[pos] = *stream
		pos += 1
	}
	json.NewEncoder(w).Encode(streamList)
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

	log.Printf("Starting %s %s", os.Args[0], version)

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
	http.HandleFunc("/", jsonStats)
	http.ListenAndServe(":2112", nil)
}
