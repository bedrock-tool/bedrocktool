package utils

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
)

func getRealm(ctx context.Context, realmName, id string) (name string, address string, err error) {
	realms, err := GetRealmsAPI().Realms(ctx)
	if err != nil {
		return "", "", err
	}
	for _, realm := range realms {
		if strings.HasPrefix(realm.Name, realmName) {
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

func (*RealmListCMD) Name() string               { return "list-realms" }
func (*RealmListCMD) Synopsis() string           { return locale.Loc("list_realms_synopsis", nil) }
func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Execute(ctx context.Context, ui UI) error {
	realms, err := GetRealmsAPI().Realms(ctx)
	if err != nil {
		return err
	}
	for _, realm := range realms {
		fmt.Println(locale.Loc("realm_list_line", locale.Strmap{"Name": realm.Name, "Id": realm.ID}))
	}
	return nil
}

func init() {
	RegisterCommand(&RealmListCMD{})
}
