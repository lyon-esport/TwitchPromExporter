package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

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

// UserData struct represents a user as defined by the twitch api
type UserData struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	Type            string `json:"type"`
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	ProfileImageURL string `json:"profile_image_url"`
	OfflineImageURL string `json:"offline_image_url"`
	ViewCount       int    `json:"view_count"`
	Email           string `json:"email"`
}

type UserFollow struct {
	Total int `json:"total"`
}

type Streams struct {
	Data []StreamData `json:"data"`
}

type Users struct {
	Data []UserData `json:"data"`
}

// Client represents a client to interact with the twitch API
type Client struct {
	ClientID   string
	httpClient *http.Client
}

// NewClient will initialize a new client for the twitch api
func NewClient(cid string) *Client {
	return &Client{
		ClientID:   cid,
		httpClient: &http.Client{},
	}
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

// GetStreams will get a list of live Streams
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetStreams(streamsList []string) ([]StreamData, int, error) {
	// since first, when uninitialized is 0, we have to set it to the default value
	/*if i.First == 0 {
		i.First = 20
	}*/

	var uri *url.URL

	uri, err := url.Parse(baseURL + getStreamsEndpoint)
	if err != nil {
		return nil, 0, err
	}

	query := uri.Query()
	for _, stream := range streamsList {
		query.Add("user_login", stream)
	}
	uri.RawQuery = query.Encode()
	res, err := c.doRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, 0, err
	}

	defer res.Body.Close()

	s := Streams{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	var token int
	strToken, ok := res.Header["Ratelimit-Remaining"]
	if ok {
		if len(strToken) > 0 {
			token, err = strconv.Atoi(strToken[0])
			if err != nil {
				token = 0
			}
		}
	} else {
		token = 0
	}
	log.Debugf("Remaining tokens: %d", token)

	json.Unmarshal(body, &s)
	return s.Data, token, nil
}

// GetStreams will get a list of live Streams
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetUsers(usersList []string) ([]UserData, error) {
	var uri *url.URL

	uri, err := url.Parse(baseURL + getUsersEndpoint)
	if err != nil {
		return nil, err
	}

	query := uri.Query()
	for _, user := range usersList {
		query.Add("login", user)
	}
	uri.RawQuery = query.Encode()
	log.Debug("URI: ", uri.String())
	res, err := c.doRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	s := Users{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(body, &s)
	return s.Data, nil
}

// GetStreams will get a list of live Streams
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetFollows(userID string) (int, error) {
	var uri *url.URL

	uri, err := url.Parse(baseURL + getUserFollow)
	if err != nil {
		return 0, err
	}

	query := uri.Query()
	query.Add("to_id", userID)
	uri.RawQuery = query.Encode()
	log.Debug("URI: ", uri.String())
	res, err := c.doRequest("GET", uri.String(), nil)
	if err != nil {
		return 0, err
	}

	defer res.Body.Close()

	s := UserFollow{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}

	json.Unmarshal(body, &s)
	// fmt.Println(s.Total)
	return s.Total, nil
}
