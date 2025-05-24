package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type ConnectInfo struct {
	Value string

	gathering *discovery.Gathering
	realm     *realms.Realm
}

func (c *ConnectInfo) getGathering(ctx context.Context, name string) (*discovery.Gathering, error) {
	if c.gathering != nil && c.gathering.Title == name {
		return c.gathering, nil
	}
	gatheringsService, err := Auth.Gatherings(ctx)
	if err != nil {
		return nil, err
	}
	gatherings, err := gatheringsService.Gatherings(ctx)
	if err != nil {
		return nil, err
	}
	for _, gathering := range gatherings {
		title := strings.ToLower(gathering.Title)
		id := strings.ToLower(gathering.GatheringID)
		if strings.HasPrefix(title, name) || strings.HasPrefix(id, name) {
			return gathering, nil
		}
	}
	return nil, fmt.Errorf("gathering %s not found", name)
}

func (c *ConnectInfo) getRealm(ctx context.Context, name string) (*realms.Realm, error) {
	if c.realm != nil && c.realm.Name == name {
		return c.realm, nil
	}
	realms, err := Auth.Realms().Realms(ctx)
	if err != nil {
		return nil, err
	}
	for _, realm := range realms {
		if strings.HasPrefix(strings.ToLower(realm.Name), strings.ToLower(name)) {
			return &realm, nil
		}
	}
	return nil, fmt.Errorf("realm %s not found", name)
}

func (c *ConnectInfo) Name(ctx context.Context) (string, error) {
	info, err := parseConnectInfo(c.Value)
	if err != nil {
		return "", nil
	}
	if info.serverAddress != "" {
		host, port, err := net.SplitHostPort(info.serverAddress)
		if err != nil {
			host = info.serverAddress
		} else if port != "19132" {
			host += "_" + port
		}

		return host, nil
	}
	if info.replayName != "" {
		return path.Base(info.replayName), nil
	}
	if info.realmName != "" {
		realm, err := c.getRealm(ctx, info.realmName)
		if err != nil {
			return "", err
		}
		return realm.Name, nil
	}
	if info.gatheringName != "" {
		gathering, err := c.getGathering(ctx, info.gatheringName)
		if err != nil {
			return "", err
		}
		return gathering.Title, nil
	}
	return "invalid", nil
}

func (c *ConnectInfo) Address(ctx context.Context) (string, error) {
	info, err := parseConnectInfo(c.Value)
	if err != nil {
		return "", err
	}
	if info.serverAddress != "" {
		return info.serverAddress, nil
	}
	if info.replayName != "" {
		return info.replayName, nil
	}
	if info.realmName != "" {
		realm, err := c.getRealm(ctx, info.realmName)
		if err != nil {
			return "", err
		}
		return realm.Address(ctx)
	}
	if info.gatheringName != "" {
		gathering, err := c.getGathering(ctx, info.gatheringName)
		if err != nil {
			return "", err
		}
		return gathering.Address(ctx)
	}
	return "", errors.New("invalid address")
}

func (c *ConnectInfo) IsReplay() bool {
	return pcapRegex.MatchString(c.Value)
}

func (c *ConnectInfo) SetRealm(realm *realms.Realm) {
	c.Value = "realm:" + realm.Name
	c.realm = realm
}

func (c *ConnectInfo) SetGathering(gathering *discovery.Gathering) {
	c.Value = "gathering:" + gathering.Title
	c.gathering = gathering
}

type parsedConnectInfo struct {
	gatheringName string
	realmName     string
	replayName    string
	serverAddress string
}

func parseConnectInfo(value string) (*parsedConnectInfo, error) {
	if gatheringRegex.MatchString(value) {
		p := regexGetParams(gatheringRegex, value)
		input := strings.ToLower(p["Title"])
		return &parsedConnectInfo{gatheringName: input}, nil
	}

	// realm
	if realmRegex.MatchString(value) {
		p := regexGetParams(realmRegex, value)
		input := strings.ToLower(p["Name"])
		return &parsedConnectInfo{realmName: input}, nil
	}

	// pcap replay
	if pcapRegex.MatchString(value) {
		p := regexGetParams(pcapRegex, value)
		input := p["Filename"]
		return &parsedConnectInfo{replayName: input}, nil
	}

	// normal server dns or ip
	serverAddress := value
	if len(strings.Split(serverAddress, ":")) == 1 {
		serverAddress += ":19132"
	}
	return &parsedConnectInfo{serverAddress: serverAddress}, nil
}
