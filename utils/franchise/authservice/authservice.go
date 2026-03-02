package authservice

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/franchise/discovery"
	"github.com/bedrock-tool/bedrocktool/utils/franchise/internal"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type AuthService struct {
	Config discovery.AuthService
}

func NewAuthService(discovery *discovery.Discovery) (*AuthService, error) {
	a := &AuthService{}
	err := discovery.Environment(&a.Config, "auth")
	if err != nil {
		return nil, err
	}
	return a, nil
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

func (token *MCToken) AddHeader(req *http.Request) {
	req.Header.Set("Authorization", token.AuthorizationHeader)
}

func (a *AuthService) StartSession(ctx context.Context, token, titleid string) (*MCToken, error) {
	type mcTokenRequest struct {
		Device mcTokenDevice `json:"device"`
		User   mcTokenUser   `json:"user"`
	}
	resp, err := internal.DoRequest[internal.Result[MCToken]](
		ctx, http.DefaultClient, "POST",
		a.Config.Url("/api/v1.0/session/start"),
		mcTokenRequest{
			Device: mcTokenDevice{
				ApplicationType:    "MinecraftPE",
				Capabilities:       []string{"RayTracing"},
				GameVersion:        protocol.CurrentVersion,
				ID:                 uuid.New().String(),
				Memory:             fmt.Sprintf("%d", int64(16*(1024*1024*1024))), // 16 GB
				Platform:           "Windows10",
				PlayFabTitleID:     strings.ToUpper(a.Config.PlayfabTitleID),
				StorePlatform:      "uwp.store",
				TreatmentOverrides: nil,
				Type:               "Windows10",
			},
			User: mcTokenUser{
				Language:     "en",
				LanguageCode: "en-US",
				RegionCode:   "US",
				Token:        token,
				TokenType:    "Xbox",
			},
		}, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (a *AuthService) MultiplayerSessionStart(ctx context.Context, publicKey []byte, mcToken *MCToken) (signedToken string, validUntil time.Time, err error) {
	type multiplayerSessionStartResponse struct {
		IssuedAt    time.Time `json:"issuedAt"`
		SignedToken string    `json:"signedToken"`
		ValidUntil  time.Time `json:"validUntil"`
	}
	resp, err := internal.DoRequest[internal.Result[multiplayerSessionStartResponse]](
		ctx, http.DefaultClient, "POST",
		a.Config.Url("/api/v1.0/multiplayer/session/start"),
		map[string]any{"publicKey": base64.RawStdEncoding.EncodeToString(publicKey)},
		mcToken.AddHeader,
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return resp.Data.SignedToken, resp.Data.ValidUntil, nil
}
