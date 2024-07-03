package utils

import (
	"context"
	"net"

	"github.com/sandertv/go-raknet"
	"github.com/sandertv/gophertunnel/minecraft"
)

// RakNet is an implementation of a RakNet v10 Network.
type RakNetMTU struct {
	MaxMTU uint16
}

// DialContext ...
func (r RakNetMTU) DialContext(ctx context.Context, address string) (net.Conn, error) {
	return raknet.Dialer{
		FixupMaxMTU: r.MaxMTU,
	}.DialContext(ctx, address)
}

// PingContext ...
func (r RakNetMTU) PingContext(ctx context.Context, address string) (response []byte, err error) {
	return raknet.PingContext(ctx, address)
}

// Listen ...
func (r RakNetMTU) Listen(address string) (minecraft.NetworkListener, error) {
	return raknet.Listen(address)
}

// init registers the RakNet network.
func init() {
	minecraft.RegisterNetwork("raknet-1200", RakNetMTU{
		MaxMTU: 1200,
	})
}
