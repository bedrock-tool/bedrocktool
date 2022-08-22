package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/subcommands"
)

type Realm struct {
	Id    int    `json:"id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Motd  string `json:"motd"`
	State string `json:"state"`
}

func realms_get(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://pocket.realms.minecraft.net/%s", path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MCPE/UWP")
	req.Header.Set("Client-Version", "1.10.1")
	GetXBLToken(context.Background()).SetAuthHeader(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (realm *Realm) Address() (string, error) {
	var body []byte
	var err error
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	i := 0
	for range ticker.C {
		i++
		body, err = realms_get(fmt.Sprintf("worlds/%d/join", realm.Id))
		if err != nil {
			if err.Error() == "HTTP 503" {
				fmt.Printf("Waiting for the realm to start... %d\033[K\r", i)
				continue
			}
			return "", err
		}
		println()

		var data struct {
			Address       string `json:"address"`
			PendingUpdate bool   `json:"pendingUpdate"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", err
		}
		return data.Address, nil
	}
	panic("unreachable")
}

// get_realms lists all realms the user has access to
func get_realms() ([]Realm, error) {
	data, err := realms_get("worlds")
	if err != nil {
		return nil, err
	}

	var realms struct {
		Servers []Realm `json:"servers"`
	}
	if err := json.Unmarshal(data, &realms); err != nil {
		return nil, err
	}

	return realms.Servers, nil
}

func get_realm(realm_name, id string) (name string, address string, err error) {
	realms, err := get_realms()
	if err != nil {
		return "", "", err
	}
	for _, realm := range realms {
		if strings.HasPrefix(realm.Name, realm_name) {
			if id != "" && id != fmt.Sprint(id) {
				continue
			}
			name = realm.Name
			address, err = realm.Address()
			if err != nil {
				return "", "", err
			}
			return
		}
	}
	return "", "", fmt.Errorf("realm not found")
}

type TokenCMD struct{}

func (*TokenCMD) Name() string     { return "realms-token" }
func (*TokenCMD) Synopsis() string { return "print xbl3.0 token for realms api" }

func (c *TokenCMD) SetFlags(f *flag.FlagSet) {}
func (c *TokenCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *TokenCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fmt.Printf("%s\n", GetXBLToken(ctx).AuthorizationToken.Token)
	return 0
}

type RealmListCMD struct{}

func (*RealmListCMD) Name() string     { return "list-realms" }
func (*RealmListCMD) Synopsis() string { return "prints all realms you have access to" }

func (c *RealmListCMD) SetFlags(f *flag.FlagSet) {}
func (c *RealmListCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *RealmListCMD) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	realms, err := get_realms()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, realm := range realms {
		fmt.Printf("Name: %s\tid: %d\n", realm.Name, realm.Id)
	}
	return 0
}

func init() {
	register_command(&TokenCMD{})
	register_command(&RealmListCMD{})
}
