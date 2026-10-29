package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/df-mc/go-playfab/v2"
	"github.com/df-mc/go-xsapi/v2"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sandertv/gophertunnel/minecraft/service"
	"github.com/sandertv/gophertunnel/minecraft/service/gatherings"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Account struct {
	name       string
	env        string
	token      *tokenInfo
	playfab    atomic.Pointer[playfab.Client]
	xblClient  atomic.Pointer[xsapi.Client]
	mcToken    atomic.Pointer[service.Token]
	realms     atomic.Pointer[realms.Client]
	gatherings atomic.Pointer[gatherings.Client]
}

func (account *Account) Name() string {
	return account.name
}

func (account *Account) LiveToken(ctx context.Context) (t *oauth2.Token, err error) {
	if account.token == nil {
		return nil, ErrNotLoggedIn
	}
	liveToken := account.token.LiveToken()
	if !liveToken.Valid() {
		logrus.WithField("part", "Auth").Info("Refreshing Microsoft Token")
		conf := account.token.AuthConfig()
		if conf == nil {
			return nil, fmt.Errorf("invalid token")
		}
		liveToken, err := conf.RefreshTokenSource(liveToken).Token()
		if err != nil {
			return nil, err
		}
		account.token.Token = liveToken
		if err = writeAuth(tokenFileName(account.name), *account.token); err != nil {
			return nil, err
		}
	}
	return account.token.LiveToken(), nil
}

func checkMCToken(mcToken *service.Token) bool {
	if mcToken.ValidUntil.Before(time.Now()) {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.Split(mcToken.AuthorizationHeader, ".")[1])
	if err != nil {
		logrus.Error("invalid mctoken, refreshing")
		return false
	}
	var claims map[string]any
	if err := json.Unmarshal(data, &claims); err != nil {
		logrus.Error("invalid mctoken, refreshing")
		return false
	}
	claimVer := claims["ver"].(string)
	if claimVer != protocol.CurrentVersion {
		logrus.Info("mctoken for older version, refreshing")
		return false
	}
	return true
}

func (account *Account) AuthService(ctx context.Context) (*service.AuthorizationEnvironment, error) {
	discovery, err := service.Default(ctx)
	if err != nil {
		return nil, err
	}
	var authService service.AuthorizationEnvironment
	err = discovery.Environment(&authService)
	if err != nil {
		return nil, err
	}
	return &authService, nil
}

func (account *Account) MCToken(ctx context.Context) (*service.Token, error) {
	if mcToken := account.mcToken.Load(); mcToken != nil && checkMCToken(mcToken) {
		return mcToken, nil
	}
	authService, err := account.AuthService(ctx)
	if err != nil {
		return nil, err
	}

	playfabClient, err := account.PlayFabClient(ctx)
	if err != nil {
		return nil, err
	}
	sessionTicket, err := playfabClient.SessionTicket(ctx)
	if err != nil {
		return nil, err
	}

	mcToken, err := authService.Token(ctx, service.TokenConfig{
		User: service.UserConfig{
			TokenType: "PlayFab",
			Token:     sessionTicket,
		},
	})
	if err != nil {
		return nil, err
	}
	account.mcToken.Store(mcToken)
	return mcToken, nil
}

func (account *Account) Gatherings(ctx context.Context) (*gatherings.Client, error) {
	if gatheringsClient := account.gatherings.Load(); gatheringsClient != nil {
		return gatheringsClient, nil
	}
	tokenSource, err := account.TokenSource(ctx)
	if err != nil {
		return nil, err
	}
	gatheringsClient := gatherings.NewClient(tokenSource)
	account.gatherings.Store(gatheringsClient)
	return gatheringsClient, nil
}

func (account *Account) Realms(ctx context.Context) (*realms.Client, error) {
	if realmsClient := account.realms.Load(); realmsClient != nil {
		return realmsClient, nil
	}
	t, err := account.LiveToken(ctx)
	if err != nil {
		return nil, err
	}
	realmsClient := realms.NewClient(oauth2.StaticTokenSource(t), nil, "")
	account.realms.Store(realmsClient)
	return realmsClient, nil
}

/*
func (a *Account) MultiplayerSessionToken(ctx context.Context, publicKey *ecdsa.PublicKey) (string, error) {
	discovery, err := a.Discovery(ctx)
	if err != nil {
		return "", err
	}
	authService, err := authservice.NewAuthService(discovery)
	if err != nil {
		return "", err
	}
	mcToken, err := a.MCToken(ctx)
	if err != nil {
		return "", err
	}

	keyData, _ := x509.MarshalPKIXPublicKey(publicKey)
	signedToken, _, err := authService.MultiplayerSessionStart(ctx, keyData, mcToken)
	return signedToken, err
}
*/

var _ oauth2.TokenSource = (*Account)(nil)

// Token implements [oauth2.TokenSource].
func (account *Account) Token() (*oauth2.Token, error) {
	return account.LiveToken(context.Background())
}

func (account *Account) XBLClient(ctx context.Context) (*xsapi.Client, error) {
	if xblClient := account.xblClient.Load(); xblClient != nil {
		return xblClient, nil
	}
	xblClient, err := xsapi.ClientConfig{}.New(ctx, defaultDeviceType.New(account, nil))
	if err != nil {
		return nil, err
	}
	account.xblClient.Store(xblClient)
	return xblClient, nil
}

func (account *Account) PlayFabClient(ctx context.Context) (*playfab.Client, error) {
	xblClient, err := account.XBLClient(ctx)
	if err != nil {
		return nil, err
	}
	playfabClient, err := playfab.LoginWithXbox(ctx, "20CA2", xblClient, playfab.ClientConfig{
		CreateAccount: true,
	})
	if err != nil {
		return nil, err
	}
	account.playfab.Store(playfabClient)
	return playfabClient, nil
}

func (account *Account) TokenSource(ctx context.Context) (service.TokenSource, error) {
	authService, err := account.AuthService(ctx)
	if err != nil {
		return nil, err
	}

	playfabClient, err := account.PlayFabClient(ctx)
	if err != nil {
		return nil, err
	}
	tokenSource := authService.TokenSource(playfabClient, service.TokenConfig{})
	return tokenSource, nil
}

// maybe add MultiplayerTokenSource

func (account *Account) Close() error {
	if xblClient := account.xblClient.Swap(nil); xblClient != nil {
		_ = xblClient.Close()
	}
	if playfabClient := account.playfab.Swap(nil); playfabClient != nil {
		_ = playfabClient.Close()
	}
	return nil
}
