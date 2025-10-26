package worlds

import (
	"fmt"
	"maps"
	"math"
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
			w.openWorldState()
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
		if toServer && pk.ActionType == packet.AnimateActionSwingArm && !w.mapUI.isDisabled {
			w.mapUI.ChangeZoom()
			w.session.SendPopup(locale.Loc("zoom_level", locale.Strmap{"Level": w.mapUI.zoomLevel}))
		}

	case *packet.SpawnParticleEffect:
		if w.scripting != nil {
			w.scripting.OnSpawnParticle(pk.ParticleName, pk.Position, timeReceived)
		}

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
		w.handleSyncActorProperty(pk)

	case *packet.AddActor:
		w.currentWorld(func(world *worldstate.World) {
			err := world.ActEntity(pk.EntityRuntimeID, true, func(ent *entity.Entity) error {
				isNew := ent.EntityType == ""

				// get samples of the entity render distance
				if ent.DeletedDistance > 0 && len(w.serverState.entityRenderDistances) < 200 {
					playerPos := w.session.Player.Position
					dist3d := ent.Position.Sub(playerPos).Len()
					diff := float32(math.Abs(float64(ent.DeletedDistance - dist3d)))
					if diff < 10 {
						w.serverState.entityRenderDistances = append(w.serverState.entityRenderDistances, dist3d)
					}
				}
				ent.DeletedDistance = -1
				ent.LastTeleport = 0

				prevPosition := ent.Position

				ent.UniqueID = pk.EntityUniqueID
				ent.EntityType = pk.EntityType
				ent.Position = pk.Position
				ent.Pitch = pk.Pitch
				ent.Yaw = pk.Yaw
				ent.HeadYaw = pk.HeadYaw
				ent.Velocity = pk.Velocity

				maps.Copy(ent.Metadata, pk.EntityMetadata)
				properties := w.serverState.entityProperties[ent.EntityType]
				changes := applyProperties(properties, pk.EntityProperties, ent.Properties)

				if w.scripting != nil {
					if !w.scripting.OnEntityAdd(ent, isNew, timeReceived) {
						logrus.Infof("Ignoring Entity: %s %d", ent.EntityType, ent.UniqueID)
						return worldstate.ErrIgnoreEntity
					}
				}

				if !isNew {
					w.onEntityUpdate(ent, &prevPosition, changes)
				}

				for _, el := range pk.EntityLinks {
					world.AddEntityLink(el)
				}
				w.serverState.behaviorPack.AddEntity(pk.EntityType, pk.Attributes, ent.Metadata, properties)
				return nil
			})
			if err != nil {
				logrus.Errorf("AddActor: %s", err)
			}
		})

	case *packet.MovePlayer:
		entityRenderDistance := w.serverState.getEntityRenderDistance()
		if pk.EntityRuntimeID == w.session.Player.RuntimeID && pk.Mode == packet.MoveModeTeleport {
			w.currentWorld(func(world *worldstate.World) {
				world.PlayerMove(w.session.Player.TeleportLocation, entityRenderDistance, w.session.Player.Teleports)
			})
		}

	case *packet.RemoveActor:
		entityRenderDistance := w.serverState.getEntityRenderDistance()
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(world.GetEntityRuntimeID(pk.EntityUniqueID), false, func(ent *entity.Entity) error {
				playerPos := w.session.Player.Position
				dist3d := ent.Position.Sub(playerPos).Len()

				/*
					player2d := mgl32.Vec2{playerPos.X(), playerPos.Z()}
					entity2d := mgl32.Vec2{ent.Position.X(), ent.Position.Z()}
					dist2d := entity2d.Sub(player2d).Len()

					playerChunk := mgl64.Vec2{math.Floor(float64(playerPos.X() / 16)), math.Floor(float64(playerPos.Z() / 16))}
					entityChunk := mgl64.Vec2{math.Floor(float64(ent.Position.X() / 16)), math.Floor(float64(ent.Position.Z() / 16))}
					distch := entityChunk.Sub(playerChunk).Len()
					if utils.IsDebug() {
						fmt.Printf("RemoveActor 3d: %.5f 2d: %.5f ch: %.5f\t%s\n", dist3d, dist2d, distch, ent.EntityType)
					}
				*/

				// if the player recently was teleported, check if the removed actor was in the view distance previously
				// if it was, that means it wasnt actually removed,
				// set delete distance to the render distance to say its an out of view remove instead
				if w.session.Player.Teleports == ent.LastTeleport {
					dist3d := w.session.Player.TeleportLocation.Sub(ent.Position).Len()
					if dist3d < entityRenderDistance+3 {
						ent.DeletedDistance = entityRenderDistance
					}
				} else {
					ent.DeletedDistance = dist3d
				}
				w.onEntityUpdate(ent, nil, nil)
				return nil
			})
		})

	case *packet.SetActorData:
		w.currentWorld(func(world *worldstate.World) {
			if pk.EntityRuntimeID == w.session.Player.RuntimeID {
				properties := w.serverState.entityProperties["minecraft:player"]
				changed := applyProperties(properties, pk.EntityProperties, w.serverState.playerPropertyValues)
				_ = changed
			} else {
				world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
					properties := w.serverState.entityProperties[ent.EntityType]
					maps.Copy(ent.Metadata, pk.EntityMetadata)
					changedProperties := applyProperties(properties, pk.EntityProperties, ent.Properties)
					w.serverState.behaviorPack.AddEntity(ent.EntityType, nil, ent.Metadata, properties)
					w.onEntityUpdate(ent, nil, changedProperties)
					return nil
				})
			}
		})

	case *packet.PlayerUpdateEntityOverrides:
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(world.GetEntityRuntimeID(pk.EntityUniqueID), false, func(ent *entity.Entity) error {
				var changes = make(map[string]any)
				if pk.Type == packet.PlayerUpdateEntityOverridesTypeClearAll {
					for name := range ent.Properties {
						ent.Properties[name] = nil
					}
				} else {
					properties := w.serverState.entityProperties[ent.EntityType]
					propertyName := properties[pk.PropertyIndex].Name
					prevValue := ent.Properties[propertyName]
					var value any
					switch pk.Type {
					case packet.PlayerUpdateEntityOverridesTypeRemove:
						delete(ent.Properties, propertyName)
						value = nil
					case packet.PlayerUpdateEntityOverridesTypeInt:
						value = pk.IntValue
					case packet.PlayerUpdateEntityOverridesTypeFloat:
						value = pk.FloatValue
					}
					if value != nil {
						ent.Properties[propertyName] = value
					}
					changes[propertyName] = prevValue
				}

				w.onEntityUpdate(ent, nil, changes)
				return nil
			})
		})

	case *packet.SetActorMotion:
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
				ent.Velocity = pk.Velocity
				w.onEntityUpdate(ent, nil, nil)
				return nil
			})

		})

	case *packet.MoveActorDelta:
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
				prevPosition := ent.Position
				if pk.Flags&packet.MoveActorDeltaFlagHasX != 0 {
					ent.Position[0] = pk.Position[0]
				}
				if pk.Flags&packet.MoveActorDeltaFlagHasY != 0 {
					ent.Position[1] = pk.Position[1]
				}
				if pk.Flags&packet.MoveActorDeltaFlagHasZ != 0 {
					ent.Position[2] = pk.Position[2]
				}
				if pk.Flags&packet.MoveActorDeltaFlagHasRotX != 0 {
					ent.Pitch = pk.Rotation.X()
				}
				if pk.Flags&packet.MoveActorDeltaFlagHasRotY != 0 {
					ent.Yaw = pk.Rotation.Y()
				}
				if !ent.Velocity.ApproxEqual(mgl32.Vec3{}) {
					ent.HasMoved = true
				}
				w.onEntityUpdate(ent, &prevPosition, nil)
				return nil
			})
		})

	case *packet.MoveActorAbsolute:
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
				prevPosition := ent.Position
				ent.Position = pk.Position
				ent.Pitch = pk.Rotation.X()
				ent.Yaw = pk.Rotation.Y()
				if !ent.Velocity.ApproxEqual(mgl32.Vec3{}) {
					ent.HasMoved = true
				}
				w.onEntityUpdate(ent, &prevPosition, nil)
				return nil
			})
		})

	case *packet.MobEquipment:
		if pk.EntityRuntimeID == w.session.Player.RuntimeID {
			if pk.NewItem.Stack.NBTData["map_uuid"] == int64(ViewMapID) {
				_pk = nil
			}
		}
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
				window, ok := ent.Inventory[pk.WindowID]
				if !ok {
					window = make(map[byte]protocol.ItemInstance)
					ent.Inventory[pk.WindowID] = window
				}
				window[pk.HotBarSlot] = pk.NewItem
				w.onEntityUpdate(ent, nil, nil)
				return nil
			})
		})

	case *packet.MobArmourEquipment:
		w.currentWorld(func(world *worldstate.World) {
			world.ActEntity(pk.EntityRuntimeID, false, func(ent *entity.Entity) error {
				ent.Helmet = &pk.Helmet
				ent.Chestplate = &pk.Chestplate
				ent.Leggings = &pk.Chestplate
				ent.Boots = &pk.Boots
				w.onEntityUpdate(ent, nil, nil)
				return nil
			})
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
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DropStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.PlaceInContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.TakeOutContainerStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
						continue
					}
				}
				if sra, ok := sra.(*protocol.DestroyStackRequestAction); ok {
					if sra.Source.StackNetworkID == mapItem.StackNetworkID {
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
			if !w.mapUI.isDisabled {
				_pk = nil
				if len(pk.Content) > 0 {
					w.mapUI.offHandItem = pk.Content[0]
				}
			}
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
			if !w.mapUI.isDisabled {
				_pk = nil
				w.mapUI.offHandItem = pk.NewItem
			}
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
				if w.scripting != nil {
					if !w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived) {
						return
					}
				}
				world.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
			})

		case *packet.UpdateBlockSynced:
			w.currentWorld(func(world *worldstate.World) {
				rid, name, properties, found := world.BlockByID(pk.NewBlockRuntimeID)
				if !found {
					return
				}
				if w.scripting != nil {
					if !w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived) {
						return
					}
				}
				world.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
			})

		case *packet.UpdateSubChunkBlocks:
			w.currentWorld(func(world *worldstate.World) {
				for _, block := range pk.Blocks {
					rid, name, properties, found := world.BlockByID(block.BlockRuntimeID)
					if !found {
						break
					}
					if w.scripting != nil {
						if !w.scripting.OnBlockUpdate(name, properties, block.BlockPos, timeReceived) {
							return
						}
					}
					world.QueueBlockUpdate(block.BlockPos, rid, uint8(0))
				}
			})
		}
	}

	return _pk, nil
}

func (w *worldsHandler) packetHandler(_ *proxy.Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	if w.scripting != nil {
		if !w.scripting.OnPacket(pk, toServer, timeReceived) {
			return nil, nil
		}
	}
	if preLogin {
		return w.packetHandlerPreLogin(pk, timeReceived)
	}
	return w.packetHandlerIngame(pk, toServer, timeReceived)
}

func (w *worldsHandler) handleSyncActorProperty(pk *packet.SyncActorProperty) {
	entityType, ok := pk.PropertyData["type"].(string)
	if !ok {
		return
	}
	properties, ok := pk.PropertyData["properties"].([]any)
	if !ok {
		return
	}

	var propertiesOut = make([]entity.EntityPropertyDef, 0, len(properties))
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

		var prop entity.EntityPropertyDef
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

func applyProperties(defs []entity.EntityPropertyDef, props protocol.EntityProperties, out map[string]any) (changed map[string]any) {
	changed = make(map[string]any)

	updateVal := func(name string, value any) {
		prev := out[name]
		out[name] = value
		if prev != out[name] {
			changed[name] = prev
		}
	}

	for _, prop := range props.IntegerProperties {
		if int(prop.Index) > len(defs)-1 {
			logrus.Errorf("entity property index more than there are properties, BUG %v", prop)
			continue
		}
		propType := defs[prop.Index]
		var value any = prop.Value
		if propType.Type == entity.PropertyTypeBool {
			value = prop.Value == 1
		}
		updateVal(propType.Name, value)
	}
	for _, prop := range props.FloatProperties {
		if int(prop.Index) > len(defs)-1 {
			logrus.Errorf("entity property index more than there are properties, BUG %v", prop)
			continue
		}
		propType := defs[prop.Index]
		updateVal(propType.Name, prop.Value)
	}
	return changed
}

func (w *worldsHandler) onEntityUpdate(
	ent *entity.Entity,
	prevPosition *mgl32.Vec3,
	changedProperties map[string]any,
) {
	if w.scripting != nil {
		w.scripting.OnEntityUpdate(ent, prevPosition, changedProperties, w.session.Now())
	}
}
