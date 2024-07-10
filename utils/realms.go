package utils

import (
	"context"
	"fmt"
	"strings"
)

func getRealm(ctx context.Context, realmName, id string) (name string, address string, err error) {
	realmsClient, err := Auth.Realms()
	if err != nil {
		return "", "", err
	}
	realms, err := realmsClient.Realms(ctx)
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
