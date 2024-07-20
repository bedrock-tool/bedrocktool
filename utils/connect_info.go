package utils

import (
	"context"
	"errors"
	"net"
	"path"

	"github.com/bedrock-tool/bedrocktool/utils/gatherings"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type ConnectInfo struct {
	Gathering     *gatherings.Gathering
	Realm         *realms.Realm
	Replay        string
	ServerAddress string
}

func (c *ConnectInfo) Name() string {
	if c.ServerAddress != "" {
		host, port, err := net.SplitHostPort(c.ServerAddress)
		if err != nil {
			host = c.ServerAddress
		} else if port != "19132" {
			host += "_" + port
		}

		return host
	}
	if c.Replay != "" {
		return path.Base(c.Replay)
	}
	if c.Realm != nil {
		return c.Realm.Name
	}
	if c.Gathering != nil {
		return c.Gathering.Title
	}
	return "invalid"
}

func (c *ConnectInfo) Address(ctx context.Context) (string, error) {
	if c.ServerAddress != "" {
		return c.ServerAddress, nil
	}
	if c.Replay != "" {
		return "PCAP!" + c.Replay, nil
	}
	if c.Realm != nil {
		return c.Realm.Address(ctx)
	}
	if c.Gathering != nil {
		return c.Gathering.Address(ctx)
	}
	return "", errors.New("invalid address")
}

type key int

var ConnectInfoKey key
