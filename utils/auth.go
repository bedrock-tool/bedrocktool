package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const TokenFile = "token.json"

type authsrv struct {
	src       oauth2.TokenSource
	Ctx       context.Context
	MSHandler auth.MSAuthHandler
}

var Auth authsrv

func (a *authsrv) HaveToken() bool {
	_, err := os.Stat(TokenFile)
	return err == nil
}

func (a *authsrv) writeToken(token *oauth2.Token) error {
	f, err := os.Create(TokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	return e.Encode(token)
}

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

func (a *authsrv) GetTokenSource() (src oauth2.TokenSource, err error) {
	if a.src != nil {
		return a.src, nil
	}
	var token *oauth2.Token
	if a.HaveToken() {
		// read the existing token
		token, err = a.readToken()
		if err != nil {
			return nil, err
		}
	} else {
		// request a new token
		token, err = auth.RequestLiveTokenWriter(a.Ctx, a.MSHandler)
		if err != nil {
			return nil, err
		}
		err := a.writeToken(token)
		if err != nil {
			return nil, err
		}
	}
	a.src = auth.RefreshTokenSource(token)

	// if the old token isnt valid save the new one
	if !token.Valid() {
		logrus.Debug("Refreshing token")
		token, err = a.src.Token()
		if err != nil {
			return nil, err
		}
		err = a.writeToken(token)
		if err != nil {
			return nil, err
		}
	}

	return a.src, nil
}

var RealmsEnv string

var realmsAPI *realms.Client

func GetRealmsAPI() *realms.Client {
	if realmsAPI == nil {
		if RealmsEnv != "" {
			realms.RealmsAPIBase = fmt.Sprintf("https://pocket-%s.realms.minecraft.net/", RealmsEnv)
		}
		src, err := Auth.GetTokenSource()
		if err != nil {
			logrus.Fatal(err)
		}
		realmsAPI = realms.NewClient(src)
	}
	return realmsAPI
}
