package auth

import (
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"golang.org/x/oauth2"
)

type tokenInfo struct {
	*oauth2.Token
	DeviceType string
	MCToken    *discovery.MCToken
}

func (t *tokenInfo) LiveToken() *oauth2.Token {
	return t.Token
}

func (t *tokenInfo) XboxDeviceType() *xbox.DeviceType {
	switch t.DeviceType {
	case "Android":
		return &xbox.DeviceTypeAndroid
	case "iOS":
		return &xbox.DeviceTypeIOS
	case "Win32":
		return &xbox.DeviceTypeWindows
	case "Nintendo":
		return &xbox.DeviceTypeNintendo
	case "":
		return &xbox.DeviceTypeAndroid
	default:
		return nil
	}
}
