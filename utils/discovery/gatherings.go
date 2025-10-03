package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

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
	client *GatheringsService

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

	resp1, err := doRequest[map[string]any](ctx, http.DefaultClient, "GET",
		fmt.Sprintf("%s/api/v1.0/access?lang=en-US&clientVersion=%s&clientPlatform=Windows10&clientSubPlatform=Windows10", g.client.ServiceURI, protocol.CurrentVersion),
		nil, g.client.mcTokenAuth,
	)
	if err != nil {
		return "", err
	}
	_ = resp1

	resp, err := doRequest[venueResponse](ctx, http.DefaultClient, "GET",
		fmt.Sprintf("%s/api/v1.0/venue/%s", g.client.ServiceURI, g.GatheringID),
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

type GatheringsService struct {
	Service
	token *MCToken
}

func (g *GatheringsService) SetToken(token *MCToken) {
	g.token = token
}

func (g *GatheringsService) Gatherings(ctx context.Context) ([]*Gathering, error) {
	type gatheringsResponse struct {
		Result []Gathering `json:"result"`
	}

	resp, err := doRequest[gatheringsResponse](ctx, http.DefaultClient, "GET",
		fmt.Sprintf("%s/api/v1.0/config/public?lang=en-GB&clientVersion=%s&clientPlatform=Windows10&clientSubPlatform=Windows10", g.ServiceURI, protocol.CurrentVersion),
		nil, g.mcTokenAuth,
	)
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

func (g *GatheringsService) JoinExperience(ctx context.Context, id uuid.UUID) (string, error) {
	type joinExperienceResponse struct {
		Result struct {
			NetworkProtocol string `json:"networkProtocol"`
			IPV4Address     string `json:"ipV4Address"`
			Port            int    `json:"port"`
		} `json:"result"`
	}
	resp, err := doRequest[joinExperienceResponse](ctx, http.DefaultClient, "POST",
		fmt.Sprintf("%s/api/v2.0/join/experience", g.ServiceURI),
		map[string]any{
			"experienceId": id,
		},
		g.mcTokenAuth,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", resp.Result.IPV4Address, resp.Result.Port), nil
}

func (g *GatheringsService) mcTokenAuth(req *http.Request) {
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
