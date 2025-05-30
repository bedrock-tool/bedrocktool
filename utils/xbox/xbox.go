package xbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

func generatePkce() (verifier, challenge string) {
	var b = make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	verifier = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

func generateCsrf() string {
	var b = make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

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
	r.Header.Set("Authorization", t.XBL())
}

func (t XBLToken) XBL() string {
	return fmt.Sprintf("XBL3.0 x=%v;%v", t.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash, t.AuthorizationToken.Token)
}

// RequestXBLToken requests an XBOX Live auth token using the passed Live token pair.
func RequestXBLToken(ctx context.Context, liveToken *oauth2.Token, relyingParty string, deviceType *DeviceType) (*XBLToken, error) {
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

	verifier, challenge := generatePkce()
	csrf := generateCsrf()
	_ = verifier

	sessionID, err := sisuAuthenticate(ctx, c, key, deviceToken, deviceType, challenge, csrf)
	if err != nil {
		return nil, err
	}

	return sisuAuthorize(ctx, c, key, liveToken, deviceToken, relyingParty, deviceType, sessionID)
}

func sisuAuthenticate(ctx context.Context, c *http.Client, key *ecdsa.PrivateKey, device *deviceToken, deviceType *DeviceType, pkceChallenge, csrf string) (string, error) {
	data, _ := json.Marshal(map[string]any{
		"AppId":       deviceType.ClientID,
		"TitleId":     deviceType.TitleID,
		"RedirectUri": "https://login.live.com/oauth20_desktop.srf",
		"deviceToken": device.Token,
		"Sandbox":     "RETAIL",
		"TokenType":   "code",
		"Offers":      []string{"service::user.auth.xboxlive.com::MBI_SSL"},
		"Query": map[string]any{
			"Display":             "",
			"CodeChallenge":       pkceChallenge,
			"CodeChallengeMethod": "S256",
			"State":               csrf,
		},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://sisu.xboxlive.com/authenticate", bytes.NewReader(data))
	req.Header.Set("x-xbl-contract-version", "1")
	req.Header.Set("User-Agent", deviceType.UserAgent)
	sign(req, data, key)

	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST %v: %w", "https://sisu.xboxlive.com/authenticate", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("POST %v: %v", "https://sisu.xboxlive.com/authenticate", resp.Status)
	}
	sessionID := resp.Header.Get("X-SessionId")
	return sessionID, nil
}

func sisuAuthorize(ctx context.Context, c *http.Client, key *ecdsa.PrivateKey, liveToken *oauth2.Token, device *deviceToken, relyingParty string, deviceType *DeviceType, sessionID string) (*XBLToken, error) {
	data, _ := json.Marshal(map[string]any{
		"AccessToken":       "t=" + liveToken.AccessToken,
		"AppId":             deviceType.ClientID,
		"deviceToken":       device.Token,
		"Sandbox":           "RETAIL",
		"UseModernGamertag": true,
		"SiteName":          "user.auth.xboxlive.com",
		"RelyingParty":      relyingParty,
		"SessionId":         sessionID,
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
		b, _ := io.ReadAll(resp.Body)
		_ = b
		return nil, fmt.Errorf("POST %v: %v", "https://sisu.xboxlive.com/authorize", resp.Status)
	}
	info := new(XBLToken)
	return info, json.NewDecoder(resp.Body).Decode(info)
}
