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
	"sync/atomic"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
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
	env     string

	token *tokenInfo

	discovery  *discovery.Discovery
	realms     *realms.Client
	gatherings *discovery.GatheringsService
}

var Auth *authsrv = &authsrv{
	log: logrus.WithField("part", "Auth"),
}

// reads token from storage if there is one
func (a *authsrv) Startup(env string) (err error) {
	a.token = nil
	a.env = env
	tokenInfo, err := a.readToken()
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	if err != nil {
		return err
	}
	a.token = tokenInfo

	return nil
}

// if the user is currently logged in or not
func (a *authsrv) LoggedIn() bool {
	return a.token != nil
}

// performs microsoft login using the handler passed
func (a *authsrv) SetHandler(handler xbox.MSAuthHandler) (err error) {
	a.handler = handler
	return nil
}

func (a *authsrv) Login(ctx context.Context, deviceType *xbox.DeviceType) (err error) {
	if deviceType == nil {
		deviceType = a.token.DeviceType2()
	}
	if deviceType == nil {
		deviceType = &xbox.DeviceTypeAndroid
	}
	liveToken, err := xbox.RequestLiveTokenWriter(ctx, deviceType, a.handler)
	if err != nil {
		return err
	}
	a.token = &tokenInfo{
		Token:      liveToken,
		DeviceType: deviceType.DeviceType,
	}
	if err = a.writeToken(); err != nil {
		return err
	}
	return nil
}

func (a *authsrv) Logout() {
	a.token = nil
	os.Remove("token.json")
	os.Remove("chain.bin")
}

func (a *authsrv) refreshLiveToken() error {
	if a.token.LiveToken().Valid() {
		return nil
	}

	a.log.Info("Refreshing Microsoft Token")
	liveToken, err := xbox.RefreshToken(a.token.LiveToken(), a.token.DeviceType2())
	if err != nil {
		return err
	}
	a.token.Token = liveToken
	return a.writeToken()
}

var Ver1token func(f io.ReadSeeker, o any) error
var Tokene = func(w io.Writer, o any) error {
	return json.NewEncoder(w).Encode(o)
}

func readAuth[T any](name string) (*T, error) {
	f, err := os.Open(PathData(name))
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
	f, err := os.Create(PathData(name))
	if err != nil {
		return err
	}
	defer f.Close()
	return Tokene(f, o)
}

type tokenInfo struct {
	*oauth2.Token
	DeviceType string
	MCToken    *discovery.MCToken
}

func (t *tokenInfo) LiveToken() *oauth2.Token {
	return t.Token
}

func (t *tokenInfo) DeviceType2() *xbox.DeviceType {
	switch t.DeviceType {
	case "Android":
		return &xbox.DeviceTypeAndroid
	case "iOS":
		return &xbox.DeviceTypeIOS
	case "Win32":
		return &xbox.DeviceTypeWindows
	case "Nintendo":
		return &xbox.DeviceTypeNintendo
	case "":
		return &xbox.DeviceTypeAndroid
	default:
		return nil
	}
}

// writes the livetoken to storage
func (a *authsrv) writeToken() error {
	return writeAuth("token.json", *a.token)
}

// reads the live token from storage, returns os.ErrNotExist if no token is stored
func (a *authsrv) readToken() (*tokenInfo, error) {
	return readAuth[tokenInfo]("token.json")
}

var ErrNotLoggedIn = errors.New("not logged in")

// Token implements oauth2.TokenSource, returns ErrNotLoggedIn if there is no token, refreshes it if it expired
func (a *authsrv) Token() (t *oauth2.Token, err error) {
	if a.token == nil {
		return nil, ErrNotLoggedIn
	}
	if !a.token.LiveToken().Valid() {
		err = a.refreshLiveToken()
		if err != nil {
			return nil, err
		}
	}
	return a.token.LiveToken(), nil
}

func (a *authsrv) Discovery() (d *discovery.Discovery, err error) {
	if a.discovery == nil {
		a.discovery, err = discovery.GetDiscovery(a.env)
		if err != nil {
			return nil, err
		}
	}
	return a.discovery, nil
}

func (a *authsrv) PlayfabXblToken(ctx context.Context) (*xbox.XBLToken, error) {
	liveToken, err := a.Token()
	if err != nil {
		return nil, err
	}
	xboxToken, err := xbox.RequestXBLToken(ctx, liveToken, "rp://playfabapi.com/", &xbox.DeviceTypeAndroid)
	if err != nil {
		return nil, err
	}
	return xboxToken, nil
}

func (a *authsrv) MCToken(ctx context.Context) (*discovery.MCToken, error) {
	if a.token.MCToken == nil || a.token.MCToken.ValidUntil.Before(time.Now()) {
		discovery, err := a.Discovery()
		if err != nil {
			return nil, err
		}
		authService, err := discovery.AuthService()
		if err != nil {
			return nil, err
		}
		pfXblToken, err := a.PlayfabXblToken(ctx)
		if err != nil {
			return nil, err
		}
		res, err := authService.StartSession(ctx, pfXblToken.XBL(), authService.PlayfabTitleID)
		if err != nil {
			return nil, err
		}
		a.token.MCToken = res
		err = a.writeToken()
		if err != nil {
			return nil, err
		}
	}
	return a.token.MCToken, nil
}

func (a *authsrv) Gatherings(ctx context.Context) (*discovery.GatheringsService, error) {
	if a.gatherings == nil {
		discovery, err := a.Discovery()
		if err != nil {
			return nil, err
		}
		mcToken, err := a.MCToken(ctx)
		if err != nil {
			return nil, err
		}

		gatheringService, err := discovery.GatheringsService()
		if err != nil {
			return nil, err
		}
		gatheringService.SetToken(mcToken)

		a.gatherings = gatheringService
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
	if ch != nil && ch.DeviceType != a.token.DeviceType {
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
			DeviceType: a.token.DeviceType,
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
	xsts, err := xbox.RequestXBLToken(ctx, liveToken, "https://multiplayer.minecraft.net/", a.token.DeviceType2())
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

var authCtxCancel atomic.Pointer[context.CancelFunc]

func CancelLogin() {
	cancel := authCtxCancel.Swap(nil)
	(*cancel)()
}

func RequestLogin() chan error {
	errC := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		authCtxCancel.Store(&cancel)
		deviceType := &xbox.DeviceTypeAndroid
		err := Auth.Login(ctx, deviceType)
		messages.SendEvent(&messages.EventAuthFinished{
			Error: err,
		})
		if err != nil {
			errC <- err
		}
		close(errC)
	}()
	return errC
}
