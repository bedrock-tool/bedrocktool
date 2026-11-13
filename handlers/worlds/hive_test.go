package worlds

import (
	"testing"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

func TestIsHiveServer(t *testing.T) {
	for _, c := range []struct {
		addr, name string
		want       bool
	}{
		{"ca.hivebedrock.network:19132", "", true},
		{"geo.hivebedrock.network", "", true},
		{"play.playhive.com:19132", "", true},
		{"", "§eThe Hive", true},
		{"play.cubecraft.net", "CubeCraft", false},
		{"", "Beehive Builders", false},
	} {
		if got := isHiveServer(c.addr, c.name); got != c.want {
			t.Errorf("isHiveServer(%q, %q) = %v", c.addr, c.name, got)
		}
	}
}

type namedPack struct {
	resource.Pack
	name string
}

func (p namedPack) Name() string { return p.name }

func TestHasHivePacks(t *testing.T) {
	if !hasHivePacks([]resource.Pack{namedPack{name: "Some Pack"}, namedPack{name: "§eThe Hive: Hub"}}) {
		t.Error("The Hive: Hub not recognised")
	}
	if !hasHivePacks([]resource.Pack{namedPack{name: "HiveCommon"}}) {
		t.Error("HiveCommon not recognised")
	}
	if hasHivePacks([]resource.Pack{namedPack{name: "Beehive Builders"}, namedPack{name: "CubeCraft"}}) {
		t.Error("non-Hive packs taken for Hive")
	}
}

func TestIsNPCPlayer(t *testing.T) {
	npc, zero, real, unlisted := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	xuids := map[uuid.UUID]string{npc: "", zero: "0", real: "2535412345678901"}
	if !isNPCPlayer(xuids, npc) || !isNPCPlayer(xuids, zero) {
		t.Error("XUID-less players are NPCs")
	}
	if isNPCPlayer(xuids, real) || isNPCPlayer(xuids, unlisted) {
		t.Error("real or unlisted players aren't NPCs")
	}
}
