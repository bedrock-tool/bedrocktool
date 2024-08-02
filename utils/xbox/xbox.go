package xbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// XBLToken holds info on the authorization token used for authenticating with XBOX Live.
type XBLToken struct {
	AuthorizationToken struct {
		DisplayClaims struct {
			UserInfo []struct {
				GamerTag string `json:"gtg"`
				XUID     string `json:"xid"`
				UserHash string `json:"uhs"`
			} `json:"xui"`
		}
		Token string
	}
}

// SetAuthHeader returns a string that may be used for the 'Authorization' header used for Minecraft
// related endpoints that need an XBOX Live authenticated caller.
func (t XBLToken) SetAuthHeader(r *http.Request) {
	r.Header.Set("Authorization", fmt.Sprintf("XBL3.0 x=%v;%v", t.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash, t.AuthorizationToken.Token))
}

// RequestXBLToken requests an XBOX Live auth token using the passed Live token pair.
func RequestXBLToken(ctx context.Context, liveToken *oauth2.Token, relyingParty string, deviceType *DeviceType) (*XBLToken, error) {
	if deviceType == nil {
		deviceType = &DeviceTypeAndroid
	}
	if !liveToken.Valid() {
		return nil, fmt.Errorf("live token is no longer valid")
	}
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Renegotiation:      tls.RenegotiateOnceAsClient,
				InsecureSkipVerify: true,
			},
		},
	}
	defer c.CloseIdleConnections()

	// We first generate an ECDSA private key which will be used to provide a 'ProofKey' to each of the
	// requests, and to sign these requests.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	deviceToken, err := obtainDeviceToken(ctx, c, key, deviceType)
	if err != nil {
		return nil, err
	}
	return obtainXBLToken(ctx, c, key, liveToken, deviceToken, relyingParty, deviceType)
}

func obtainXBLToken(ctx context.Context, c *http.Client, key *ecdsa.PrivateKey, liveToken *oauth2.Token, device *deviceToken, relyingParty string, deviceType *DeviceType) (*XBLToken, error) {
	data, _ := json.Marshal(map[string]any{
		"AccessToken":       "t=" + liveToken.AccessToken,
		"AppId":             deviceType.ClientID,
		"deviceToken":       device.Token,
		"Sandbox":           "RETAIL",
		"UseModernGamertag": true,
		"SiteName":          "user.auth.xboxlive.com",
		"RelyingParty":      relyingParty,
		"ProofKey": map[string]any{
			"crv": "P-256",
			"alg": "ES256",
			"use": "sig",
			"kty": "EC",
			"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.Bytes()),
			"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.Bytes()),
		},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://sisu.xboxlive.com/authorize", bytes.NewReader(data))
	req.Header.Set("x-xbl-contract-version", "1")
	req.Header.Set("User-Agent", deviceType.UserAgent)
	sign(req, data, key)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %v: %w", "https://sisu.xboxlive.com/authorize", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("POST %v: %v", "https://sisu.xboxlive.com/authorize", resp.Status)
	}
	info := new(XBLToken)
	return info, json.NewDecoder(resp.Body).Decode(info)
}
