package connectinfo

import (
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"testing"
)

func TestRealmJSONRPCEndpointPreservesRemoteIDAndRegion(t *testing.T) {
	a := realms.RealmAddress{Address: "01234567-89ab-cdef-0123-456789abcdef", NetworkProtocol: realms.NetworkProtocolNetherNetJSONRPC}
	a.SessionRegionData.RegionName = "WestEurope"
	got, err := realmEndpoint(a)
	if err != nil {
		t.Fatal(err)
	}
	if got.Transport != TransportRealmJSONRPC || got.Address != a.Address || got.Region != "WestEurope" {
		t.Fatalf("lost Realm routing metadata: %+v", got)
	}
}
