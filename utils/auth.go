package utils

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const TokenFile = "token.json"

type authsrv struct {
	liveToken   *oauth2.Token
	tokenSource oauth2.TokenSource
	realms      *realms.Client
	log         *logrus.Entry
}

var Auth authsrv = authsrv{
	log: logrus.WithField("part", "Auth"),
}

// reads token from storage if there is one
func (a *authsrv) Startup() (err error) {
	a.liveToken, err = a.readToken()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	a.tokenSource = auth.RefreshTokenSource(a.liveToken)
	a.realms = realms.NewClient(a.tokenSource)
	_, err = a.TokenSource()
	if err != nil {
		return err
	}
	return nil
}

// if the user is currently logged in or not
func (a *authsrv) LoggedIn() bool {
	return a.tokenSource != nil
}

// performs microsoft login using the handler passed
func (a *authsrv) Login(ctx context.Context, handler auth.MSAuthHandler) (err error) {
	a.liveToken, err = auth.RequestLiveTokenWriter(ctx, handler)
	if err != nil {
		return err
	}
	err = a.writeToken(a.liveToken)
	if err != nil {
		return err
	}
	a.tokenSource = auth.RefreshTokenSource(a.liveToken)
	a.realms = realms.NewClient(a.tokenSource)
	return nil
}

func (a *authsrv) Logout() {
	a.liveToken = nil
	a.tokenSource = nil
	a.realms = nil
	os.Remove(TokenFile)
}

func (a *authsrv) refreshLiveToken() (err error) {
	if a.liveToken.Valid() {
		return nil
	}
	a.log.Info("Refreshing Microsoft Token")
	a.liveToken, err = a.tokenSource.Token()
	if err != nil {
		return err
	}
	err = a.writeToken(a.liveToken)
	if err != nil {
		return err
	}
	a.tokenSource = auth.RefreshTokenSource(a.liveToken)
	a.realms = realms.NewClient(a.tokenSource)
	return nil
}

// writes the livetoken to storage
func (a *authsrv) writeToken(token *oauth2.Token) error {
	f, err := os.Create(TokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	return e.Encode(token)
}

// reads the live token from storage, returns os.ErrNotExist if no token is stored
func (a *authsrv) readToken() (*oauth2.Token, error) {
	var token oauth2.Token
	f, err := os.Open(TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	e := json.NewDecoder(f)
	err = e.Decode(&token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

var ErrNotLoggedIn = errors.New("not logged in")

func (a *authsrv) TokenSource() (src oauth2.TokenSource, err error) {
	if a.tokenSource == nil {
		return nil, ErrNotLoggedIn
	}
	err = a.refreshLiveToken()
	if err != nil {
		return nil, err
	}
	return a.tokenSource, nil
}

func (a *authsrv) Realms() (*realms.Client, error) {
	if a.realms != nil {
		return a.realms, nil
	}
	return nil, ErrNotLoggedIn
}
