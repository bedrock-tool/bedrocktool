package worlds

import (
	"testing"

	"github.com/google/uuid"
)

func TestIsHiveGameVersion(t *testing.T) {
	for _, c := range []struct {
		version string
		want    bool
	}{
		{"Hive Games proxy", true},
		{"1.26.60 Hive Games proxy", true},
		{"1.26.60", false},
		{"", false},
	} {
		if got := isHiveGameVersion(c.version); got != c.want {
			t.Errorf("isHiveGameVersion(%q) = %v", c.version, got)
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
