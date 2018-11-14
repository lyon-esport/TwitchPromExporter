package main

import (
	"TwitchLanStats/client"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	twitch := client.NewClient(os.Getenv("CLIENT_KEY"))

	fd, err := os.Open("channels.json")
	if err != nil {
		panic("Error opening file")
	}
	content, err := ioutil.ReadAll(fd)
	var channels []string

	json.Unmarshal(content, &channels)

	streamInfos, err := twitch.GetStreams(channels)
	if err != nil {
		fmt.Printf("Error getting twitch user: %v", err)
	}

	for _, streamInfo := range streamInfos {
		fmt.Printf("Stream: %s - %s, %d viewers\n", streamInfo.UserName, streamInfo.Title, streamInfo.ViewerCount)
	}

}
