package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	baseURL            = "https://api.twitch.tv/helix"
	getStreamsEndpoint = "/streams"
)

func main() {
	client := Client{
		ClientID:   os.Getenv("CLIENT_KEY"),
		httpClient: &http.Client{},
	}

	fd, err := os.Open("channels.json")
	if err != nil {
		panic("Error opening file")
	}
	content, err := ioutil.ReadAll(fd)
	var channels []string

	json.Unmarshal(content, &channels)

	streamInfos, err := client.GetStreams(channels)
	if err != nil {
		fmt.Printf("Error getting twitch user: %v", err)
	}

	for _, streamInfo := range streamInfos {
		fmt.Printf("Stream: %s - %s, %d viewers\n", streamInfo.UserName, streamInfo.Title, streamInfo.ViewerCount)
	}

}

// StreamData represents the data a single stream
type StreamData struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	CommunityIds []string  `json:"community_ids"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	ViewerCount  int       `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	Language     string    `json:"language"`
	ThumbnailURL string    `json:"thumbnail_url"`
}

type streams struct {
	Data []StreamData `json:"data"`
}

// Client represents a client to interact with the twitch API
type Client struct {
	ClientID   string
	httpClient *http.Client
}

func (c Client) doRequest(method, uri string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Client-ID", c.ClientID)

	res, err := c.httpClient.Do(r)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 401 {
		return nil, errors.New("server returned 401. likely caused due to an invalid client id")
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("server returned non 200 status code. status code: %v", res.StatusCode)
	}

	return res, nil
}

// GetStreams will get a list of live streams
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetStreams(streamsList []string) ([]StreamData, error) {
	// since first, when uninitialized is 0, we have to set it to the default value
	/*if i.First == 0 {
		i.First = 20
	}*/

	var uri *url.URL

	uri, err := url.Parse(baseURL + getStreamsEndpoint)
	if err != nil {
		return nil, err
	}

	query := uri.Query()
	for _, stream := range streamsList {
		query.Add("user_login", stream)
	}
	uri.RawQuery = query.Encode()
	res, err := c.doRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	s := streams{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(body, &s)
	return s.Data, nil
}
