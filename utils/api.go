package utils

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Metrics interface {
	// Start starts the metric pusher
	Start(url, user, password string) error
	// deletes from pusher
	Delete()
}

type apiClient struct {
	server string
	key    string

	client *http.Client

	queue   apiQueue
	Metrics Metrics
}

var APIClient *apiClient

// InitAPIClient creates the api client
func InitAPIClient(APIServer, APIKey string, metrics Metrics) error {
	APIClient = &apiClient{
		server: APIServer,
		key:    APIKey,
		client: &http.Client{},
		queue: apiQueue{
			connect_lock: &sync.Mutex{},
		},
		Metrics: metrics,
	}
	return nil
}

func (u *apiClient) doRequest(req *http.Request) (resp *http.Response, err error) {
	req.Header.Set("Authorization", u.key)
	return u.client.Do(req)
}

// Start starts the api client
func (u *apiClient) Start() error {
	var response struct {
		AMQPUrl           string
		PrometheusPushURL string
		PrometheusAuth    string
	}

	req, _ := http.NewRequest("GET", APIClient.server+"/routes", nil)
	resp, err := APIClient.doRequest(req)
	if err != nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// rabbitmq
	u.queue.uri = response.AMQPUrl
	u.queue.Reconnect()

	// prometheus
	if u.Metrics != nil {
		auth := strings.Split(response.PrometheusAuth, ":")
		if err := u.Metrics.Start(response.PrometheusPushURL, auth[0], auth[1]); err != nil {
			return err
		}
	}

	return nil
}

var c = 0

// UploadSkin pushes a skin to the message server
func (u *apiClient) UploadSkin(skin *Skin, username, xuid string, serverAddress string) {
	c += 1
	logrus.Infof("Uploading Skin %s %s %d", serverAddress, username, c)

	body, _ := json.Marshal(struct {
		Username      string
		Xuid          string
		Skin          *jsonSkinData
		ServerAddress string
		Time          int64
	}{username, xuid, skin.Json(), serverAddress, time.Now().Unix()})

	buf := bytes.NewBuffer(nil)
	w := gzip.NewWriter(buf)
	w.Write(body)
	w.Close()

	err := u.queue.Publish(context.Background(), buf.Bytes(), "application/json-gz")
	if err != nil {
		logrus.Warn(err)
	}
}
