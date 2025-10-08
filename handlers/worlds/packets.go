package worlds

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) packetHandlerPreLogin(_pk packet.Packet, timeReceived time.Time) (packet.Packet, error) {
	switch pk := _pk.(type) {
	case *packet.GameRulesChanged:
		var haveGameRule = false
		for i, gameRule := range pk.GameRules {
			if gameRule.Name == "showCoordinates" {
				haveGameRule = true
				pk.GameRules[i] = protocol.GameRule{
					Name:  "showCoordinates",
					Value: true,
				}
				break
			}
		}
		if !haveGameRule {
			pk.GameRules = append(pk.GameRules, protocol.GameRule{
				Name:  "showCoordinates",
				Value: true,
			})
		}
	case *packet.StartGame:
		if !w.serverState.haveStartGame {
			w.serverState.haveStartGame = true
			w.serverState.useHashedRids = pk.UseBlockNetworkIDHashes

			var haveGameRule = false
			for i, gameRule := range pk.GameRules {
				if gameRule.Name == "showCoordinates" {
					haveGameRule = true
					gameRule.Value = true
					pk.GameRules[i] = gameRule
					break
				}
			}
			if !haveGameRule {
				pk.GameRules = append(pk.GameRules, protocol.GameRule{
					Name:  "showCoordinates",
					Value: true,
				})
			}

			w.serverState.blocks = world.DefaultBlockRegistry.Clone().(*world.BlockRegistryImpl)
			if len(pk.Blocks) > 0 {
				w.log.Info(locale.Loc("using_customblocks", nil))
				for _, be := range pk.Blocks {
					w.serverState.behaviorPack.AddBlock(be)
				}
				err := world.AddCustomBlocks(w.serverState.blocks, pk.Blocks)
				if err != nil {
					return nil, err
				}
				w.serverState.customBlocks = pk.Blocks
			}
			w.serverState.blocks.Finalize()
			w.serverState.worldName = pk.WorldName

			dim, _ := world.DimensionByID(int(pk.Dimension))
			w.worldStateMu.Lock()
			if pk.WorldName != "" {
				w.worldState.Name = pk.WorldName
			}
			w.worldState.SetDimension(dim)
			w.worldState.SetTime(timeReceived, int(pk.Time))
			w.openWorldState(w.settings.StartPaused)
			w.worldStateMu.Unlock()
		}

	case *packet.DimensionData:
		for _, dd := range pk.Definitions {
			if dd.Name == "minecraft:overworld" {
				w.serverState.dimensions[0] = dd
			}
		}

	case *packet.ItemRegistry:
		world.InsertCustomItems(pk.Items)
		for _, ie := range pk.Items {
			w.serverState.behaviorPack.AddItem(ie)
		}
		w.serverState.behaviorPack.ApplyComponentEntries(pk.Items)

	case *packet.BiomeDefinitionList:
		for _, biome := range pk.BiomeDefinitions {
			biomeName := pk.StringList[biome.NameIndex]
			sp := strings.SplitN(biomeName, ":", 2)
			if len(sp) > 1 {
				ns := sp[0]
				if ns == "minecraft" {
					biomeName = sp[1]
				}
			}
			_, ok := w.serverState.biomes.BiomeByName(biomeName)
			if !ok {
				w.serverState.biomes.Register(&customBiome{name: biomeName, biome: biome, pk: pk})
				w.serverState.behaviorPack.AddBiome(biomeName, biome)
			}
		}

	}

	return _pk, nil
}

func (w *worldsHandler) packetHandlerIngame(_pk packet.Packet, toServer bool, timeReceived time.Time) (packet.Packet, error) {
	switch pk := _pk.(type) {
	case *packet.RequestChunkRadius:
		pk.ChunkRadius = w.settings.ChunkRadius
		//pk.MaxChunkRadius = w.settings.ChunkRadius

	case *packet.ChunkRadiusUpdated:
		w.serverState.realChunkRadius = pk.ChunkRadius
		pk.ChunkRadius = w.settings.ChunkRadius

	case *packet.SetCommandsEnabled:
		pk.Enabled = true

	case *packet.SetTime:
		w.currentWorld(func(world *worldstate.World) {
			world.SetTime(timeReceived, int(pk.Time))
		})

	// chunk

	case *packet.ChangeDimension:
		dim, _ := world.DimensionByID(int(pk.Dimension))
		w.SaveAndReset(false, dim)

	case *packet.LevelChunk:
		err := w.handleLevelChunk(pk, timeReceived)
		if err != nil {
			w.log.Errorf("processLevelChunk %s", err)
		}

	case *packet.SubChunk:
		if err := w.processSubChunk(pk); err != nil {
			w.log.WithField("packet", "SubChunk").Error(err)
		}

	case *packet.BlockActorData:
		p := pk.Position
		pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
		w.currentWorld(func(world *worldstate.World) {
			world.SetBlockNBT(pos, pk.NBTData, false)
		})

	case *packet.ClientBoundMapItemData:
		w.currentWorld(func(world *worldstate.World) {
			world.StoreMap(pk)
		})

	case *packet.MapInfoRequest:
		if pk.MapID == ViewMapID {
			w.mapUI.SchedRedraw()
			_pk = nil
		}
	case *packet.Animate:
		if toServer && pk.ActionType == packet.AnimateActionSwingArm {
			w.mapUI.ChangeZoom()
			w.session.SendPopup(locale.Loc("zoom_level", locale.Strmap{"Level": w.mapUI.zoomLevel}))
		}

	case *packet.SpawnParticleEffect:
		w.scripting.OnSpawnParticle(pk.ParticleName, pk.Position, timeReceived)

	case *packet.AddPlayer:
		w.currentWorld(func(world *worldstate.World) {
			world.AddPlayer(pk)
		})

	case *packet.PlayerList:
		if pk.ActionType == packet.PlayerListActionAdd {
			for _, player := range pk.Entries {
				w.serverState.playerSkins[player.UUID] = &player.Skin
			}
		}

	case *packet.PlayerSkin:
		w.serverState.playerSkins[pk.UUID] = &pk.Skin

	case *packet.SyncActorProperty:
		w.syncActorProperty(pk)

	case *packet.AddActor:
		w.currentWorld(func(world *worldstate.World) {
			ent := world.GetEntity(pk.EntityRuntimeID)
			if ent == nil {
				ent = &entity.Entity{
					RuntimeID:  pk.EntityRuntimeID,
					UniqueID:   pk.EntityUniqueID,
					EntityType: pk.EntityType,
					Inventory:  make(map[byte]map[byte]protocol.ItemInstance),
					Metadata:   make(map[uint32]any),
					Properties: make(map[string]*entity.EntityProperty),
				}
			}
			ent.Position = pk.Position
			ent.Pitch = pk.Pitch
			ent.Yaw = pk.Yaw
			ent.HeadYaw = pk.HeadYaw
			ent.Velocity = pk.Velocity
			w.applyEntityData(ent, pk.EntityMetadata, pk.EntityProperties, timeReceived)

			if !w.scripting.OnEntityAdd(ent, timeReceived) {
				logrus.Infof("Ignoring Entity: %s %d", ent.EntityType, ent.UniqueID)
				return
			}
			world.StoreEntity(pk.EntityRuntimeID, ent)
			for _, el := range pk.EntityLinks {
				world.AddEntityLink(el)
			}
			properties := w.serverState.entityProperties[ent.EntityType]
			w.serverState.behaviorPack.AddEntity(pk.EntityType, pk.Attributes, ent.Metadata, properties)
		})

		/*
			case *packet.RemoveActor:
				entity := w.currentWorld.GetEntityUniqueID(pk.EntityUniqueID)
				if entity != nil {
						dist := entity.Position.Vec2().Sub(playerPos.Vec2()).Len()

						fmt.Fprintf(distf, "%.5f\t%s\n", dist, entity.EntityType)

						_ = dist
						println()
				}
		*/

	case *packet.SetActorData:
		w.currentWorld(func(world *worldstate.World) {
			if pk.EntityRuntimeID == w.session.Player.RuntimeID {
				w.applyPlayerData(pk.EntityMetadata, pk.EntityProperties, timeReceived)
			}
			if entity := world.GetEntity(pk.EntityRuntimeID); entity != nil {
				w.applyEntityData(entity, pk.EntityMetadata, pk.EntityProperties, timeReceived)
				properties := w.serverState.entityProperties[entity.EntityType]
				w.serverState.behaviorPack.AddEntity(entity.EntityType, nil, entity.Metadata, properties)
			}
		})

	case *packet.SetActorMotion:
		w.currentWorld(func(world *worldstate.World) {
			if e := world.GetEntity(pk.EntityRuntimeID); e != nil {
				e.Velocity = pk.Velocity
			}
		})

	case *packet.MoveActorDelta:
		w.currentWorld(func(world *worldstate.World) {
			if e := world.GetEntity(pk.EntityRuntimeID); e != nil {
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
				if !e.Velocity.ApproxEqual(mgl32.Vec3{}) {
					e.HasMoved = true
				}
			}
		})

	case *packet.MoveActorAbsolute:
		w.currentWorld(func(world *worldstate.World) {
			if e := world.GetEntity(pk.EntityRuntimeID); e != nil {
				e.Position = pk.Position
				e.Pitch = pk.Rotation.X()
				e.Yaw = pk.Rotation.Y()
				if !e.Velocity.ApproxEqual(mgl32.Vec3{}) {
					e.HasMoved = true
				}
			}
		})

	case *packet.MobEquipment:
		if pk.NewItem.Stack.NBTData["map_uuid"] == int64(ViewMapID) {
			_pk = nil
		} else {
			w.currentWorld(func(world *worldstate.World) {
				if e := world.GetEntity(pk.EntityRuntimeID); e != nil {
					w, ok := e.Inventory[pk.WindowID]
					if !ok {
						w = make(map[byte]protocol.ItemInstance)
						e.Inventory[pk.WindowID] = w
					}
					w[pk.HotBarSlot] = pk.NewItem
				}
			})
		}

	case *packet.MobArmourEquipment:
		w.currentWorld(func(world *worldstate.World) {
			if e := world.GetEntity(pk.EntityRuntimeID); e != nil {
				e.Helmet = &pk.Helmet
				e.Chestplate = &pk.Chestplate
				e.Leggings = &pk.Chestplate
				e.Boots = &pk.Boots
			}
		})

	case *packet.SetActorLink:
		w.currentWorld(func(world *worldstate.World) {
			world.AddEntityLink(pk.EntityLink)
		})

	case *packet.ItemStackRequest:
		var requests []protocol.ItemStackRequest
		for _, isr := range pk.Requests {
			for _, sra := range isr.Actions {
				if sra, ok := sra.(*protocol.TakeStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DropStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.PlaceInContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.TakeOutContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItemPacket.Content[0].StackNetworkID {
						continue
					}
				}
			}
			requests = append(requests, isr)
		}
		pk.Requests = requests

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
		switch pk.WindowID {
		case 0:
			w.serverState.playerInventory = pk.Content
		case protocol.WindowIDOffHand:
			_pk = nil
		default:
			// save content
			existing, ok := w.serverState.openItemContainers[byte(pk.WindowID)]
			if ok {
				existing.Content = pk
			}
		}

	case *packet.InventorySlot:
		switch pk.WindowID {
		case 0:
			if w.serverState.playerInventory == nil {
				w.serverState.playerInventory = make([]protocol.ItemInstance, 36)
			}
			w.serverState.playerInventory[pk.Slot] = pk.NewItem
		case protocol.WindowIDOffHand:
			_pk = nil
		default:
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
				item := utils.StackToItem(w.serverState.blocks, c.Stack)
				inv.SetItem(i, item)
			}

			// put into subchunk
			w.currentWorld(func(world *worldstate.World) {
				p := existing.OpenPacket.ContainerPosition
				pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
				world.SetBlockNBT(pos, map[string]any{
					"Items": nbtconv.InvToNBT(inv),
				}, true)
			})

			w.session.SendMessage(locale.Loc("saved_block_inv", nil))

			// remove it again
			delete(w.serverState.openItemContainers, byte(pk.WindowID))
		}
	}

	if w.settings.BlockUpdates {
		switch pk := _pk.(type) {
		case *packet.UpdateBlock:
			w.currentWorld(func(world *worldstate.World) {
				rid, name, properties, found := world.BlockByID(pk.NewBlockRuntimeID)
				if !found {
					return
				}
				apply := w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived)
				if apply {
					world.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
				}
			})

		case *packet.UpdateBlockSynced:
			w.currentWorld(func(world *worldstate.World) {
				rid, name, properties, found := world.BlockByID(pk.NewBlockRuntimeID)
				if !found {
					return
				}
				apply := w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived)
				if apply {
					world.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
				}
			})

		case *packet.UpdateSubChunkBlocks:
			w.currentWorld(func(world *worldstate.World) {
				for _, block := range pk.Blocks {
					rid, name, properties, found := world.BlockByID(block.BlockRuntimeID)
					if !found {
						break
					}
					apply := w.scripting.OnBlockUpdate(name, properties, block.BlockPos, timeReceived)
					if apply {
						world.QueueBlockUpdate(block.BlockPos, rid, uint8(0))
					}
				}
			})
		}
	}

	return _pk, nil
}

func (w *worldsHandler) packetHandler(_ *proxy.Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	drop := w.scripting.OnPacket(pk, toServer, timeReceived)
	if drop {
		return nil, nil
	}
	if preLogin {
		return w.packetHandlerPreLogin(pk, timeReceived)
	}
	return w.packetHandlerIngame(pk, toServer, timeReceived)
}

func (w *worldsHandler) syncActorProperty(pk *packet.SyncActorProperty) {
	entityType, ok := pk.PropertyData["type"].(string)
	if !ok {
		return
	}
	properties, ok := pk.PropertyData["properties"].([]any)
	if !ok {
		return
	}

	var propertiesOut = make([]entity.EntityProperty, 0, len(properties))
	for _, property := range properties {
		property := property.(map[string]any)
		propertyName, ok := property["name"].(string)
		if !ok {
			continue
		}
		propertyType, ok := property["type"].(int32)
		if !ok {
			continue
		}

		var prop entity.EntityProperty
		prop.Name = propertyName
		prop.Type = propertyType

		switch propertyType {
		case entity.PropertyTypeInt:
			min, ok := property["min"].(int32)
			if !ok {
				continue
			}
			max, ok := property["max"].(int32)
			if !ok {
				continue
			}
			prop.Min = float32(min)
			prop.Max = float32(max)
		case entity.PropertyTypeFloat:
			min, ok := property["min"].(float32)
			if !ok {
				continue
			}
			max, ok := property["max"].(float32)
			if !ok {
				continue
			}
			prop.Min = min
			prop.Max = max
		case entity.PropertyTypeBool:
		case entity.PropertyTypeEnum:
			prop.Enum, _ = property["enum"].([]any)
		default:
			fmt.Printf("Unknown property type %d", propertyType)
		}
		propertiesOut = append(propertiesOut, prop)
	}
	w.serverState.entityProperties[entityType] = propertiesOut
	if entityType == "minecraft:player" {
		w.serverState.behaviorPack.SetPlayerProperties(propertiesOut)
	}
}

func (w *worldsHandler) applyPlayerData(entityMetadata protocol.EntityMetadata, entityProperties protocol.EntityProperties, timeReceived time.Time) {
	properties := w.serverState.entityProperties["minecraft:player"]
	applyProperties(w.log, properties, entityProperties, w.serverState.playerProperties)
}

func (w *worldsHandler) applyEntityData(ent *entity.Entity, entityMetadata protocol.EntityMetadata, entityProperties protocol.EntityProperties, timeReceived time.Time) {
	maps.Copy(ent.Metadata, entityMetadata)
	w.scripting.OnEntityDataUpdate(ent, timeReceived)
	properties := w.serverState.entityProperties[ent.EntityType]
	applyProperties(w.log, properties, entityProperties, ent.Properties)
}

func applyProperties(log *logrus.Entry, properties []entity.EntityProperty, entityProperties protocol.EntityProperties, out map[string]*entity.EntityProperty) {
	for _, prop := range entityProperties.IntegerProperties {
		if int(prop.Index) > len(properties)-1 {
			log.Errorf("entity property index more than there are properties, BUG %v", prop)
			continue
		}
		propType := properties[prop.Index]
		if propType.Type == entity.PropertyTypeBool {
			propType.Value = prop.Value == 1
		} else {
			propType.Value = prop.Value
		}
		out[propType.Name] = &propType
	}
	for _, prop := range entityProperties.IntegerProperties {
		if int(prop.Index) > len(properties)-1 {
			log.Errorf("entity property index more than there are properties, BUG %v", prop)
			continue
		}
		propType := properties[prop.Index]
		propType.Value = prop.Value
		out[propType.Name] = &propType
	}
}
