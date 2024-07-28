package playfab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"golang.org/x/oauth2"
)

const minecraftUserAgent = "libhttpclient/1.0.0.0"
const minecraftDefaultSDK = "XPlatCppSdk-3.6.190304"

type Client struct {
	src       oauth2.TokenSource
	http      *http.Client
	discovery *discovery.Discovery

	accountID     string
	sessionTicket string

	mcToken     *MCToken
	playerToken *EntityToken
	masterToken *EntityToken
}

func NewClient(discovery *discovery.Discovery, src oauth2.TokenSource) *Client {
	return &Client{
		src:       src,
		http:      http.DefaultClient,
		discovery: discovery,
	}
}

func (c *Client) LoggedIn() bool {
	return c.mcToken != nil && c.mcToken.ValidUntil.Before(time.Now())
}

func (c *Client) Login(ctx context.Context) error {
	liveToken, err := c.src.Token()
	if err != nil {
		return err
	}
	err = c.loginWithXbox(ctx, liveToken)
	if err != nil {
		return err
	}

	err = c.loginMaster(ctx)
	if err != nil {
		return err
	}

	err = c.startSession(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) loginWithXbox(ctx context.Context, liveToken *oauth2.Token) error {
	xboxToken, err := auth.RequestXBLToken(ctx, liveToken, "rp://playfabapi.com/")
	if err != nil {
		return err
	}

	authService, err := c.discovery.AuthService()
	if err != nil {
		return err
	}

	resp, err := doPlayfabRequest[loginResponse](ctx, c.http, authService.PlayfabTitleID, "/Client/LoginWithXbox?sdk="+minecraftDefaultSDK, xboxLoginRequest{
		CreateAccount: true,
		InfoRequestParameters: infoRequestParameters{
			PlayerProfile:   true,
			UserAccountInfo: true,
		},
		TitleID:   strings.ToUpper(authService.PlayfabTitleID),
		XboxToken: fmt.Sprintf("XBL3.0 x=%v;%v", xboxToken.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash, xboxToken.AuthorizationToken.Token),
	}, nil)
	if err != nil {
		return err
	}

	c.accountID = resp.Data.PlayFabID
	c.sessionTicket = resp.Data.SessionTicket
	c.playerToken = &resp.Data.EntityToken
	return nil
}

func (c *Client) loginMaster(ctx context.Context) error {
	authService, err := c.discovery.AuthService()
	if err != nil {
		return err
	}

	resp, err := doPlayfabRequest[EntityToken](ctx, c.http, authService.PlayfabTitleID, "/Authentication/GetEntityToken?sdk="+minecraftDefaultSDK, entityTokenRequest{
		Entity: &Entity{
			ID:   c.accountID,
			Type: "master_player_account",
		},
	}, authToken(c.playerToken))
	if err != nil {
		return err
	}

	c.masterToken = resp.Data
	return nil
}

func (c *Client) startSession(ctx context.Context) error {
	authService, err := c.discovery.AuthService()
	if err != nil {
		return err
	}

	if c.playerToken == nil || c.playerToken.TokenExpiration.Before(time.Now()) {
		err = c.Login(ctx)
		if err != nil {
			return err
		}
	}

	resp, err := doRequest[mcTokenResponse](ctx, c.http, fmt.Sprintf("%s/api/v1.0/session/start", authService.ServiceURI), mcTokenRequest{
		Device: mcTokenDevice{
			ApplicationType:    "MinecraftPE",
			Capabilities:       []string{"RayTracing"},
			GameVersion:        protocol.CurrentVersion,
			ID:                 uuid.New().String(),
			Memory:             fmt.Sprint(16 * (1024 * 1024 * 1024)), // 16 GB
			Platform:           "Windows10",
			PlayFabTitleID:     strings.ToUpper(authService.PlayfabTitleID),
			StorePlatform:      "uwp.store",
			TreatmentOverrides: nil,
			Type:               "Windows10",
		},
		User: mcTokenUser{
			Language:     "en",
			LanguageCode: "en-US",
			RegionCode:   "US",
			Token:        c.sessionTicket,
			TokenType:    "PlayFab",
		},
	})
	if err != nil {
		return err
	}

	c.mcToken = &resp.Result
	return nil
}

func (c *Client) MCToken() (*MCToken, error) {
	if c.mcToken == nil || c.mcToken.ValidUntil.Before(time.Now()) {
		err := c.startSession(context.TODO())
		if err != nil {
			return nil, err
		}
	}
	return c.mcToken, nil
}

func doRequest[T any](ctx context.Context, client *http.Client, url string, payload any) (*T, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", minecraftUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Cache-Control", "no-cache")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var resp T
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func doPlayfabRequest[T any](ctx context.Context, client *http.Client, titleID, endpoint string, payload any, token func(*http.Request)) (*Response[T], error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://"+titleID+".playfabapi.com"+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", minecraftUserAgent)
	req.Header.Set("X-PlayFabSDK", minecraftDefaultSDK)
	req.Header.Set("X-ReportErrorAsSuccess", "true")
	if token != nil {
		token(req)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		playfabErr := PlayfabError{}
		err = json.NewDecoder(res.Body).Decode(&playfabErr)
		if err != nil {
			return nil, err
		}
		return nil, &playfabErr
	}

	var resp Response[T]
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func authToken(token *EntityToken) func(req *http.Request) {
	return func(req *http.Request) {
		req.Header.Set("X-EntityToken", token.EntityToken)
	}
}
