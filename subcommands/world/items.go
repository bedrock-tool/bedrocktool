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

func (w *WorldState) processItemPacketsServer(pk packet.Packet) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ContainerOpen:
		if w.experimentInventory {
			// add to open containers
			existing, ok := w.openItemContainers[pk.WindowID]
			if !ok {
				existing = &itemContainer{}
			}
			w.openItemContainers[pk.WindowID] = &itemContainer{
				OpenPacket: pk,
				Content:    existing.Content,
			}
		}
	case *packet.InventoryContent:
		if w.experimentInventory {
			// save content
			existing, ok := w.openItemContainers[byte(pk.WindowID)]
			if !ok {
				if pk.WindowID == 0x0 { // inventory
					w.openItemContainers[byte(pk.WindowID)] = &itemContainer{
						Content: pk,
					}
				}
				break
			}
			existing.Content = pk
		}
	case *packet.ContainerClose:
		if w.experimentInventory {
			switch pk.WindowID {
			case protocol.WindowIDArmour: // todo handle
			case protocol.WindowIDOffHand: // todo handle
			case protocol.WindowIDUI:
			case protocol.WindowIDInventory: // todo handle
			default:
				// find container info
				existing, ok := w.openItemContainers[byte(pk.WindowID)]
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
				nbts := w.blockNBT[cp]
				for i, v := range nbts {
					NBTPos := protocol.BlockPos{v["x"].(int32), v["y"].(int32), v["z"].(int32)}
					if NBTPos == pos {
						w.blockNBT[cp][i]["Items"] = nbtconv.InvToNBT(inv)
					}
				}

				w.proxy.SendMessage(locale.Loc("saved_block_inv", nil))

				// remove it again
				delete(w.openItemContainers, byte(pk.WindowID))
			}
		}
	case *packet.ItemComponent:
		w.bp.ApplyComponentEntries(pk.Items)
	}
	return pk
}

func (w *WorldState) processItemPacketsClient(pk packet.Packet, forward *bool) packet.Packet {
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
