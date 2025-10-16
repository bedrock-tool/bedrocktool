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

func (g *Gathering) Address(ctx context.Context, mcToken *MCToken) (string, error) {
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
		nil, mcTokenAuth(mcToken),
	)
	if err != nil {
		return "", err
	}
	_ = resp1

	resp, err := doRequest[venueResponse](ctx, http.DefaultClient, "GET",
		fmt.Sprintf("%s/api/v1.0/venue/%s", g.client.ServiceURI, g.GatheringID),
		nil, mcTokenAuth(mcToken),
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
}

func (g *GatheringsService) GetGatherings(ctx context.Context, mcToken *MCToken) ([]*Gathering, error) {
	type gatheringsResponse struct {
		Result []Gathering `json:"result"`
	}

	resp, err := doRequest[gatheringsResponse](ctx, http.DefaultClient, "GET",
		fmt.Sprintf("%s/api/v1.0/config/public?lang=en-GB&clientVersion=%s&clientPlatform=Windows10&clientSubPlatform=Windows10", g.ServiceURI, protocol.CurrentVersion),
		nil, mcTokenAuth(mcToken),
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

func (g *GatheringsService) JoinExperience(ctx context.Context, mcToken *MCToken, id uuid.UUID) (string, error) {
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
		mcTokenAuth(mcToken),
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", resp.Result.IPV4Address, resp.Result.Port), nil
}

type FeaturedServer struct {
	Name         string
	Address      string
	ExperienceId string
}

func (g *GatheringsService) GetFeaturedServers(ctx context.Context, mcToken *MCToken) ([]FeaturedServer, error) {
	type Translated struct {
		Neutral string `json:"NEUTRAL"`
	}
	type Images struct {
		Tag  string `json:"Tag"`
		ID   string `json:"Id"`
		Type string `json:"Type"`
		URL  string `json:"Url"`
	}
	type AvailableGames struct {
		Description string `json:"description"`
		ImageTag    string `json:"imageTag"`
		Subtitle    string `json:"subtitle"`
		Title       string `json:"title"`
	}
	type DisplayProperties struct {
		AvailableGames    []AvailableGames `json:"availableGames"`
		CreatorName       string           `json:"creatorName"`
		MaxClientVersion  string           `json:"maxClientVersion"`
		MinClientVersion  string           `json:"minClientVersion"`
		News              string           `json:"news"`
		NewsTitle         string           `json:"newsTitle"`
		OriginalCreatorID string           `json:"originalCreatorId"`
		Port              int              `json:"port"`
		RequireXBL        string           `json:"requireXBL"`
		StorePageID       string           `json:"storePageId"`
		URL               string           `json:"url"`
		WhitelistURL      string           `json:"whitelistUrl"`
		AllowListURL      string           `json:"allowListUrl"`
		ExperienceID      string           `json:"experienceId"`
		IsTop             bool             `json:"isTop"`
	}
	type EntityKey struct {
		ID         string `json:"Id"`
		Type       string `json:"Type"`
		TypeString string `json:"TypeString"`
	}
	type Items struct {
		ID                string            `json:"Id"`
		Type              string            `json:"Type"`
		AlternateIds      []any             `json:"AlternateIds"`
		Title             Translated        `json:"Title"`
		Description       Translated        `json:"Description,omitempty"`
		ContentType       string            `json:"ContentType"`
		Platforms         []string          `json:"Platforms"`
		Tags              []string          `json:"Tags"`
		CreationDate      time.Time         `json:"CreationDate"`
		LastModifiedDate  time.Time         `json:"LastModifiedDate"`
		StartDate         time.Time         `json:"StartDate"`
		Contents          []any             `json:"Contents"`
		Images            []Images          `json:"Images"`
		ItemReferences    []any             `json:"ItemReferences"`
		DisplayProperties DisplayProperties `json:"DisplayProperties,omitempty"`
		IsStackable       bool              `json:"IsStackable"`
		CreatorEntityKey  EntityKey         `json:"CreatorEntityKey"`
		IsHydrated        bool              `json:"IsHydrated"`
		Keywords          Translated        `json:"Keywords"`
		CreatorEntity     EntityKey         `json:"CreatorEntity,omitempty"`
	}
	type Data struct {
		Count             int     `json:"Count"`
		Items             []Items `json:"Items"`
		ConfigurationName string  `json:"ConfigurationName"`
	}
	type Response struct {
		Status string `json:"status"`
		Code   int    `json:"code"`
		Data   Data   `json:"data"`
	}

	resp, err := doRequest[Response](ctx, http.DefaultClient, "POST",
		fmt.Sprintf("%s/api/v2.0/discovery/blob/client", g.ServiceURI), nil, mcTokenAuth(mcToken))
	if err != nil {
		return nil, err
	}

	var out []FeaturedServer
	for _, item := range resp.Data.Items {
		address := ""
		if item.DisplayProperties.URL != "" {
			address = fmt.Sprintf("%s:%d", item.DisplayProperties.URL, item.DisplayProperties.Port)
		}
		out = append(out, FeaturedServer{
			Name:         item.Title.Neutral,
			Address:      address,
			ExperienceId: item.DisplayProperties.ExperienceID,
		})
	}
	return out, nil
}
