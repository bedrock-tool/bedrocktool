package worlds

import (
	"fmt"
	"image"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/bedrock-tool/bedrocktool/utils/resourcepack"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (w *worldsHandler) getEntity(id entity.RuntimeID) *entity.Entity {
	return w.currentWorld.GetEntity(id)
}

func (w *worldsHandler) packetCB(_pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	drop := w.scripting.OnPacket(_pk, toServer, timeReceived)
	if drop {
		return nil, nil
	}

	if preLogin {
		switch pk := _pk.(type) {
		case *packet.CompressedBiomeDefinitionList: // for client side generation, disabled by proxy
			return nil, nil
		case *packet.StartGame:
			if !w.serverState.haveStartGame {
				w.serverState.haveStartGame = true
				w.currentWorld.SetTime(timeReceived, int(pk.Time))
				w.serverState.useHashedRids = pk.UseBlockNetworkIDHashes

				w.serverState.blocks = world.DefaultBlockRegistry.Clone().(*world.BlockRegistryImpl)

				world.InsertCustomItems(pk.Items)
				for _, ie := range pk.Items {
					w.serverState.behaviorPack.AddItem(ie)
				}
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

				w.serverState.WorldName = pk.WorldName
				if pk.WorldName != "" {
					w.currentWorld.Name = pk.WorldName
				}

				if len(pk.BaseGameVersion) > 0 && pk.BaseGameVersion != "*" { // check game version
					gv := strings.Split(pk.BaseGameVersion, ".")
					var err error
					if len(gv) > 1 {
						var ver int
						ver, err = strconv.Atoi(gv[1])
						w.serverState.useOldBiomes = ver < 18
					}
					if err != nil || len(gv) <= 1 {
						w.log.Info(locale.Loc("guessing_version", nil))
					}

					if w.serverState.useOldBiomes {
						w.log.Info(locale.Loc("using_under_118", nil))
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
			w.serverState.behaviorPack.ApplyComponentEntries(pk.Items)

		case *packet.BiomeDefinitionList:
			var biomes map[string]any
			err := nbt.UnmarshalEncoding(pk.SerialisedBiomeDefinitions, &biomes, nbt.NetworkLittleEndian)
			if err != nil {
				w.log.WithField("packet", "BiomeDefinitionList").Error(err)
			}

			for k, v := range biomes {
				_, ok := w.serverState.biomes.BiomeByName(k)
				if !ok {
					w.serverState.biomes.Register(&customBiome{
						name: k,
						data: v.(map[string]any),
					})
				}
			}
			w.serverState.behaviorPack.AddBiomes(biomes)
		}

		return _pk, nil
	}

	switch pk := _pk.(type) {
	case *packet.RequestChunkRadius:
		pk.ChunkRadius = w.settings.ChunkRadius
		pk.MaxChunkRadius = w.settings.ChunkRadius

	case *packet.ChunkRadiusUpdated:
		w.serverState.radius = pk.ChunkRadius
		pk.ChunkRadius = w.settings.ChunkRadius

	case *packet.SetCommandsEnabled:
		pk.Enabled = true

	case *packet.SetTime:
		w.currentWorld.SetTime(timeReceived, int(pk.Time))

	// chunk

	case *packet.ChangeDimension:
		dim, _ := world.DimensionByID(int(pk.Dimension))
		w.SaveAndReset(false, dim)

	case *packet.LevelChunk:
		w.processLevelChunk(pk, timeReceived)

	case *packet.SubChunk:
		if err := w.processSubChunk(pk); err != nil {
			w.log.WithField("packet", "SubChunk").Error(err)
		}

	case *packet.BlockActorData:
		p := pk.Position
		pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
		w.currentWorld.SetBlockNBT(pos, pk.NBTData, false)

	case *packet.UpdateBlock:
		if w.settings.BlockUpdates {
			rid, name, properties, found := w.currentWorld.BlockByID(pk.NewBlockRuntimeID)
			if !found {
				break
			}
			apply := w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived)
			if apply {
				w.currentWorld.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
			}
		}

	case *packet.UpdateBlockSynced:
		if w.settings.BlockUpdates {
			rid, name, properties, found := w.currentWorld.BlockByID(pk.NewBlockRuntimeID)
			if !found {
				break
			}
			apply := w.scripting.OnBlockUpdate(name, properties, pk.Position, timeReceived)
			if apply {
				w.currentWorld.QueueBlockUpdate(pk.Position, rid, uint8(pk.Layer))
			}
		}

	case *packet.UpdateSubChunkBlocks:
		if w.settings.BlockUpdates {
			for _, block := range pk.Blocks {
				rid, name, properties, found := w.currentWorld.BlockByID(block.BlockRuntimeID)
				if !found {
					break
				}
				apply := w.scripting.OnBlockUpdate(name, properties, block.BlockPos, timeReceived)
				if apply {
					w.currentWorld.QueueBlockUpdate(block.BlockPos, rid, uint8(0))
				}
			}

		}

	case *packet.ClientBoundMapItemData:
		w.currentWorld.StoreMap(pk)

	case *packet.SpawnParticleEffect:
		w.scripting.OnSpawnParticle(pk.ParticleName, pk.Position, timeReceived)

		// player
	case *packet.AddPlayer:
		w.currentWorld.AddPlayer(pk)
		w.addPlayer(pk)
	case *packet.PlayerList:
		if pk.ActionType == packet.PlayerListActionAdd { // remove
			for _, player := range pk.Entries {
				w.serverState.playerSkins[player.UUID] = &player.Skin
			}
		}

	case *packet.PlayerSkin:
		w.serverState.playerSkins[pk.UUID] = &pk.Skin

	// entity

	case *packet.SyncActorProperty:
		w.syncActorProperty(pk)

	case *packet.AddActor:
		ent := w.currentWorld.GetEntity(pk.EntityRuntimeID)
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

		if !w.scripting.OnEntityAdd(ent, ent.Metadata, ent.Properties, timeReceived) {
			logrus.Infof("Ignoring Entity: %s %d", ent.EntityType, ent.UniqueID)
		} else {
			w.currentWorld.StoreEntity(pk.EntityRuntimeID, ent)
			for _, el := range pk.EntityLinks {
				w.currentWorld.AddEntityLink(el)
			}
			w.serverState.behaviorPack.AddEntity(pk.EntityType, pk.Attributes, ent.Metadata, ent.Properties)
		}

	case *packet.RemoveActor:
		entity := w.currentWorld.GetEntityUniqueID(pk.EntityUniqueID)
		if entity != nil {
			/*
				dist := entity.Position.Vec2().Sub(playerPos.Vec2()).Len()

				fmt.Fprintf(distf, "%.5f\t%s\n", dist, entity.EntityType)

				_ = dist
				println()
			*/
		}

	case *packet.SetActorData:
		if entity := w.getEntity(pk.EntityRuntimeID); entity != nil {
			w.applyEntityData(entity, pk.EntityMetadata, pk.EntityProperties, timeReceived)
			w.serverState.behaviorPack.AddEntity(entity.EntityType, nil, entity.Metadata, entity.Properties)
		}

	case *packet.SetActorMotion:
		if e := w.getEntity(pk.EntityRuntimeID); e != nil {
			e.Velocity = pk.Velocity
		}

	case *packet.MoveActorDelta:
		if e := w.getEntity(pk.EntityRuntimeID); e != nil {
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

	case *packet.MoveActorAbsolute:
		if e := w.getEntity(pk.EntityRuntimeID); e != nil {
			e.Position = pk.Position
			e.Pitch = pk.Rotation.X()
			e.Yaw = pk.Rotation.Y()
			if !e.Velocity.ApproxEqual(mgl32.Vec3{}) {
				e.HasMoved = true
			}
		}

	case *packet.MobEquipment:
		if pk.NewItem.Stack.NBTData["map_uuid"] == int64(ViewMapID) {
			_pk = nil
		} else {
			if e := w.getEntity(pk.EntityRuntimeID); e != nil {
				w, ok := e.Inventory[pk.WindowID]
				if !ok {
					w = make(map[byte]protocol.ItemInstance)
					e.Inventory[pk.WindowID] = w
				}
				w[pk.HotBarSlot] = pk.NewItem
			}
		}

	case *packet.MobArmourEquipment:
		if e := w.getEntity(pk.EntityRuntimeID); e != nil {
			e.Helmet = &pk.Helmet
			e.Chestplate = &pk.Chestplate
			e.Leggings = &pk.Chestplate
			e.Boots = &pk.Boots
		}

	case *packet.SetActorLink:
		w.currentWorld.AddEntityLink(pk.EntityLink)

	// map

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

	// items

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
				item := utils.StackToItem(w.serverState.blocks, c.Stack)
				inv.SetItem(i, item)
			}

			// put into subchunk
			p := existing.OpenPacket.ContainerPosition
			pos := cube.Pos{int(p.X()), int(p.Y()), int(p.Z())}
			w.currentWorld.SetBlockNBT(pos, map[string]any{
				"Items": nbtconv.InvToNBT(inv),
			}, true)

			w.session.SendMessage(locale.Loc("saved_block_inv", nil))

			// remove it again
			delete(w.serverState.openItemContainers, byte(pk.WindowID))
		}
	}

	return _pk, nil
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
		case entity.PropertyTypeBool:
		case entity.PropertyTypeEnum:
			prop.Enum, _ = property["enum"].([]any)
		default:
			fmt.Printf("Unknown property type %d", propertyType)
			continue
		}

		propertiesOut = append(propertiesOut, prop)
	}

	w.serverState.entityProperties[entityType] = propertiesOut
}

func (w *worldsHandler) applyEntityData(ent *entity.Entity, EntityMetadata protocol.EntityMetadata, EntityProperties protocol.EntityProperties, timeReceived time.Time) {
	maps.Copy(ent.Metadata, EntityMetadata)
	w.scripting.OnEntityDataUpdate(ent, timeReceived)

	for _, prop := range EntityProperties.IntegerProperties {
		propType := w.serverState.entityProperties[ent.EntityType][prop.Index]
		if propType.Type == entity.PropertyTypeBool {
			propType.Value = prop.Value == 1
		} else {
			propType.Value = prop.Value
		}
		ent.Properties[propType.Name] = &propType
	}
	for _, prop := range EntityProperties.IntegerProperties {
		propType := w.serverState.entityProperties[ent.EntityType][prop.Index]
		propType.Value = prop.Value
		ent.Properties[propType.Name] = &propType
	}
}

func (w *worldsHandler) addPlayer(pk *packet.AddPlayer) {
	skin, ok := w.serverState.playerSkins[pk.UUID]
	if ok {
		skinTexture := image.NewRGBA(image.Rect(0, 0, int(skin.SkinImageWidth), int(skin.SkinImageHeight)))
		copy(skinTexture.Pix, skin.SkinData)

		var capeTexture *image.RGBA
		if skin.CapeID != "" {
			capeTexture = image.NewRGBA(image.Rect(0, 0, int(skin.CapeImageWidth), int(skin.CapeImageHeight)))
			copy(capeTexture.Pix, skin.CapeData)
		}

		var resourcePatch map[string]map[string]string
		if len(skin.SkinResourcePatch) > 0 {
			err := utils.ParseJson(skin.SkinResourcePatch, &resourcePatch)
			if err != nil {
				w.log.WithField("data", "SkinResourcePatch").Error(err)
				return
			}
		}

		var geometryName = ""
		if resourcePatch != nil {
			geometryName = resourcePatch["geometry"]["default"]
		}

		var geometry *resourcepack.GeometryFile
		var isDefault bool
		if len(skin.SkinGeometry) > 0 {
			skinGeometry, _, err := utils.ParseSkinGeometry(skin.SkinGeometry)
			if err != nil {
				w.log.WithField("player", pk.Username).Warn(err)
				return
			}
			if skinGeometry != nil {
				geometry = &resourcepack.GeometryFile{
					FormatVersion: string(skin.GeometryDataEngineVersion),
					Geometry: []*resourcepack.Geometry{
						{
							Description: skinGeometry.Description,
							Bones:       skinGeometry.Bones,
						},
					},
				}
			}
		}
		if geometry == nil {
			geometry = &resourcepack.GeometryFile{
				FormatVersion: string(skin.GeometryDataEngineVersion),
				Geometry: []*resourcepack.Geometry{
					{
						Description: utils.SkinGeometryDescription{
							Identifier:    geometryName,
							TextureWidth:  int(skin.SkinImageWidth),
							TextureHeight: int(skin.SkinImageHeight),
						},
					},
				},
			}
			isDefault = true
		}

		w.serverState.resourcePack.AddPlayer(pk.UUID.String(), skinTexture, capeTexture, skin.CapeID, geometry, isDefault)
	}
}
