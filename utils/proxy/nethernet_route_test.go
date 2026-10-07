package proxy

import (
	"context"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
	"strings"
	"testing"
)

func TestRealmRegionCannotCreateInvalidHost(t *testing.T) {
	for _, region := range []string{"west.example.org", "foo/path", "../", "bad region", strings.Repeat("a", 57)} {
		if _, err := realmSignalingEnvironment(region); err == nil {
			t.Fatalf("accepted unsafe/invalid region %q", region)
		}
	}
	if env, err := realmSignalingEnvironment(""); env != nil || err != nil {
		t.Fatalf("missing region must use discovery: %v, %v", env, err)
	}
}

func TestXboxFriendRouteStillJoinsMPSDAndPreservesNonce(t *testing.T) {
	handle := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	target := connectinfo.Endpoint{Transport: connectinfo.TransportXboxFriend, Address: handle.String()}
	called := 0
	want := netherNetRoute{Address: "987654321", Nonce: "test-nonce", Type: p2p.ConnectionTypeSignalingOverWebSocket}
	got, err := resolveNetherNet(context.Background(), target, func(_ context.Context, id uuid.UUID) (netherNetRoute, error) {
		called++
		if id != handle {
			t.Fatalf("wrong Xbox handle: %s", id)
		}
		return want, nil
	})
	if err != nil || called != 1 || got != want {
		t.Fatalf("friend routing regression: got %+v, joins %d, error %v", got, called, err)
	}
}

func TestRealmJSONRPCRouteNeverJoinsXboxSession(t *testing.T) {
	target := connectinfo.Endpoint{Transport: connectinfo.TransportRealmJSONRPC, Address: "01234567-89ab-cdef-0123-456789abcdef", Region: "WestEurope"}
	route, err := resolveNetherNet(context.Background(), target, func(context.Context, uuid.UUID) (netherNetRoute, error) {
		t.Fatal("Realm ID sent to Xbox MPSD Join")
		return netherNetRoute{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Address != target.Address || route.Nonce != "" || route.Type != p2p.ConnectionTypeSignalingOverJSONRPC || route.Region != target.Region {
		t.Fatalf("wrong Realm route: %+v", route)
	}
	env, err := realmSignalingEnvironment(route.Region)
	if err != nil {
		t.Fatal(err)
	}
	if got := env.ServiceURI.String(); got != "https://signal-westeurope.franchise.minecraft-services.net" {
		t.Fatalf("wrong signaling endpoint: %s", got)
	}
}
