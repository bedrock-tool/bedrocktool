package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
	"github.com/sandertv/gophertunnel/minecraft/service/signaling"
	"github.com/sandertv/gophertunnel/minecraft/service/signaling/messaging"
)

func dialRealmSignaling(ctx context.Context, region string, dial func(context.Context, *signaling.Environment) (*messaging.Conn, error)) (*messaging.Conn, error) {
	env, err := realmSignalingEnvironment(region)
	if err != nil {
		return nil, err
	}
	conn, err := dial(ctx, env)
	var dnsErr *net.DNSError
	// Only a missing regional signaling host is eligible. Token-service DNS,
	// authentication and transient failures must not change credential origins.
	if err != nil && env != nil && ctx.Err() == nil && errors.As(err, &dnsErr) &&
		dnsErr.IsNotFound && !dnsErr.IsTimeout && !dnsErr.IsTemporary && dnsErr.Name == env.ServiceURI.Hostname() {
		return dial(ctx, nil) // The messaging dialer discovers the default service.
	}
	return conn, err
}

type netherNetRoute struct {
	Address string
	Nonce   string
	Type    int
	Region  string
}

// resolveNetherNet only joins MPSD for Xbox friend handles, never for Realm messaging IDs.
func resolveNetherNet(ctx context.Context, target connectinfo.Endpoint, join func(context.Context, uuid.UUID) (netherNetRoute, error)) (netherNetRoute, error) {
	id, err := uuid.Parse(target.Address)
	if err != nil {
		return netherNetRoute{}, fmt.Errorf("parsing nethernet address: %w", err)
	}
	switch target.Transport {
	case connectinfo.TransportRealmJSONRPC:
		return netherNetRoute{Address: target.Address, Type: p2p.ConnectionTypeSignalingOverJSONRPC, Region: target.Region}, nil
	case connectinfo.TransportXboxFriend:
		return join(ctx, id)
	default:
		return netherNetRoute{}, fmt.Errorf("invalid NetherNet transport %d", target.Transport)
	}
}

var signalingRegion = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,54}[a-z0-9])?$`)

// An absent region leaves discovery to the messaging dialer. Reject anything
// other than a DNS label before putting service credentials on the connection.
func realmSignalingEnvironment(region string) (*signaling.Environment, error) {
	if region == "" {
		return nil, nil
	}
	region = strings.ToLower(region)
	if !signalingRegion.MatchString(region) {
		return nil, fmt.Errorf("invalid Realm signaling region")
	}
	u := &url.URL{Scheme: "https", Host: "signal-" + region + ".franchise.minecraft-services.net"}
	return &signaling.Environment{ServiceURI: u}, nil
}
