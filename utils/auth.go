package utils

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/bedrock-tool/bedrocktool/utils/gatherings"
	"github.com/bedrock-tool/bedrocktool/utils/playfab"
	"github.com/bedrock-tool/bedrocktool/utils/xbox"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type authsrv struct {
	log     *logrus.Entry
	handler xbox.MSAuthHandler

	liveToken  *oauth2.Token
	deviceType *xbox.DeviceType

	discovery  *discovery.Discovery
	realms     *realms.Client
	playfab    *playfab.Client
	gatherings *gatherings.GatheringsClient
}

var Auth *authsrv = &authsrv{
	log: logrus.WithField("part", "Auth"),
}

// reads token from storage if there is one
func (a *authsrv) Startup() (err error) {
	tokenInfo, err := a.readToken()
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	if err != nil {
		return err
	}
	a.liveToken = tokenInfo.Token
	switch tokenInfo.DeviceType {
	case "Android":
		a.deviceType = &xbox.DeviceTypeAndroid
	case "iOS":
		a.deviceType = &xbox.DeviceTypeIOS
	case "Win32":
		a.deviceType = &xbox.DeviceTypeWindows
	case "Nintendo":
		a.deviceType = &xbox.DeviceTypeNintendo
	case "":
		a.deviceType = &xbox.DeviceTypeAndroid
	default:
		a.liveToken = nil
	}
	return nil
}

// if the user is currently logged in or not
func (a *authsrv) LoggedIn() bool {
	return a.liveToken != nil
}

// performs microsoft login using the handler passed
func (a *authsrv) SetHandler(handler xbox.MSAuthHandler) (err error) {
	a.handler = handler
	return nil
}

func (a *authsrv) Login(ctx context.Context, deviceType *xbox.DeviceType) (err error) {
	if deviceType == nil {
		deviceType = a.deviceType
	}
	if deviceType == nil {
		deviceType = &xbox.DeviceTypeAndroid
	}
	a.liveToken, err = xbox.RequestLiveTokenWriter(ctx, deviceType, a.handler)
	if err != nil {
		return err
	}
	a.deviceType = deviceType
	err = a.writeToken(tokenInfo{
		Token:      a.liveToken,
		DeviceType: deviceType.DeviceType,
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *authsrv) Logout() {
	a.liveToken = nil
	os.Remove("token.json")
	os.Remove("chain.bin")
}

func (a *authsrv) refreshLiveToken() error {
	if a.liveToken.Valid() {
		return nil
	}

	a.log.Info("Refreshing Microsoft Token")
	liveToken, err := xbox.RefreshToken(a.liveToken, a.deviceType)
	if err != nil {
		return err
	}
	a.liveToken = liveToken
	return a.writeToken(tokenInfo{
		Token:      a.liveToken,
		DeviceType: a.deviceType.DeviceType,
	})
}

var Ver1token func(f io.ReadSeeker, o any) error
var Tokene = func(w io.Writer, o any) error {
	return json.NewEncoder(w).Encode(o)
}

func readAuth[T any](name string) (*T, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var b = make([]byte, 1)
	_, err = f.ReadAt(b, 0)
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case '{':
		var o T
		e := json.NewDecoder(f)
		err = e.Decode(&o)
		if err != nil {
			return nil, err
		}
		return &o, nil
	case '1':
		if Ver1token != nil {
			var o T
			err = Ver1token(f, &o)
			if err != nil {
				return nil, err
			}
			return &o, nil
		}
	}

	return nil, errors.ErrUnsupported
}

func writeAuth(name string, o any) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return Tokene(f, o)
}

type tokenInfo struct {
	*oauth2.Token
	DeviceType string
}

// writes the livetoken to storage
func (a *authsrv) writeToken(token tokenInfo) error {
	return writeAuth("token.json", token)
}

// reads the live token from storage, returns os.ErrNotExist if no token is stored
func (a *authsrv) readToken() (*tokenInfo, error) {
	return readAuth[tokenInfo]("token.json")
}

var ErrNotLoggedIn = errors.New("not logged in")

// Token implements oauth2.TokenSource, returns ErrNotLoggedIn if there is no token, refreshes it if it expired
func (a *authsrv) Token() (t *oauth2.Token, err error) {
	if a.liveToken == nil {
		return nil, ErrNotLoggedIn
	}
	if !a.liveToken.Valid() {
		err = a.refreshLiveToken()
		if err != nil {
			return nil, err
		}
	}
	return a.liveToken, nil
}

func (a *authsrv) Discovery() (d *discovery.Discovery, err error) {
	if a.discovery == nil {
		a.discovery, err = discovery.GetDiscovery(Options.Env)
		if err != nil {
			return nil, err
		}
	}
	return a.discovery, nil
}

func (a *authsrv) Playfab(ctx context.Context) (*playfab.Client, error) {
	if a.playfab == nil {
		discovery, err := a.Discovery()
		if err != nil {
			return nil, err
		}
		a.playfab = playfab.NewClient(discovery, a)
	}
	if !a.playfab.LoggedIn() {
		err := a.playfab.Login(ctx)
		if err != nil {
			return nil, err
		}
	}
	return a.playfab, nil
}

func (a *authsrv) Gatherings(ctx context.Context) (*gatherings.GatheringsClient, error) {
	if a.gatherings == nil {
		playfabClient, err := a.Playfab(ctx)
		if err != nil {
			return nil, err
		}
		mcToken, err := playfabClient.MCToken()
		if err != nil {
			return nil, err
		}
		discovery, err := a.Discovery()
		if err != nil {
			return nil, err
		}
		a.gatherings = gatherings.NewGatheringsClient(mcToken, discovery)
	}
	return a.gatherings, nil
}

func (a *authsrv) Realms() *realms.Client {
	if a.realms == nil {
		a.realms = realms.NewClient(a, "")
	}
	return a.realms
}

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
	tok, err := jwt.ParseSigned(chain)
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

func (a *authsrv) readChain() (*chain, error) {
	return readAuth[chain]("chain.bin")
}

func (a *authsrv) writeChain(ch *chain) error {
	return writeAuth("chain.bin", ch)
}

func (a *authsrv) Chain(ctx context.Context) (ChainKey *ecdsa.PrivateKey, ChainData string, err error) {
	ch, err := a.readChain()
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return nil, "", err
	}
	if ch != nil && ch.DeviceType != a.deviceType.DeviceType {
		ch = nil
	}
	if ch == nil || ch.Expired() {
		ChainKey, ChainData, err := a.authChain(ctx)
		if err != nil {
			return nil, "", err
		}
		ch = &chain{
			ChainKey:   ChainKey,
			ChainData:  ChainData,
			DeviceType: a.deviceType.DeviceType,
		}
		err = a.writeChain(ch)
		if err != nil {
			return nil, "", err
		}
	}
	return ch.ChainKey, ch.ChainData, nil
}

// authChain requests the Minecraft auth JWT chain using the credentials passed. If successful, an encoded
// chain ready to be put in a login request is returned.
func (a *authsrv) authChain(ctx context.Context) (key *ecdsa.PrivateKey, chain string, err error) {
	key, _ = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)

	// Obtain the Live token, and using that the XSTS token.
	liveToken, err := a.Token()
	if err != nil {
		return nil, "", fmt.Errorf("request Live Connect token: %w", err)
	}
	xsts, err := xbox.RequestXBLToken(ctx, liveToken, "https://multiplayer.minecraft.net/", a.deviceType)
	if err != nil {
		return nil, "", fmt.Errorf("request XBOX Live token: %w", err)
	}

	xstsa := &auth.XBLToken{
		AuthorizationToken: xsts.AuthorizationToken,
	}

	// Obtain the raw chain data using the
	chain, err = auth.RequestMinecraftChain(ctx, xstsa, key)
	if err != nil {
		return nil, "", fmt.Errorf("request Minecraft auth chain: %w", err)
	}
	return key, chain, nil
}
