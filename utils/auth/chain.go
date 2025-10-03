package auth

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type chain struct {
	ChainKey   *ecdsa.PrivateKey
	ChainData  string
	DeviceType string
}

func (c *chain) UnmarshalJSON(b []byte) error {
	var m map[string]string
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	chainKeyBase64, err := base64.StdEncoding.DecodeString(m["ChainKey"])
	if err != nil {
		return err
	}
	chainKey, err := x509.ParseECPrivateKey(chainKeyBase64)
	if err != nil {
		return err
	}
	c.ChainKey = chainKey
	c.ChainData = m["ChainData"]
	c.DeviceType = m["DeviceType"]
	return nil
}

func (c *chain) MarshalJSON() ([]byte, error) {
	ChainKey, err := x509.MarshalECPrivateKey(c.ChainKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{
		"ChainKey":   base64.StdEncoding.EncodeToString(ChainKey),
		"ChainData":  c.ChainData,
		"DeviceType": c.DeviceType,
	})
}

func (c *chain) Expired() bool {
	var m map[string]any
	err := json.Unmarshal([]byte(c.ChainData), &m)
	if err != nil {
		return true
	}
	chain := m["chain"].([]any)[1].(string)
	tok, err := jwt.ParseSigned(chain, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return true
	}
	var mm map[string]any
	err = tok.UnsafeClaimsWithoutVerification(&mm)
	if err != nil {
		return true
	}
	exp := mm["exp"].(float64)
	t := time.Unix(int64(exp), 0)
	return time.Until(t) < 1*time.Hour
}
