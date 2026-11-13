package worlds

import (
	"testing"

	"github.com/google/uuid"
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
