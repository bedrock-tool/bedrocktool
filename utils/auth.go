package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"golang.org/x/oauth2"
)

const TokenFile = "token.json"

type authsrv struct {
	t   *oauth2.Token
	src oauth2.TokenSource

	LoginWithMicrosoftCallback func(io.Reader)
}

var Auth authsrv

func (a *authsrv) HaveToken() bool {
	_, err := os.Stat(TokenFile)
	return err == nil
}

func (a *authsrv) Refresh() error {
	a.src = auth.RefreshTokenSource(a.t)
	return nil
}

func (a *authsrv) writeToken() error {
	f, err := os.Create(TokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	return e.Encode(a.t)
}

func (a *authsrv) readToken() error {
	var token oauth2.Token
	f, err := os.Open(TokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewDecoder(f)
	err = e.Decode(&token)
	if err != nil {
		return err
	}
	a.t = &token
	return nil
}

func (a *authsrv) GetTokenSource() (src oauth2.TokenSource, err error) {
	if a.src != nil {
		return a.src, nil
	}
	if !a.HaveToken() {
		// request a new token
		r, w := io.Pipe()
		go a.LoginWithMicrosoftCallback(r)
		a.t, err = auth.RequestLiveTokenWriter(w)
		if err != nil {
			return nil, err
		}
		err := a.writeToken()
		if err != nil {
			return nil, err
		}
	} else {
		// read the existing token
		err := a.readToken()
		if err != nil {
			return nil, err
		}
	}
	// refresh the token if necessary
	err = a.Refresh()
	if err != nil {
		return nil, err
	}
	// if the old token isnt valid save the new one
	if !a.t.Valid() {
		newToken, err := a.src.Token()
		if err != nil {
			return nil, err
		}
		a.t = newToken
		err = a.writeToken()
		if err != nil {
			return nil, err
		}
	}
	return a.src, nil
}

var RealmsEnv string

var gRealmsAPI *realms.Client

func GetRealmsAPI() *realms.Client {
	if gRealmsAPI == nil {
		if RealmsEnv != "" {
			realms.RealmsAPIBase = fmt.Sprintf("https://pocket-%s.realms.minecraft.net/", RealmsEnv)
		}
		gRealmsAPI = realms.NewClient(Auth.src)
	}
	return gRealmsAPI
}
