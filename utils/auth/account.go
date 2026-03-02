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
	"github.com/bedrock-tool/bedrocktool/utils/franchise/authservice"
	"github.com/bedrock-tool/bedrocktool/utils/franchise/discovery"
	"github.com/bedrock-tool/bedrocktool/utils/franchise/gatherings"
	"github.com/bedrock-tool/bedrocktool/utils/franchise/signaling"
	"github.com/df-mc/go-xsapi"
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
	gatherings *gatherings.GatheringsService
	signaling  *signaling.SignalingService
}

type liveTokenSource struct {
	account *Account
}

func (a liveTokenSource) Token() (t *oauth2.Token, err error) {
	return a.account.LiveToken(context.Background())
}

type xsapiTokenSource struct {
	account *Account
}

var _ xsapi.TokenSource = xsapiTokenSource{}

type xsapiToken struct {
	*auth.XBLToken
}

func (x xsapiToken) DisplayClaims() xsapi.DisplayClaims {
	return xsapi.DisplayClaims{
		GamerTag: x.XBLToken.AuthorizationToken.DisplayClaims.UserInfo[0].GamerTag,
		XUID:     x.XBLToken.AuthorizationToken.DisplayClaims.UserInfo[0].XUID,
		UserHash: x.XBLToken.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash,
	}
}

func (x xsapiToken) String() string {
	return x.AuthorizationToken.Token
}

func (x xsapiTokenSource) Token() (xsapi.Token, error) {
	token, err := x.account.XBLToken(context.Background(), "https://multiplayer.minecraft.net/")
	if err != nil {
		return nil, err
	}
	return xsapiToken{token}, nil
}

func (a *Account) Name() string {
	return a.name
}

func (a *Account) LiveToken(ctx context.Context) (t *oauth2.Token, err error) {
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

func (a *Account) Discovery(ctx context.Context) (d *discovery.Discovery, err error) {
	if a.discovery == nil {
		a.discovery, err = discovery.GetDiscovery(ctx, a.env)
		if err != nil {
			return nil, err
		}
	}
	return a.discovery, nil
}

func (a *Account) MCToken(ctx context.Context) (*authservice.MCToken, error) {
	if a.token.MCToken == nil || a.token.MCToken.ValidUntil.Before(time.Now()) {
		discovery, err := a.Discovery(ctx)
		if err != nil {
			return nil, err
		}
		authService, err := authservice.NewAuthService(discovery)
		if err != nil {
			return nil, err
		}
		pfXblToken, err := a.XBLToken(ctx, "rp://playfabapi.com/")
		if err != nil {
			return nil, err
		}

		res, err := authService.StartSession(ctx, pfXblToken.Token(), authService.Config.PlayfabTitleID)
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

func (a *Account) Gatherings(ctx context.Context) (*gatherings.GatheringsService, error) {
	if a.gatherings == nil {
		discovery, err := a.Discovery(ctx)
		if err != nil {
			return nil, err
		}
		gatheringsService, err := gatherings.NewGatheringsService(discovery)
		if err != nil {
			return nil, err
		}
		a.gatherings = gatheringsService
	}
	return a.gatherings, nil
}

func (a *Account) Signaling(ctx context.Context) (*signaling.SignalingService, error) {
	if a.gatherings == nil {
		discovery, err := a.Discovery(ctx)
		if err != nil {
			return nil, err
		}
		signalingService, err := signaling.NewSignalingService(discovery)
		if err != nil {
			return nil, err
		}
		a.signaling = signalingService
	}
	return a.signaling, nil
}

func (a *Account) Realms() *realms.Client {
	if a.realms == nil {
		a.realms = realms.NewClient(liveTokenSource{account: a}, nil, "")
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

func (a *Account) XBLToken(ctx context.Context, relyingParty string) (*auth.XBLToken, error) {
	liveToken, err := a.LiveToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("request Live Connect token: %w", err)
	}
	xsts, err := xbox.RequestXBLToken(ctx, liveToken, relyingParty, a.token.XboxDeviceType())
	if err != nil {
		return nil, fmt.Errorf("request XBOX Live token: %w", err)
	}
	return &auth.XBLToken{
		AuthorizationToken: xsts.AuthorizationToken,
	}, nil
}

func (a *Account) authChain(ctx context.Context) (key *ecdsa.PrivateKey, chain string, err error) {
	key, _ = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	xstsa, err := a.XBLToken(ctx, "https://multiplayer.minecraft.net/")
	if err != nil {
		return nil, "", err
	}
	chain, err = auth.RequestMinecraftChain(ctx, xstsa, key)
	if err != nil {
		return nil, "", fmt.Errorf("request Minecraft auth chain: %w", err)
	}
	return key, chain, nil
}
