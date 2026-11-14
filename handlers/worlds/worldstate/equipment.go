package worldstate

import (
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// emptySlot is how the game saves an empty equipment slot.
func emptySlot() map[string]any {
	return map[string]any{"Name": "", "Count": byte(0), "Damage": int16(0), "WasPickedUp": byte(0)}
}

// itemNBT is an item as the game saves it, or an empty slot.
func (w *World) itemNBT(it *protocol.ItemInstance) map[string]any {
	if it == nil || it.Stack.NetworkID == 0 || it.Stack.Count == 0 || w.BlockRegistry == nil {
		return emptySlot()
	}
	stack := utils.StackToItem(w.BlockRegistry, it.Stack)
	if stack.Empty() {
		return emptySlot()
	}
	return nbtconv.WriteItem(stack, true)
}

// writeEquipment saves what an entity wears and holds: Armor (head, chest,
// legs, feet), Mainhand, Offhand and Body, in the lists the game loads them
// from. Entities with nothing on are left as they were.
func (w *World) writeEquipment(ent *entity.Entity, data map[string]any) {
	slots := []*protocol.ItemInstance{ent.Helmet, ent.Chestplate, ent.Leggings, ent.Boots, ent.MainHand, ent.OffHand, ent.Body}
	worn := false
	for _, s := range slots {
		if s != nil && s.Stack.NetworkID != 0 {
			worn = true
			break
		}
	}
	if !worn {
		return
	}
	data["Armor"] = []map[string]any{w.itemNBT(ent.Helmet), w.itemNBT(ent.Chestplate), w.itemNBT(ent.Leggings), w.itemNBT(ent.Boots)}
	data["Mainhand"] = []map[string]any{w.itemNBT(ent.MainHand)}
	data["Offhand"] = []map[string]any{w.itemNBT(ent.OffHand)}
	data["Body"] = []map[string]any{w.itemNBT(ent.Body)}
}
