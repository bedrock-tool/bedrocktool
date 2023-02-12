package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const TokenFile = "token.json"

var gTokenSrc oauth2.TokenSource

func GetTokenSource() oauth2.TokenSource {
	if gTokenSrc != nil {
		return gTokenSrc
	}
	token := getToken()
	gTokenSrc = auth.RefreshTokenSource(&token)
	newToken, err := gTokenSrc.Token()
	if err != nil {
		panic(err)
	}
	if !token.Valid() {
		logrus.Info(locale.Loc("refreshed_token", nil))
		writeToken(newToken)
	}

	return gTokenSrc
}

var RealmsEnv string

var gRealmsAPI *realms.Client

func GetRealmsAPI() *realms.Client {
	if gRealmsAPI == nil {
		if RealmsEnv != "" {
			realms.RealmsAPIBase = fmt.Sprintf("https://pocket-%s.realms.minecraft.net/", RealmsEnv)
		}
		gRealmsAPI = realms.NewClient(GetTokenSource())
	}
	return gRealmsAPI
}

func writeToken(token *oauth2.Token) {
	buf, err := json.Marshal(token)
	if err != nil {
		panic(err)
	}
	os.WriteFile(TokenFile, buf, 0o755)
}

func getToken() oauth2.Token {
	var token oauth2.Token
	if _, err := os.Stat(TokenFile); err == nil {
		f, err := os.Open(TokenFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&token); err != nil {
			panic(err)
		}
	} else {
		_token, err := auth.RequestLiveToken()
		if err != nil {
			panic(err)
		}
		writeToken(_token)
		token = *_token
	}
	return token
}
