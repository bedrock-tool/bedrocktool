package worlds

import (
	"image"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
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

func (w *worldsHandler) getEntity(id worldstate.EntityRuntimeID) *worldstate.EntityState {
	return w.currentWorld.GetEntity(id)
}

func (w *worldsHandler) packetCB(_pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
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
					world.AddCustomBlocks(w.serverState.blocks, pk.Blocks)
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
		w.processLevelChunk(pk)

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
			cp := world.ChunkPos{pk.Position.X() >> 4, pk.Position.Z() >> 4}
			w.currentWorld.QueueBlockUpdate(cp, pk)
		}

	case *packet.UpdateBlockSynced:
		if w.settings.BlockUpdates {
			cp := world.ChunkPos{pk.Position.X() >> 4, pk.Position.Z() >> 4}
			w.currentWorld.QueueBlockUpdate(cp, pk)
		}

	case *packet.UpdateSubChunkBlocks:
		if w.settings.BlockUpdates {
			cp := world.ChunkPos{pk.Position.X(), pk.Position.Z()}
			w.currentWorld.QueueBlockUpdate(cp, pk)
		}

	case *packet.ClientBoundMapItemData:
		w.currentWorld.StoreMap(pk)

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
		w.serverState.behaviorPack.SyncActorProperty(pk)

	case *packet.AddActor:
		w.currentWorld.ProcessAddActor(pk, func(es *worldstate.EntityState) bool {
			return w.scripting.OnEntityAdd(es, es.Metadata)
		}, w.serverState.behaviorPack.AddEntity)

	case *packet.SetActorData:
		if e := w.getEntity(pk.EntityRuntimeID); e != nil {
			metadata := make(protocol.EntityMetadata)
			maps.Copy(metadata, pk.EntityMetadata)
			w.scripting.OnEntityDataUpdate(e, metadata)
			maps.Copy(e.Metadata, metadata)
			e.Properties = pk.EntityProperties

			w.serverState.behaviorPack.AddEntity(behaviourpack.EntityIn{
				Identifier: e.EntityType,
				Attr:       nil,
				Meta:       metadata,
			})
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
