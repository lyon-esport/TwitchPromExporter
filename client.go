package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// StreamData represents the data a single stream
type StreamData struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	GameName     string    `json:"game_name"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	ViewerCount  int       `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	Language     string    `json:"language"`
	ThumbnailURL string    `json:"thumbnail_url"`
	TagIDs       []string  `json:"tag_ids"`
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
	CreatedAt       string `json:"created_at"`
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

// Token represents a token used by client to interact with the twitch API
type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	RenewDate   time.Time
	TokenType   string `json:"token_type"`
}

// Client represents a client to interact with the twitch API
type Client struct {
	ClientID     string
	ClientSecret string
	Token        Token
	httpClient   *http.Client
}

// NewClient will initialize a new client for the twitch api
func NewClient(clientID, clientSecret string) (*Client, error) {
	c := &Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		httpClient:   &http.Client{},
	}

	err := c.GetToken()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c Client) doRequest(method, uri string, header http.Header, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}

	r.Header = header

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

// GetToken will get a Twitch API Bearer Token
func (c *Client) GetToken() error {
	var uri *url.URL

	// set url
	uri, err := url.Parse(baseAuthURL + getToken)
	if err != nil {
		return err
	}

	query := uri.Query()
	query.Add("client_id", c.ClientID)
	query.Add("client_secret", c.ClientSecret)
	query.Add("grant_type", "client_credentials")
	query.Add("scope", "")
	uri.RawQuery = query.Encode()

	log.Debug("URI: ", uri.String())

	res, err := c.doRequest(http.MethodPost, uri.String(), http.Header{}, nil)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	t := Token{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	json.Unmarshal(body, &t)

	var expireDate = time.Now().Add(time.Second * time.Duration(t.ExpiresIn))
	var diffDate = expireDate.Sub(time.Now())
	t.RenewDate = time.Now().Add(diffDate / 2)

	c.Token = t

	return nil
}

// GetStreams is a wrapper to handle more than 100 Streams
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetStreams(streamsList []string) ([]StreamData, int, error) {
	var s []StreamData

	for i := 0; i < int(math.Ceil(float64(len(streamsList))/100)); i++ {
		end := i*100+100
		if len(streamsList) < end {
			end = len(streamsList)
		}

		r, token, err := c.get100Streams(streamsList[i*100:end])
		if err != nil {
			return s, token, nil
		}
		s = append(s, r...)
	}

	return s, 0, nil
}

// get100Streams will get a list of live Streams, limited to 100 users
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) get100Streams(streamsList []string) ([]StreamData, int, error) {
	if len(streamsList) >= 100 {
		return nil, 0, errors.New("can't get more than 100 users")
	}

	var uri *url.URL
	var header = http.Header{}

	// set url
	uri, err := url.Parse(baseURL + getStreamsEndpoint)
	if err != nil {
		return nil, 0, err
	}

	// set header
	header.Add("Client-ID", c.ClientID)
	header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	query := uri.Query()
	for _, stream := range streamsList {
		query.Add("user_login", stream)
	}
	uri.RawQuery = query.Encode()
	log.Debug("Streams query URL: ", uri.String())
	res, err := c.doRequest(http.MethodGet, uri.String(), header, nil)
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
	log.Debugf("Stream remaining tokens: %d", token)

	json.Unmarshal(body, &s)
	return s.Data, token, nil
}

// GetUsers is a wrapper to handle more than 100 Users
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetUsers(usersList []string) ([]UserData, error) {
	var s []UserData

	for i := 0; i < int(math.Ceil(float64(len(usersList))/100)); i++ {
		end := i*100+100
		if len(usersList) < end {
			end = len(usersList)
		}

		r, err := c.get100Users(usersList[i*100:end])
		if err != nil {
			return s, nil
		}

		s = append(s, r...)
	}

	return s, nil
}

// get100Users will get a list of users information, limited to 100 users
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) get100Users(usersList []string) ([]UserData, error) {
	var uri *url.URL
	var header = http.Header{}

	// set url
	uri, err := url.Parse(baseURL + getUsersEndpoint)
	if err != nil {
		return nil, err
	}

	// set header
	header.Add("Client-ID", c.ClientID)
	header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	query := uri.Query()
	for _, user := range usersList {
		query.Add("login", user)
	}
	uri.RawQuery = query.Encode()
	log.Debug("URI: ", uri.String())
	res, err := c.doRequest(http.MethodGet, uri.String(), header, nil)
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

// GetFollows will get the number of followers for a user
// The url query parameter are defined by the GetStreamsInput struct
func (c Client) GetFollows(userID string) (int, error) {
	var uri *url.URL
	var header = http.Header{}

	// set url
	uri, err := url.Parse(baseURL + getUserFollow)
	if err != nil {
		return 0, err
	}

	// set header
	header.Add("Client-ID", c.ClientID)
	header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	query := uri.Query()
	query.Add("to_id", userID)
	uri.RawQuery = query.Encode()
	log.Debug("URI: ", uri.String())
	res, err := c.doRequest(http.MethodGet, uri.String(), header, nil)
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
	return s.Total, nil
}
