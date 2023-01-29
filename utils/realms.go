package utils

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

func get_realm(ctx context.Context, realm_name, id string) (name string, address string, err error) {
	realms, err := GetRealmsApi().Realms(ctx)
	if err != nil {
		return "", "", err
	}
	for _, realm := range realms {
		if strings.HasPrefix(realm.Name, realm_name) {
			if id != "" && id != fmt.Sprint(id) {
				continue
			}
			name = realm.Name
			address, err = realm.Address(ctx)
			if err != nil {
				return "", "", err
			}
			return
		}
	}
	return "", "", fmt.Errorf("realm not found")
}

type RealmListCMD struct{}

func (*RealmListCMD) Name() string     { return "list-realms" }
func (*RealmListCMD) Synopsis() string { return locale.Loc("list_realms_synopsis", nil) }

func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *RealmListCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	realms, err := GetRealmsApi().Realms(ctx)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	for _, realm := range realms {
		fmt.Println(locale.Loc("realm_list_line", locale.Strmap{"Name": realm.Name, "Id": realm.ID}))
	}
	return 0
}

func init() {
	RegisterCommand(&RealmListCMD{})
}
