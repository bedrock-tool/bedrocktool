package utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/bedrock-tool/bedrocktool/utils/gatherings"
	"github.com/bedrock-tool/bedrocktool/utils/playfab"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const TokenFile = "token.json"

type authsrv struct {
	log        *logrus.Entry
	handler    auth.MSAuthHandler
	liveToken  *oauth2.Token
	discovery  *discovery.Discovery
	Realms     *realms.Client
	playfab    *playfab.Client
	gatherings *gatherings.GatheringsClient
}

var Auth *authsrv = &authsrv{
	log: logrus.WithField("part", "Auth"),
}

// reads token from storage if there is one
func (a *authsrv) Startup() (err error) {
	a.liveToken, err = a.readToken()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	return a.afterLogin()
}

func (a *authsrv) afterLogin() (err error) {
	a.discovery, err = discovery.GetDiscovery(Options.Env)
	if err != nil {
		return err
	}

	/*
		realmsService, err := a.discovery.RealmsfrontendService()
		if err != nil {
			return err
		}
	*/

	a.Realms = realms.NewClient(a, "")
	a.playfab = playfab.NewClient(a.discovery)
	return nil
}

// if the user is currently logged in or not
func (a *authsrv) LoggedIn() bool {
	return a.liveToken != nil
}

// performs microsoft login using the handler passed
func (a *authsrv) SetHandler(handler auth.MSAuthHandler) (err error) {
	a.handler = handler
	return nil
}

func (a *authsrv) Login(ctx context.Context) (err error) {
	a.liveToken, err = auth.RequestLiveTokenWriter(ctx, a.handler)
	if err != nil {
		return err
	}
	err = a.writeToken(a.liveToken)
	if err != nil {
		return err
	}
	return a.afterLogin()
}

func (a *authsrv) Logout() {
	a.liveToken = nil
	os.Remove(TokenFile)
}

func (a *authsrv) refreshLiveToken() (err error) {
	if a.liveToken.Valid() {
		return nil
	}

	a.log.Info("Refreshing Microsoft Token")
	a.liveToken, err = auth.RefreshToken(a.liveToken)
	if err != nil {
		return err
	}

	err = a.writeToken(a.liveToken)
	if err != nil {
		return err
	}
	return nil
}

var Ver1token func(f io.ReadSeeker) (*oauth2.Token, error)
var Tokene = func(t *oauth2.Token, w io.Writer) error {
	return json.NewEncoder(w).Encode(t)
}

// writes the livetoken to storage
func (a *authsrv) writeToken(token *oauth2.Token) error {
	f, err := os.Create(TokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return Tokene(token, f)
}

// reads the live token from storage, returns os.ErrNotExist if no token is stored
func (a *authsrv) readToken() (*oauth2.Token, error) {
	f, err := os.Open(TokenFile)
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
		var token oauth2.Token
		e := json.NewDecoder(f)
		err = e.Decode(&token)
		if err != nil {
			return nil, err
		}
		return &token, nil
	case '1':
		if Ver1token != nil {
			return Ver1token(f)
		}
	}

	return nil, errors.New("unsupported token file")
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

func (a *authsrv) Playfab(ctx context.Context) (*playfab.Client, error) {
	if a.playfab.LoggedIn() {
		return a.playfab, nil
	}
	liveToken, err := a.Token()
	if err != nil {
		return nil, err
	}
	err = a.playfab.Login(ctx, liveToken)
	if err != nil {
		return nil, err
	}
	return a.playfab, nil
}

func (a *authsrv) Gatherings(ctx context.Context) (*gatherings.GatheringsClient, error) {
	if a.gatherings != nil {
		return a.gatherings, nil
	}
	playfabClient, err := a.Playfab(ctx)
	if err != nil {
		return nil, err
	}
	mcToken, err := playfabClient.MCToken()
	if err != nil {
		return nil, err
	}
	a.gatherings = gatherings.NewGatheringsClient(mcToken, a.discovery)
	return a.gatherings, nil
}
