package xbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type DeviceType struct {
	DeviceType string
	ClientID   string
	TitleID    string
	Version    string
	UserAgent  string
}

var (
	DeviceTypeAndroid = DeviceType{
		DeviceType: "Android",
		ClientID:   "0000000048183522",
		TitleID:    "1739947436",
		Version:    "8.0.0",
		UserAgent:  "XAL Android 2020.07.20200714.000",
	}
	DeviceTypeIOS = DeviceType{
		DeviceType: "iOS",
		ClientID:   "000000004c17c01a",
		TitleID:    "1810924247",
		Version:    "15.6.1",
		UserAgent:  "XAL iOS 2021.11.20211021.000",
	}
	DeviceTypeWindows = DeviceType{
		DeviceType: "Win32",
		ClientID:   "0000000040159362",
		TitleID:    "896928775",
		Version:    "10.0.25398.4909",
		UserAgent:  "XAL Win32 2021.11.20220411.002",
	}
	DeviceTypeNintendo = DeviceType{
		DeviceType: "Nintendo",
		ClientID:   "00000000441cc96b",
		TitleID:    "2047319603",
		Version:    "0.0.0",
		UserAgent:  "XAL",
	}
	DeviceTypePlaystation = DeviceType{
		DeviceType: "Playstation",
		ClientID:   "000000004827c78e",
		TitleID:    "idk",
		Version:    "10.0.0",
		UserAgent:  "XAL",
	}
)

// deviceToken is the token obtained by requesting a device token by posting to xblDeviceAuthURL. Its Token
// field may be used in a request to obtain the XSTS token.
type deviceToken struct {
	Token string
}

// obtainDeviceToken sends a POST request to the device auth endpoint using the ECDSA private key passed to
// sign the request.
func obtainDeviceToken(ctx context.Context, c *http.Client, key *ecdsa.PrivateKey, deviceType *DeviceType) (token *deviceToken, err error) {
	var properties = map[string]any{
		"AuthMethod": "ProofOfPossession",
		"Id":         "",
		"DeviceType": deviceType.DeviceType,
		"Version":    deviceType.Version,
		"ProofKey": map[string]any{
			"crv": "P-256",
			"alg": "ES256",
			"use": "sig",
			"kty": "EC",
			"x":   base64.RawURLEncoding.EncodeToString(padTo32Bytes(key.PublicKey.X)),
			"y":   base64.RawURLEncoding.EncodeToString(padTo32Bytes(key.PublicKey.Y)),
		},
	}

	switch deviceType.DeviceType {
	case "Android", "Nintendo":
		properties["Id"] = "{" + uuid.NewString() + "}"
	case "iOS":
		properties["Id"] = strings.ToUpper(uuid.NewString())
	case "Playstation":
		properties["Id"] = uuid.NewString()
	case "Win32", "Xbox":
		properties["Id"] = "{" + strings.ToUpper(uuid.NewString()) + "}"
		properties["SerialNumber"] = properties["Id"]
	default:
		return nil, errors.New("unknown device type")
	}

	data, _ := json.Marshal(map[string]any{
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
		"Properties":   properties,
	})
	request, _ := http.NewRequestWithContext(ctx, "POST", "https://device.auth.xboxlive.com/device/authenticate", bytes.NewReader(data))
	request.Header.Set("X-Xbl-contract-version", "1")
	request.Header.Set("User-Agent", deviceType.UserAgent)
	request.Header.Set("Content-Type", "application/json;charset=utf-8")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Pragma", "no-cache")
	request.Header.Set("Cache-Control", "no-store, must-revalidate, no-cache")
	request.Header.Set("Accept-Encoding", "gzip, deflate, compress")
	request.Header.Set("Accept-Language", "en-US, en;q=0.9")
	sign(request, data, key)

	resp, err := c.Do(request)
	if err != nil {
		return nil, fmt.Errorf("POST %v: %w", "https://device.auth.xboxlive.com/device/authenticate", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("POST %v: %v", "https://device.auth.xboxlive.com/device/authenticate", resp.Status)
	}
	token = &deviceToken{}
	return token, json.NewDecoder(resp.Body).Decode(token)
}
