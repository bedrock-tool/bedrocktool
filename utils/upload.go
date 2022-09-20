package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/sirupsen/logrus"
)

type apiClient struct {
	APIServer string
	APIKey    string

	client *http.Client

	queue   apiQueue
	Metrics *Metrics
}

var APIClient *apiClient

func InitAPIClient(APIServer, APIKey string) error {
	APIClient = &apiClient{
		APIServer: APIServer,
		APIKey:    APIKey,
		client:    &http.Client{},
		queue: apiQueue{
			connect_lock: &sync.Mutex{},
		},
		Metrics: NewMetrics(),
	}
	return nil
}

func (u *apiClient) doRequest(req *http.Request) (resp *http.Response, err error) {
	req.Header.Set("Authorization", u.APIKey)
	return u.client.Do(req)
}

// Start starts the api client
func (u *apiClient) Start() error {
	var response struct {
		AMQPUrl           string
		PrometheusPushURL string
		PrometheusAuth    string
	}

	req, _ := http.NewRequest("GET", APIClient.APIServer+"/routes", nil)
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
	auth := strings.Split(response.PrometheusAuth, ":")
	pusher := push.New(response.PrometheusPushURL, metricNamespace).
		BasicAuth(auth[0], auth[1]).
		Grouping("node_id", getNodeId())

	u.Metrics.Attach(pusher)

	return nil
}

var c = 0

func (u *apiClient) UploadSkin(skin *Skin, username, xuid string, serverAddress string) error {
	c += 1
	logrus.Infof("Uploading Skin %s %s %d", serverAddress, username, c)

	body, _ := json.Marshal(struct {
		Username      string
		Xuid          string
		Skin          *jsonSkinData
		ServerAddress string
	}{username, xuid, skin.Json(), serverAddress})

	err := u.queue.Publish(context.Background(), []byte(body))
	if err != nil {
		logrus.Warn(err)
	}
	return nil
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getNodeId() string {
	if _, err := os.Stat("node_id.txt"); err == nil {
		d, _ := os.ReadFile("node_id.txt")
		return strings.Split(string(d), "\n")[0]
	}

	ret := randomHex(10)
	os.WriteFile("node_id.txt", []byte(ret), 0o777)
	return ret
}
