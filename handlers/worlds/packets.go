package worlds

import (
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/gregwebs/go-recovery"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) getEntity(id worldstate.EntityRuntimeID) (*worldstate.EntityState, bool) {
	e, ok := w.currentWorld.GetEntity(id)
	return e, ok
}

func (w *worldsHandler) packetCB(_pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	select {
	case err := <-w.err:
		return nil, err
	default:
	}

	// general / startup
	switch pk := _pk.(type) {
	case *packet.RequestChunkRadius:
		pk.ChunkRadius = w.settings.ChunkRadius

	case *packet.ChunkRadiusUpdated:
		w.serverState.radius = pk.ChunkRadius
		pk.ChunkRadius = w.settings.ChunkRadius

	case *packet.SetTime:
		w.currentWorld.SetTime(timeReceived, int(pk.Time))

	case *packet.StartGame:
		if !w.serverState.haveStartGame {
			w.serverState.haveStartGame = true
			w.currentWorld.SetTime(timeReceived, int(pk.Time))
			w.serverState.useHashedRids = pk.UseBlockNetworkIDHashes

			world.InsertCustomItems(pk.Items)
			for _, ie := range pk.Items {
				w.bp.AddItem(ie)
			}
			if len(pk.Blocks) > 0 {
				logrus.Info(locale.Loc("using_customblocks", nil))
				for _, be := range pk.Blocks {
					w.bp.AddBlock(be)
				}
				// telling the chunk code what custom blocks there are so it can generate offsets
				w.blockStates = world.InsertCustomBlocks(pk.Blocks)
				w.customBlocks = pk.Blocks
			}

			w.serverState.WorldName = pk.WorldName
			if pk.WorldName != "" {
				w.currentWorld.Name = pk.WorldName
			}

			{ // check game version
				gv := strings.Split(pk.BaseGameVersion, ".")
				var err error
				if len(gv) > 1 {
					var ver int
					ver, err = strconv.Atoi(gv[1])
					w.serverState.useOldBiomes = ver < 18
				}
				if err != nil || len(gv) <= 1 {
					logrus.Info(locale.Loc("guessing_version", nil))
				}

				if w.serverState.useOldBiomes {
					logrus.Info(locale.Loc("using_under_118", nil))
					w.serverState.dimensions[0] = protocol.DimensionDefinition{
						Name:      "minecraft:overworld",
						Range:     [2]int32{0, 256},
						Generator: 1,
					}
				}
			}
			dim, _ := world.DimensionByID(int(pk.Dimension))
			w.currentWorld.SetDimension(dim)

			w.openWorldState(w.settings.StartPaused)
		}

	case *packet.DimensionData:
		for _, dd := range pk.Definitions {
			if dd.Name == "minecraft:overworld" {
				w.serverState.dimensions[0] = dd
			}
		}

	case *packet.ItemComponent:
		w.bp.ApplyComponentEntries(pk.Items)

	case *packet.BiomeDefinitionList:
		err := nbt.UnmarshalEncoding(pk.SerialisedBiomeDefinitions, &w.serverState.biomes, nbt.NetworkLittleEndian)
		if err != nil {
			logrus.Error(err)
		}
		for k, v := range w.serverState.biomes {
			_, ok := world.BiomeByName(k)
			if !ok {
				world.RegisterBiome(&customBiome{
					name: k,
					data: v.(map[string]any),
				})
			}
		}
		w.bp.AddBiomes(w.serverState.biomes)
	}

	_pk = w.itemPackets(_pk)
	_pk = w.mapPackets(_pk, toServer)
	w.playersPackets(_pk)
	w.chunkPackets(_pk)

	// entity
	if w.settings.SaveEntities {
		w.entityPackets(_pk)
	}

	return _pk, nil
}

func (w *worldsHandler) playersPackets(_pk packet.Packet) {
	switch pk := _pk.(type) {
	case *packet.AddPlayer:
		w.currentWorld.AddPlayer(pk)
	}
}

func (w *worldsHandler) entityPackets(_pk packet.Packet) {
	switch pk := _pk.(type) {
	case *packet.AddActor:
		w.currentWorld.ProcessAddActor(pk, func(es *worldstate.EntityState) bool {
			var ignore bool
			if w.scripting.CB.OnEntityAdd != nil {
				err := recovery.Call(func() error {
					ignore = w.scripting.OnEntityAdd(es, es.Metadata)
					return nil
				})
				if err != nil {
					logrus.Errorf("scripting: %s", err)
				}
			}
			return ignore
		}, w.bp.AddEntity)

	case *packet.SetActorData:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			metadata := make(protocol.EntityMetadata)
			maps.Copy(metadata, pk.EntityMetadata)
			if w.scripting.CB.OnEntityDataUpdate != nil {
				err := recovery.Call(func() error {
					w.scripting.OnEntityDataUpdate(e, metadata)
					return nil
				})
				if err != nil {
					logrus.Errorf("Scripting %s", err)
				}
			}

			maps.Copy(e.Metadata, metadata)
			w.bp.AddEntity(behaviourpack.EntityIn{
				Identifier: e.EntityType,
				Attr:       nil,
				Meta:       metadata,
			})
		}

	case *packet.SetActorMotion:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			e.Velocity = pk.Velocity
		}

	case *packet.MoveActorDelta:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			if pk.Flags&packet.MoveActorDeltaFlagHasX != 0 {
				e.Position[0] = pk.Position[0]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasY != 0 {
				e.Position[1] = pk.Position[1]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasZ != 0 {
				e.Position[2] = pk.Position[2]
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasRotX != 0 {
				e.Pitch = pk.Rotation.X()
			}
			if pk.Flags&packet.MoveActorDeltaFlagHasRotY != 0 {
				e.Yaw = pk.Rotation.Y()
			}
		}

	case *packet.MoveActorAbsolute:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			e.Position = pk.Position
			e.Pitch = pk.Rotation.X()
			e.Yaw = pk.Rotation.Y()
		}

	case *packet.MobEquipment:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			w, ok := e.Inventory[pk.WindowID]
			if !ok {
				w = make(map[byte]protocol.ItemInstance)
				e.Inventory[pk.WindowID] = w
			}
			w[pk.HotBarSlot] = pk.NewItem
		}

	case *packet.MobArmourEquipment:
		e, ok := w.getEntity(pk.EntityRuntimeID)
		if ok {
			e.Helmet = &pk.Helmet
			e.Chestplate = &pk.Chestplate
			e.Leggings = &pk.Chestplate
			e.Boots = &pk.Boots
		}

	case *packet.SetActorLink:
		w.currentWorld.AddEntityLink(pk.EntityLink)
	}
}

func (w *worldsHandler) chunkPackets(_pk packet.Packet) {
	// chunk
	switch pk := _pk.(type) {
	case *packet.ChangeDimension:
		w.processChangeDimension(pk)

	case *packet.LevelChunk:
		w.processLevelChunk(pk)

	case *packet.SubChunk:
		if err := w.processSubChunk(pk); err != nil {
			logrus.Error(err)
		}

	case *packet.BlockActorData:
		p := pk.Position
		pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
		w.currentWorld.SetBlockNBT(pos, pk.NBTData, false)
		/*
			case *packet.UpdateBlock:
				if w.settings.BlockUpdates {
					cp := world.ChunkPos{pk.Position.X() >> 4, pk.Position.Z() >> 4}
					c, ok := w.worldState.state().chunks[cp]
					if ok {
						x, y, z := blockPosInChunk(pk.Position)
						c.SetBlock(x, y, z, uint8(pk.Layer), pk.NewBlockRuntimeID)
						w.mapUI.SetChunk(cp, c, w.worldState.useDeferred)
					}
				}
			case *packet.UpdateSubChunkBlocks:
				if w.settings.BlockUpdates {
					cp := world.ChunkPos{pk.Position.X(), pk.Position.Z()}
					c, ok := w.worldState.state().chunks[cp]
					if ok {
						for _, bce := range pk.Blocks {
							x, y, z := blockPosInChunk(bce.BlockPos)
							if bce.SyncedUpdateType == packet.BlockToEntityTransition {
								c.SetBlock(x, y, z, 0, world.AirRID())
							} else {
								c.SetBlock(x, y, z, 0, bce.BlockRuntimeID)
							}
						}
						w.mapUI.SetChunk(cp, c, w.worldState.useDeferred)
					}
				}
		*/
	}
}

func (w *worldsHandler) mapPackets(_pk packet.Packet, toServer bool) packet.Packet {
	// map
	switch pk := _pk.(type) {
	case *packet.MapInfoRequest:
		if pk.MapID == ViewMapID {
			w.mapUI.SchedRedraw()
			_pk = nil
		}
	case *packet.Animate:
		if toServer && pk.ActionType == packet.AnimateActionSwingArm {
			w.mapUI.ChangeZoom()
			w.proxy.SendPopup(locale.Loc("zoom_level", locale.Strmap{"Level": w.mapUI.zoomLevel}))
		}
	}
	return _pk
}

func (w *worldsHandler) itemPackets(_pk packet.Packet) packet.Packet {
	// items
	switch pk := _pk.(type) {
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
			_pk = nil
		}

	case *packet.ContainerOpen:
		// add to open containers
		existing, ok := w.serverState.openItemContainers[pk.WindowID]
		if !ok {
			existing = &itemContainer{}
		}
		w.serverState.openItemContainers[pk.WindowID] = &itemContainer{
			OpenPacket: pk,
			Content:    existing.Content,
		}

	case *packet.InventoryContent:
		if pk.WindowID == 0x0 { // inventory
			w.serverState.playerInventory = pk.Content
		} else {
			// save content
			existing, ok := w.serverState.openItemContainers[byte(pk.WindowID)]
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
			existing, ok := w.serverState.openItemContainers[byte(pk.WindowID)]
			if ok {
				existing.Content.Content[pk.Slot] = pk.NewItem
			}
		}

	case *packet.ContainerClose:
		// find container info
		existing, ok := w.serverState.openItemContainers[byte(pk.WindowID)]

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

			// create inventory
			inv := inventory.New(len(existing.Content.Content), nil)
			for i, c := range existing.Content.Content {
				item := utils.StackToItem(c.Stack)
				inv.SetItem(i, item)
			}

			// put into subchunk
			p := existing.OpenPacket.ContainerPosition
			pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
			w.currentWorld.SetBlockNBT(pos, map[string]any{
				"Items": nbtconv.InvToNBT(inv),
			}, true)

			w.proxy.SendMessage(locale.Loc("saved_block_inv", nil))

			// remove it again
			delete(w.serverState.openItemContainers, byte(pk.WindowID))
		}
	}
	return _pk
}
