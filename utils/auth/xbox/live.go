package xbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type MSAuthHandler interface {
	// called with the url the user needs to go to, the code they need to enter
	AuthCode(uri, code string)
	// called when the auth completes
	Finished(err error)
}

type msAuthWriter struct {
	w io.Writer
}

func (m *msAuthWriter) AuthCode(uri, code string) {
	betterUri := "https://login.live.com/oauth20_remoteconnect.srf?otc=" + code
	fmt.Fprintf(m.w, "Authenticate at %s\n", betterUri)
}

func (m *msAuthWriter) Finished(err error) {
	if err != nil {
		fmt.Fprintf(m.w, "Failed to Authenticate: %s\n", err)
	} else {
		fmt.Fprint(m.w, "Authentication successful.\n")
	}
}

// RequestLiveTokenWriter does a login request for Microsoft Live Connect using device auth. A login URL will
// be printed to the io.Writer passed with a user code which the user must use to submit.
// Once fully authenticated, an oauth2 token is returned which may be used to login to XBOX Live.
func RequestLiveTokenWriter(ctx context.Context, deviceType *DeviceType, h MSAuthHandler) (*oauth2.Token, error) {
	if h == nil {
		h = &msAuthWriter{os.Stdout}
	}
	d, err := startDeviceAuth(deviceType)
	if err != nil {
		h.Finished(err)
		return nil, err
	}

	h.AuthCode(d.VerificationURI, d.UserCode)

	ticker := time.NewTicker(time.Second * time.Duration(d.Interval))
	defer ticker.Stop()

	var errors int
	for {
		select {
		case <-ticker.C:
			t, err := pollDeviceAuth(deviceType, d.DeviceCode)
			if err != nil {
				errors++
				if errors > 5 {
					err = fmt.Errorf("error polling for device auth: %w", err)
					h.Finished(err)
					return nil, err
				}
			}
			// If the token could not be obtained yet (authentication wasn't finished yet), the token is nil.
			// We just retry if this is the case.
			if t != nil {
				h.Finished(nil)
				return t, nil
			}
		case <-ctx.Done():
			h.Finished(ctx.Err())
			return nil, ctx.Err()
		}
	}
}

// startDeviceAuth starts the device auth, retrieving a login URI for the user and a code the user needs to
// enter.
func startDeviceAuth(deviceType *DeviceType) (*deviceAuthConnect, error) {
	resp, err := http.PostForm("https://login.live.com/oauth20_connect.srf", url.Values{
		"client_id":     {deviceType.ClientID},
		"scope":         {"service::user.auth.xboxlive.com::MBI_SSL"},
		"response_type": {"device_code"},
	})
	if err != nil {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_connect.srf: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_connect.srf: %v", resp.Status)
	}
	data := new(deviceAuthConnect)
	return data, json.NewDecoder(resp.Body).Decode(data)
}

// pollDeviceAuth polls the token endpoint for the device code. A token is returned if the user authenticated
// successfully. If the user has not yet authenticated, err is nil but the token is nil too.
func pollDeviceAuth(deviceType *DeviceType, deviceCode string) (t *oauth2.Token, err error) {
	resp, err := http.PostForm(microsoft.LiveConnectEndpoint.TokenURL, url.Values{
		"client_id":   {deviceType.ClientID},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
	})
	if err != nil {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_token.srf: %w", err)
	}
	poll := new(deviceAuthPoll)
	if err := json.NewDecoder(resp.Body).Decode(poll); err != nil {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_token.srf: json decode: %w", err)
	}
	_ = resp.Body.Close()
	if poll.Error == "authorization_pending" {
		return nil, nil
	} else if poll.Error == "" {
		return &oauth2.Token{
			AccessToken:  poll.AccessToken,
			TokenType:    poll.TokenType,
			RefreshToken: poll.RefreshToken,
			Expiry:       time.Now().Add(time.Duration(poll.ExpiresIn) * time.Second),
		}, nil
	}
	return nil, fmt.Errorf("non-empty unknown poll error: %v", poll.Error)
}

// RefreshToken refreshes the oauth2.Token passed and returns a new oauth2.Token. An error is returned if
// refreshing was not successful.
func RefreshToken(t *oauth2.Token, deviceType *DeviceType) (*oauth2.Token, error) {
	// This function unfortunately needs to exist because golang.org/x/oauth2 does not pass the scope to this
	// request, which Microsoft Connect enforces.
	resp, err := http.PostForm(microsoft.LiveConnectEndpoint.TokenURL, url.Values{
		"client_id":     {deviceType.ClientID},
		"scope":         {"service::user.auth.xboxlive.com::MBI_SSL"},
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
	})
	if err != nil {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_token.srf: %w", err)
	}
	poll := new(deviceAuthPoll)
	if err := json.NewDecoder(resp.Body).Decode(poll); err != nil {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_token.srf: json decode: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("POST https://login.live.com/oauth20_token.srf: refresh error: %v", poll.Error)
	}
	return &oauth2.Token{
		AccessToken:  poll.AccessToken,
		TokenType:    poll.TokenType,
		RefreshToken: poll.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(poll.ExpiresIn) * time.Second),
	}, nil
}

type deviceAuthConnect struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expiresIn"`
}

type deviceAuthPoll struct {
	Error        string `json:"error"`
	UserID       string `json:"user_id"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
