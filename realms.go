package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	G_xbl_token.SetAuthHeader(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (realm *Realm) Address() (string, error) {
	if G_debug {
		fmt.Printf("realm.Address()\n")
	}

	body, err := realms_get(fmt.Sprintf("worlds/%d/join", realm.Id))
	if err != nil {
		if strings.Contains(err.Error(), "503") {
			return "", fmt.Errorf("realm is starting")
		}
		return "", err
	}
	var data struct {
		Address       string `json:"address"`
		PendingUpdate bool   `json:"pendingUpdate"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	return data.Address, nil
}

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

func get_realm(realm_name string) (string, string, error) {
	// returns: name, address, err
	realms, err := get_realms()
	if err != nil {
		return "", "", err
	}
	for _, realm := range realms {
		if strings.HasPrefix(realm.Name, realm_name) {
			address, err := realm.Address()
			if err != nil {
				return "", "", err
			}
			return realm.Name, address, nil
		}
	}
	return "", "", fmt.Errorf("realm not found")
}

func list_realms() error {
	realms, err := get_realms()
	if err != nil {
		return err
	}
	for _, realm := range realms {
		fmt.Printf("%s\t\t(%d)\n", realm.Name, realm.Id)
	}
	return nil
}
