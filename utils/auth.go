package utils

import (
	"encoding/json"
	"os"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const TOKEN_FILE = "token.json"

var G_token_src oauth2.TokenSource

func GetTokenSource() oauth2.TokenSource {
	if G_token_src != nil {
		return G_token_src
	}
	token := get_token()
	G_token_src = auth.RefreshTokenSource(&token)
	new_token, err := G_token_src.Token()
	if err != nil {
		panic(err)
	}
	if !token.Valid() {
		logrus.Info("Refreshed token")
		write_token(new_token)
	}

	return G_token_src
}

var G_realms_api *realms.Client

func GetRealmsApi() *realms.Client {
	if G_realms_api == nil {
		G_realms_api = realms.NewClient(GetTokenSource())
	}
	return G_realms_api
}

func write_token(token *oauth2.Token) {
	buf, err := json.Marshal(token)
	if err != nil {
		panic(err)
	}
	os.WriteFile(TOKEN_FILE, buf, 0o755)
}

func get_token() oauth2.Token {
	var token oauth2.Token
	if _, err := os.Stat(TOKEN_FILE); err == nil {
		f, err := os.Open(TOKEN_FILE)
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
		write_token(_token)
		token = *_token
	}
	return token
}
