package world

import (
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type itemContainer struct {
	OpenPacket *packet.ContainerOpen
	Content    *packet.InventoryContent
}

func (w *worldsHandler) processItemPacketsServer(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ContainerOpen:
		// add to open containers
		existing, ok := w.worldState.openItemContainers[pk.WindowID]
		if !ok {
			existing = &itemContainer{}
		}
		w.worldState.openItemContainers[pk.WindowID] = &itemContainer{
			OpenPacket: pk,
			Content:    existing.Content,
		}

	case *packet.InventoryContent:
		if pk.WindowID == 0x0 { // inventory
			w.serverState.playerInventory = pk.Content
		} else {
			// save content
			existing, ok := w.worldState.openItemContainers[byte(pk.WindowID)]
			if ok {
				existing.Content = pk
			}
		}

	case *packet.InventorySlot:
		if pk.WindowID == 0x0 {
			if w.serverState.playerInventory == nil {
				w.serverState.playerInventory = make([]protocol.ItemInstance, 36)
			}
			w.serverState.playerInventory[pk.Slot] = pk.NewItem
		} else {
			// save content
			existing, ok := w.worldState.openItemContainers[byte(pk.WindowID)]
			if ok {
				existing.Content.Content[pk.Slot] = pk.NewItem
			}
		}

	case *packet.ItemStackResponse:

	case *packet.ContainerClose:
		// find container info
		existing, ok := w.worldState.openItemContainers[byte(pk.WindowID)]

		switch pk.WindowID {
		case protocol.WindowIDArmour: // todo handle
		case protocol.WindowIDOffHand: // todo handle
		case protocol.WindowIDUI:
		case protocol.WindowIDInventory: // todo handle
			if !ok {
				break
			}

		default:
			if !ok {
				logrus.Warn(locale.Loc("warn_window_closed_not_open", nil))
				break
			}

			if existing.Content == nil {
				break
			}

			pos := existing.OpenPacket.ContainerPosition
			cp := protocol.SubChunkPos{pos.X() << 4, pos.Z() << 4}

			// create inventory
			inv := inventory.New(len(existing.Content.Content), nil)
			for i, c := range existing.Content.Content {
				item := stackToItem(c.Stack)
				inv.SetItem(i, item)
			}

			// put into subchunk
			nbts := w.worldState.blockNBT[cp]
			for i, v := range nbts {
				NBTPos := protocol.BlockPos{v["x"].(int32), v["y"].(int32), v["z"].(int32)}
				if NBTPos == pos {
					w.worldState.blockNBT[cp][i]["Items"] = nbtconv.InvToNBT(inv)
					break
				}
			}

			w.proxy.SendMessage(locale.Loc("saved_block_inv", nil))

			// remove it again
			delete(w.worldState.openItemContainers, byte(pk.WindowID))
		}

	case *packet.ItemComponent:
		w.bp.ApplyComponentEntries(pk.Items)
	case *packet.MobArmourEquipment:
		if pk.EntityRuntimeID == w.proxy.Server.GameData().EntityRuntimeID {

		}
	}
	return pk
}

func (w *worldsHandler) processItemPacketsClient(pk packet.Packet, forward *bool) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ItemStackRequest:
		var requests []protocol.ItemStackRequest
		for _, isr := range pk.Requests {
			for _, sra := range isr.Actions {
				if sra, ok := sra.(*protocol.TakeStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DropStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.PlaceInContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.TakeOutContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == MapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
			}
			requests = append(requests, isr)
		}
		pk.Requests = requests
	case *packet.MobEquipment:
		if pk.NewItem.Stack.NBTData["map_uuid"] == int64(ViewMapID) {
			*forward = false
		}
	}
	return pk
}

// stackToItem converts a network ItemStack representation back to an item.Stack.
func stackToItem(it protocol.ItemStack) item.Stack {
	t, ok := world.ItemByRuntimeID(it.NetworkID, int16(it.MetadataValue))
	if !ok {
		t = block.Air{}
	}
	if it.BlockRuntimeID > 0 {
		// It shouldn't matter if it (for whatever reason) wasn't able to get the block runtime ID,
		// since on the next line, we assert that the block is an item. If it didn't succeed, it'll
		// return air anyway.
		b, _ := world.BlockByRuntimeID(uint32(it.BlockRuntimeID))
		if t, ok = b.(world.Item); !ok {
			t = block.Air{}
		}
	}
	//noinspection SpellCheckingInspection
	if nbter, ok := t.(world.NBTer); ok && len(it.NBTData) != 0 {
		t = nbter.DecodeNBT(it.NBTData).(world.Item)
	}
	s := item.NewStack(t, int(it.Count))
	return nbtconv.ReadItem(it.NBTData, &s)
}

func (w *worldsHandler) playerData() (ret map[string]any) {
	ret = map[string]any{
		"format_version": "1.12.0",
		"identifier":     "minecraft:player",
	}

	if len(w.serverState.playerInventory) > 0 {
		inv := inventory.New(len(w.serverState.playerInventory), nil)
		for i, ii := range w.serverState.playerInventory {
			inv.SetItem(i, stackToItem(ii.Stack))
		}
		ret["Inventory"] = nbtconv.InvToNBT(inv)
	}

	ret["abilities"] = map[string]any{
		"doorsandswitches":       true,
		"op":                     true,
		"opencontainers":         true,
		"teleport":               true,
		"attackmobs":             true,
		"instabuild":             true,
		"permissionsLevel":       int32(3),
		"flying":                 false,
		"lightning":              false,
		"playerPermissionsLevel": int32(2),
		"attackplayers":          true,
		"build":                  true,
		"flySpeed":               float32(0.05),
		"invulnerable":           true,
		"mayfly":                 true,
		"mine":                   true,
		"walkSpeed":              float32(0.1),
	}

	type attribute struct {
		Name       string
		Base       float32
		Current    float32
		DefaultMax float32
		DefaultMin float32
		Max        float32
		Min        float32
	}

	ret["Attributes"] = []attribute{
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1024,
			DefaultMin: -1024,
			Max:        1024,
			Min:        -1024,
			Name:       "minecraft:luck",
		},
		{
			Base:       20,
			Current:    20,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:health",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 16,
			DefaultMin: 0,
			Max:        16,
			Min:        0,
			Name:       "minecraft:absorption",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1,
			DefaultMin: 0,
			Max:        1,
			Min:        0,
			Name:       "minecraft:knockback_resistance",
		},
		{
			Base:       0.1,
			Current:    0.1,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:movement",
		},
		{
			Base:       0.02,
			Current:    0.02,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:underwater_movement",
		},
		{
			Base:       0.02,
			Current:    0.02,
			DefaultMax: 3.4028235e+38,
			DefaultMin: 0,
			Max:        3.4028235e+38,
			Min:        0,
			Name:       "minecraft:lava_movement",
		},
		{
			Base:       16,
			Current:    16,
			DefaultMax: 2048,
			DefaultMin: 0,
			Max:        2048,
			Min:        0,
			Name:       "minecraft:follow_range",
		},
		{
			Base:       1,
			Current:    1,
			DefaultMax: 1,
			DefaultMin: 1,
			Max:        1,
			Min:        1,
			Name:       "minecraft:attack_damage",
		},
		{
			Base:       20,
			Current:    20,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.hunger",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.exhaustion",
		},
		{
			Base:       5,
			Current:    5,
			DefaultMax: 20,
			DefaultMin: 0,
			Max:        20,
			Min:        0,
			Name:       "minecraft:player.saturation",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 24791,
			DefaultMin: 0,
			Max:        24791,
			Min:        0,
			Name:       "minecraft:player.level",
		},
		{
			Base:       0,
			Current:    0,
			DefaultMax: 1,
			DefaultMin: 0,
			Max:        1,
			Min:        0,
			Name:       "minecraft:player.experience",
		},
	}

	ret["Tags"] = []string{}
	ret["OnGround"] = true

	spawn := w.proxy.Server.GameData().PlayerPosition

	ret["SpawnX"] = int32(spawn.X())
	ret["SpawnY"] = int32(spawn.Y())
	ret["SpawnZ"] = int32(spawn.Z())

	ret["Pos"] = []float32{
		float32(spawn.X()),
		float32(spawn.Y()),
		float32(spawn.Z()),
	}

	ret["Rotation"] = []float32{
		w.serverState.PlayerPos.Pitch,
		w.serverState.PlayerPos.Yaw,
	}

	return
}
