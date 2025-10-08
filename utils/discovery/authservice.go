package discovery

import (
	"context"
	"encoding/base64"
	"fmt"
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

type multiplayerSessionStartResponse struct {
	Result struct {
		IssuedAt    time.Time `json:"issuedAt"`
		SignedToken string    `json:"signedToken"`
		ValidUntil  time.Time `json:"validUntil"`
	} `json:"result"`
}

func (a *AuthService) MultiplayerSessionStart(ctx context.Context, publicKey []byte, mcToken *MCToken) (signedToken string, validUntil time.Time, err error) {
	resp, err := doRequest[multiplayerSessionStartResponse](ctx, http.DefaultClient, "POST",
		fmt.Sprintf("%s/api/v1.0/multiplayer/session/start", a.ServiceURI),
		map[string]any{
			"publicKey": base64.RawStdEncoding.EncodeToString(publicKey),
		},
		func(req *http.Request) {
			req.Header.Set("Authorization", mcToken.AuthorizationHeader)
		},
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return resp.Result.SignedToken, resp.Result.ValidUntil, nil
}
