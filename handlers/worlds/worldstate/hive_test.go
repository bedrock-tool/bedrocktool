package worldstate

import (
	"encoding/json"
	"testing"
)

// hive:text matches Hive's own text.json (as the Halloween hub ships it).
func TestHiveTextDefinition(t *testing.T) {
	if HiveTextFormatVersion != "1.18.20" {
		t.Errorf("format %s", HiveTextFormatVersion)
	}
	if len(HiveTextGroups) != 51 || len(HiveTextEvents) != 52 {
		t.Errorf("%d groups, %d events; want 51 and 52", len(HiveTextGroups), len(HiveTextEvents))
	}
	g2, _ := json.Marshal(HiveTextGroups["hive:2"])
	if string(g2) != `{"minecraft:custom_hit_test":{"hitboxes":[{"height":3.5,"pivot":[0,-1.5,0],"width":2}]},"minecraft:variant":{"value":2}}` {
		t.Errorf("hive:2 = %s", g2)
	}
	g8, _ := json.Marshal(HiveTextGroups["hive:8"])
	if string(g8) != `{"minecraft:custom_hit_test":{"hitboxes":[{"height":2.5,"pivot":[0,-1,0.7],"width":1.5}]},"minecraft:variant":{"value":8}}` {
		t.Errorf("hive:8 = %s", g8)
	}
	if _, ok := HiveTextGroups["hive:19"].(map[string]any)["minecraft:type_family"]; !ok {
		t.Error("hive:19 has no murder family")
	}
	if HiveTextComponents["minecraft:damage_sensor"].(map[string]any)["triggers"].(map[string]any)["deals_damage"] != false {
		t.Error("deals_damage should be false, as in Hive's 1.18.20 file")
	}
	ce, _ := json.Marshal(HiveTextClientEntity)
	if want := `{"format_version":"1.10.0","minecraft:client_entity":{"description":{"geometry":{"default":"geometry.item_sprite"},"identifier":"hive:text","materials":{"default":"player_spectator"},"render_controllers":["controller.render.default"],"textures":{"default":"textures/entity/cape_invisible"}}}}`; string(ce) != want {
		t.Errorf("client entity %s", ce)
	}
}
