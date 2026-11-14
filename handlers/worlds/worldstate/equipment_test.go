package worldstate

import (
	"testing"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestWriteEquipment(t *testing.T) {
	world.InsertCustomItems([]protocol.ItemEntry{{Name: "hivebackbling:test_wings", RuntimeID: 31000}})
	w := newTestWorld(t)
	w.BlockRegistry = world.DefaultBlockRegistry

	wings := protocol.ItemInstance{Stack: protocol.ItemStack{ItemType: protocol.ItemType{NetworkID: 31000}, Count: 1}}
	ent := &entity.Entity{Chestplate: &wings, MainHand: &wings}
	data := map[string]any{}
	w.writeEquipment(ent, data)

	armor, ok := data["Armor"].([]map[string]any)
	if !ok || len(armor) != 4 {
		t.Fatalf("Armor = %#v", data["Armor"])
	}
	if armor[1]["Name"] != "hivebackbling:test_wings" || armor[1]["Count"] != byte(1) {
		t.Errorf("chest slot = %v", armor[1])
	}
	if armor[0]["Name"] != "" || armor[0]["Count"] != byte(0) {
		t.Errorf("empty head slot = %v", armor[0])
	}
	if hand := data["Mainhand"].([]map[string]any); hand[0]["Name"] != "hivebackbling:test_wings" {
		t.Errorf("main hand = %v", hand[0])
	}

	// nothing worn: nothing written
	bare := map[string]any{}
	w.writeEquipment(&entity.Entity{}, bare)
	if len(bare) != 0 {
		t.Errorf("wrote %v for an entity with nothing on", bare)
	}
}
