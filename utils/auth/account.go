package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Account struct {
	name       string
	env        string
	token      *tokenInfo
	discovery  *discovery.Discovery
	realms     *realms.Client
	gatherings *discovery.GatheringsService
}

type accountTokenSource struct {
	account *Account
}

func (a *accountTokenSource) Token() (t *oauth2.Token, err error) {
	return a.account.Token(context.Background())
}

func (a *Account) Name() string {
	return a.name
}

func (a *Account) Token(ctx context.Context) (t *oauth2.Token, err error) {
	if a.token == nil {
		return nil, ErrNotLoggedIn
	}
	if !a.token.LiveToken().Valid() {
		logrus.WithField("part", "Auth").Info("Refreshing Microsoft Token")
		liveToken, err := xbox.RefreshToken(a.token.LiveToken(), a.token.XboxDeviceType())
		if err != nil {
			return nil, err
		}
		a.token.Token = liveToken
		if err = writeAuth(tokenFileName(a.name), *a.token); err != nil {
			return nil, err
		}
	}
	return a.token.LiveToken(), nil
}

func (a *Account) Discovery() (d *discovery.Discovery, err error) {
	if a.discovery == nil {
		a.discovery, err = discovery.GetDiscovery(a.env)
		if err != nil {
			return nil, err
		}
	}
	return a.discovery, nil
}

func (a *Account) PlayfabXblToken(ctx context.Context) (*xbox.XBLToken, error) {
	liveToken, err := a.Token(ctx)
	if err != nil {
		return nil, err
	}
	return xbox.RequestXBLToken(ctx, liveToken, "rp://playfabapi.com/", a.token.XboxDeviceType())
}

func (a *Account) MCToken(ctx context.Context) (*discovery.MCToken, error) {
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
		if err = writeAuth(tokenFileName(a.name), *a.token); err != nil {
			return nil, err
		}
	}
	return a.token.MCToken, nil
}

func (a *Account) Gatherings(ctx context.Context) (*discovery.GatheringsService, error) {
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

func (a *Account) Realms() *realms.Client {
	if a.realms == nil {
		a.realms = realms.NewClient(&accountTokenSource{account: a}, nil, "")
	}
	return a.realms
}

func (a *Account) Chain(ctx context.Context) (ChainKey *ecdsa.PrivateKey, ChainData string, err error) {
	ch, err := readAuth[chain](chainFileName(a.name))
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
		if err = writeAuth(chainFileName(a.name), ch); err != nil {
			return nil, "", err
		}
	}
	return ch.ChainKey, ch.ChainData, nil
}

func (a *Account) PfToken(ctx context.Context, publicKey *ecdsa.PublicKey) (string, error) {
	discovery, err := a.Discovery()
	if err != nil {
		return "", err
	}
	authService, err := discovery.AuthService()
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

func (a *Account) XBLToken(ctx context.Context) (*auth.XBLToken, error) {
	liveToken, err := a.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("request Live Connect token: %w", err)
	}
	xsts, err := xbox.RequestXBLToken(ctx, liveToken, "https://multiplayer.minecraft.net/", a.token.XboxDeviceType())
	if err != nil {
		return nil, fmt.Errorf("request XBOX Live token: %w", err)
	}
	return &auth.XBLToken{
		AuthorizationToken: xsts.AuthorizationToken,
	}, nil
}

func (a *Account) authChain(ctx context.Context) (key *ecdsa.PrivateKey, chain string, err error) {
	key, _ = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	xstsa, err := a.XBLToken(ctx)
	if err != nil {
		return nil, "", err
	}
	chain, err = auth.RequestMinecraftChain(ctx, xstsa, key)
	if err != nil {
		return nil, "", fmt.Errorf("request Minecraft auth chain: %w", err)
	}
	return key, chain, nil
}
