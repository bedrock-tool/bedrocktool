package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

func get_realm(ctx context.Context, api *realms.Client, realm_name, id string) (name string, address string, err error) {
	realms, err := api.Realms(ctx)
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
func (*RealmListCMD) Synopsis() string { return "prints all realms you have access to" }

func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *RealmListCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	api := realms.NewClient(GetTokenSource())
	realms, err := api.Realms(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, realm := range realms {
		fmt.Printf("Name: %s\tid: %d\n", realm.Name, realm.ID)
	}
	return 0
}

func init() {
	register_command(&RealmListCMD{})
}
