package gatherings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/bedrock-tool/bedrocktool/utils/playfab"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

const minecraftUserAgent = "libhttpclient/1.0.0.0"

type Segment struct {
	SegmentType  string    `json:"segmentType"`
	StartTimeUtc time.Time `json:"startTimeUtc"`
	EndTimeUtc   time.Time `json:"endTimeUtc"`
	UI           struct {
		CaptionText              string `json:"captionText"`
		CaptionForegroundColor   string `json:"captionForegroundColor"`
		CaptionBackgroundColor   string `json:"captionBackgroundColor"`
		StartScreenButtonText    string `json:"startScreenButtonText"`
		BadgeImage               string `json:"badgeImage"`
		CaptionIncludesCountdown bool   `json:"captionIncludesCountdown"`
		ActionButtonText         string `json:"actionButtonText"`
		InfoButtonText           string `json:"infoButtonText"`
		HeaderText               string `json:"headerText"`
		TitleText                string `json:"titleText"`
		BodyText                 string `json:"bodyText"`
		EventImage               string `json:"eventImage"`
		BodyImage                string `json:"bodyImage"`
		ActionButtonURL          string `json:"actionButtonUrl"`
		InfoButtonURL            string `json:"infoButtonUrl"`
	} `json:"ui"`
}

type Gathering struct {
	client *GatheringsClient

	GatheringID   string         `json:"gatheringId"`
	StartTimeUtc  time.Time      `json:"startTimeUtc"`
	EndTimeUtc    time.Time      `json:"endTimeUtc"`
	Segments      []Segment      `json:"segments"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	IsEnabled     bool           `json:"isEnabled"`
	IsPrivate     bool           `json:"isPrivate"`
	GatheringType string         `json:"gatheringType"`
	AdditionalLoc map[string]any `json:"additionalLoc"`
}

func (g *Gathering) Address(ctx context.Context) (string, error) {
	type venueResponse struct {
		Result struct {
			Venue struct {
				ServerIpAddress string `json:"serverIpAddress"`
				ServerPort      int    `json:"serverPort"`
			} `json:"venue"`
		} `json:"result"`
	}

	gatheringsService, err := g.client.discovery.GatheringsService()
	if err != nil {
		return "", err
	}

	resp1, err := doRequest[map[string]any](ctx, g.client.http, "GET",
		fmt.Sprintf("%s/api/v1.0/access?lang=en-US&clientVersion=%s&clientPlatform=Windows10&clientSubPlatform=Windows10", gatheringsService.ServiceURI, protocol.CurrentVersion),
		nil, g.client.mcTokenAuth,
	)
	if err != nil {
		return "", err
	}
	_ = resp1

	resp, err := doRequest[venueResponse](ctx, g.client.http, "GET",
		fmt.Sprintf("%s/api/v1.0/venue/%s", gatheringsService.ServiceURI, g.GatheringID),
		nil, g.client.mcTokenAuth,
	)
	if err != nil {
		return "", err
	}

	if resp.Result.Venue.ServerIpAddress == "" {
		return "", errors.New("didnt get a server address")
	}

	return fmt.Sprintf("%s:%d", resp.Result.Venue.ServerIpAddress, resp.Result.Venue.ServerPort), nil
}

type GatheringsClient struct {
	http      *http.Client
	token     *playfab.MCToken
	discovery *discovery.Discovery
}

func NewGatheringsClient(token *playfab.MCToken, discovery *discovery.Discovery) *GatheringsClient {
	client := &http.Client{}
	return &GatheringsClient{
		http:      client,
		token:     token,
		discovery: discovery,
	}
}

func (g *GatheringsClient) Gatherings(ctx context.Context) ([]*Gathering, error) {
	type gatheringsResponse struct {
		Result []Gathering `json:"result"`
	}

	gatheringService, err := g.discovery.GatheringsService()
	if err != nil {
		return nil, err
	}

	resp, err := doRequest[gatheringsResponse](ctx, g.http, "GET", fmt.Sprintf("%s/api/v1.0/config/public?lang=en-GB&clientVersion=%s&clientPlatform=Windows10&clientSubPlatform=Windows10", gatheringService.ServiceURI, protocol.CurrentVersion), nil, g.mcTokenAuth)
	if err != nil {
		return nil, err
	}

	var gatherings []*Gathering
	for _, gathering := range resp.Result {
		gathering.client = g
		gatherings = append(gatherings, &gathering)
	}
	return gatherings, nil
}

func (g *GatheringsClient) mcTokenAuth(req *http.Request) {
	req.Header.Set("Authorization", g.token.AuthorizationHeader)
}

type JsonResponseError struct {
	Status string
	Data   map[string]any
}

func (e JsonResponseError) Error() string {
	message, ok := e.Data["message"].(string)
	if ok {
		return e.Status + "\n" + message
	}
	return e.Status
}

func doRequest[T any](ctx context.Context, client *http.Client, method, url string, payload any, header func(*http.Request)) (*T, error) {
	logrus.Tracef("doRequest: %s", url)
	var err error
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", minecraftUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Cache-Control", "no-cache")
	if header != nil {
		header(req)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bodyResp, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		var resp map[string]any
		err = json.Unmarshal(bodyResp, &resp)
		if err != nil {
			return nil, err
		}
		return nil, &JsonResponseError{
			Status: res.Status,
			Data:   resp,
		}
	}

	var resp T
	err = json.Unmarshal(bodyResp, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
