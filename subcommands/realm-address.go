package subcommands

import (
	"context"

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
	connectInfo := connectinfo.ConnectInfo{Value: "realm:" + realmSettings.Realm}
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
