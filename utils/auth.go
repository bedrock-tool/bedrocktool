package utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	if err != nil {
		return err
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
		return Ver1token(f)
	}

	return nil, errors.New("unsupported token file")
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
