package connectinfo

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

// Transport keeps Realm messaging IDs distinct from Xbox friend-session handles.
type Transport uint8

const (
	TransportRakNet Transport = iota
	TransportXboxFriend
	TransportRealmJSONRPC
)

type Endpoint struct {
	Address   string
	Transport Transport
	Region    string
}

// Resolve retains the protocol and region from the Realm join response.
func (c *ConnectInfo) Resolve(ctx context.Context) (Endpoint, error) {
	info, err := parseConnectInfo(c.Value)
	if err != nil {
		return Endpoint{}, err
	}
	if info.realmName != "" {
		realm, err := c.getRealm(ctx, info.realmName)
		if err != nil {
			return Endpoint{}, err
		}
		address, err := realm.Address(ctx)
		if err != nil {
			return Endpoint{}, err
		}
		return realmEndpoint(address)
	}
	address, err := c.resolveAddress(ctx)
	if err != nil {
		return Endpoint{}, err
	}
	target := Endpoint{Address: address}
	if strings.HasPrefix(address, "nethernet:") {
		target.Transport = TransportXboxFriend
		target.Address = strings.TrimPrefix(address, "nethernet:")
	}
	return target, nil
}

func realmEndpoint(address realms.RealmAddress) (Endpoint, error) {
	switch realms.ParseNetworkProtocol(string(address.NetworkProtocol)) {
	case realms.NetworkProtocolNetherNetJSONRPC:
		if _, err := uuid.Parse(address.Address); err != nil {
			return Endpoint{}, fmt.Errorf("invalid Realm messaging ID: %w", err)
		}
		return Endpoint{Address: address.Address, Transport: TransportRealmJSONRPC, Region: address.SessionRegionData.RegionName}, nil
	case "", realms.NetworkProtocolDefault:
		if _, _, err := net.SplitHostPort(address.Address); err != nil {
			return Endpoint{}, fmt.Errorf("invalid RakNet Realm address: %w", err)
		}
		return Endpoint{Address: address.Address, Transport: TransportRakNet}, nil
	default:
		return Endpoint{}, fmt.Errorf("unsupported Realm network protocol %q", address.NetworkProtocol)
	}
}
