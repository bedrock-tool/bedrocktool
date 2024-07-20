package playfab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const minecraftTitleID = "20ca2"
const minecraftUserAgent = "libhttpclient/1.0.0.0"
const minecraftDefaultSDK = "XPlatCppSdk-3.6.190304"

type Client struct {
	http *http.Client

	accountID     string
	sessionTicket string

	mcToken     *MCToken
	playerToken *EntityToken
	masterToken *EntityToken
}

func NewClient() *Client {
	return &Client{
		http: http.DefaultClient,
	}
}

func (c *Client) LoggedIn() bool {
	return c.mcToken != nil && c.mcToken.ValidUntil.Before(time.Now())
}

func (c *Client) Login(ctx context.Context, liveToken *oauth2.Token) error {
	err := c.loginWithXbox(ctx, liveToken)
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

	resp, err := doPlayfabRequest[loginResponse](ctx, c.http, "/Client/LoginWithXbox?sdk="+minecraftDefaultSDK, xboxLoginRequest{
		CreateAccount: true,
		InfoRequestParameters: infoRequestParameters{
			PlayerProfile:   true,
			UserAccountInfo: true,
		},
		TitleID:   strings.ToUpper(minecraftTitleID),
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
	resp, err := doPlayfabRequest[EntityToken](ctx, c.http, "/Authentication/GetEntityToken?sdk="+minecraftDefaultSDK, entityTokenRequest{
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
	resp, err := doRequest[mcTokenResponse](ctx, c.http, "https://authorization.franchise.minecraft-services.net/api/v1.0/session/start", mcTokenRequest{
		Device: mcTokenDevice{
			ApplicationType:    "MinecraftPE",
			Capabilities:       []string{"RayTracing"},
			GameVersion:        protocol.CurrentVersion,
			ID:                 uuid.New().String(),
			Memory:             fmt.Sprint(16 * (1024 * 1024 * 1024)), // 16 GB
			Platform:           "Windows10",
			PlayFabTitleID:     strings.ToUpper(minecraftTitleID),
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
	return c.mcToken, nil
}

func doRequest[T any](ctx context.Context, client *http.Client, url string, payload any) (*T, error) {
	logrus.Tracef("doRequest: %s", url)
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

func doPlayfabRequest[T any](ctx context.Context, client *http.Client, endpoint string, payload any, token func(*http.Request)) (*Response[T], error) {
	logrus.Tracef("doPlayfabRequest: %s", endpoint)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://"+minecraftTitleID+".playfabapi.com"+endpoint, bytes.NewReader(body))
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
