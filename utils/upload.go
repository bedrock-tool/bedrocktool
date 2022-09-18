package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Uploader struct {
	APIServer string
	APIKey    string

	client *http.Client
}

var APIClient *Uploader = &Uploader{
	client: &http.Client{},
}

func (u *Uploader) doRequest(req *http.Request) (resp *http.Response, err error) {
	req.Header.Set("Authorization", u.APIKey)
	return u.client.Do(req)
}

func (u *Uploader) Check() error {
	req, _ := http.NewRequest("GET", u.APIServer+"/check", nil)
	resp, err := u.doRequest(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid APIKey in config; Status: %s", resp.Status)
	}
	return nil
}

func (u *Uploader) UploadSkin(skin *Skin, username, xuid string, BuildPlatform int32, serverAddress string) error {
	logrus.Infof("Uploading Skin %s", username)

	body, _ := json.Marshal(struct {
		Username      string
		Xuid          string
		Platform      int
		Skin          *jsonSkinData
		ServerAddress string
	}{username, xuid, int(BuildPlatform), skin.Json(), serverAddress})

	req, _ := http.NewRequest("POST", u.APIServer+"/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.doRequest(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to upload Skin %s, %d", username, resp.StatusCode)
	}
	return nil
}
