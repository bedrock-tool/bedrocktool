package subcommands

import (
	"context"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/sirupsen/logrus"
)

type RealmAddressSettings struct {
	Realm string `opt:"Realm Name" flag:"realm"`
}

type RealmAddressCMD struct{}

func (RealmAddressCMD) Name() string {
	return "realm-address"
}

func (RealmAddressCMD) Description() string {
	return "gets realms address"
}

func (RealmAddressCMD) Settings() any {
	return new(RealmAddressSettings)
}

func (c *RealmAddressCMD) Run(ctx context.Context, settings any) error {
	realmSettings := settings.(*RealmAddressSettings)
	// Ensure we're logged in and have an account attached to the ConnectInfo
	if !auth.Auth.LoggedIn() {
		if err := auth.Auth.Login(ctx, &xbox.DeviceTypeAndroid, ""); err != nil {
			return err
		}
	}
	if auth.Auth.Account() == nil {
		return fmt.Errorf("no account available after login; ensure the login flow completed")
	}
	connectInfo := connectinfo.ConnectInfo{Value: "realm:" + realmSettings.Realm, Account: auth.Auth.Account()}
	address, err := connectInfo.Address(ctx)
	if err != nil {
		return err
	}

	logrus.Infof("Address: %s", address)
	return nil
}

func init() {
	commands.RegisterCommand(&RealmAddressCMD{})
}
