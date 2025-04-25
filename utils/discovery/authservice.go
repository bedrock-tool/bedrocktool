package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type AuthService struct {
	ServiceWithEdu
	Issuer string `json:"issuer"`
}

type mcTokenDevice struct {
	ApplicationType    string   `json:"applicationType"`
	Capabilities       []string `json:"capabilities"`
	GameVersion        string   `json:"gameVersion"`
	ID                 string   `json:"id"`
	Memory             string   `json:"memory"`
	Platform           string   `json:"platform"`
	PlayFabTitleID     string   `json:"playFabTitleId"`
	StorePlatform      string   `json:"storePlatform"`
	TreatmentOverrides any      `json:"treatmentOverrides"`
	Type               string   `json:"type"`
}

type mcTokenUser struct {
	Language     string `json:"language"`
	LanguageCode string `json:"languageCode"`
	RegionCode   string `json:"regionCode"`
	Token        string `json:"token"`
	TokenType    string `json:"tokenType"`
}
type mcTokenRequest struct {
	Device mcTokenDevice `json:"device"`
	User   mcTokenUser   `json:"user"`
}

type MCToken struct {
	AuthorizationHeader string    `json:"authorizationHeader"`
	ValidUntil          time.Time `json:"validUntil"`
	Treatments          []string  `json:"treatments"`
	Configurations      struct {
		Minecraft struct {
			ID         string         `json:"id"`
			Parameters map[string]any `json:"parameters"`
		} `json:"minecraft"`
	} `json:"configurations"`
}

type mcTokenResponse struct {
	Result MCToken `json:"result"`
}

func (a *AuthService) StartSession(ctx context.Context, xblToken, titleid string) (*MCToken, error) {
	resp, err := doRequest[mcTokenResponse](ctx, http.DefaultClient, "POST", fmt.Sprintf("%s/api/v1.0/session/start", a.ServiceURI), mcTokenRequest{
		Device: mcTokenDevice{
			ApplicationType:    "MinecraftPE",
			Capabilities:       []string{"RayTracing"},
			GameVersion:        protocol.CurrentVersion,
			ID:                 uuid.New().String(),
			Memory:             fmt.Sprintf("%d", int64(16*(1024*1024*1024))), // 16 GB
			Platform:           "Windows10",
			PlayFabTitleID:     strings.ToUpper(a.PlayfabTitleID),
			StorePlatform:      "uwp.store",
			TreatmentOverrides: nil,
			Type:               "Windows10",
		},
		User: mcTokenUser{
			Language:     "en",
			LanguageCode: "en-US",
			RegionCode:   "US",
			Token:        xblToken,
			TokenType:    "Xbox",
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func doRequest[T any](ctx context.Context, client *http.Client, method, url string, payload any, extraHeaders func(*http.Request)) (*T, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
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
	if extraHeaders != nil {
		extraHeaders(req)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		bodyResp, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
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
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
