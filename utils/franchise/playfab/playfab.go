package playfab

import (
	"context"
	"net/http"

	"github.com/bedrock-tool/bedrocktool/utils/franchise/internal"
)

type loginResult struct {
	SessionTicket string `json:"SessionTicket"`
}

func LoginWithXbox(ctx context.Context, titleid, token string) (string, error) {
	res, err := internal.DoRequest[loginResult](ctx, http.DefaultClient, "POST", "https://"+titleid+".playfabapi.com/Client/LoginWithXbox", map[string]any{
		"TitleId":       titleid,
		"CreateAccount": true,
		"XboxToken":     token,
	}, nil)
	if err != nil {
		return "", err
	}
	return res.SessionTicket, nil
}
