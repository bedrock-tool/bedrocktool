package worldstate

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type worldStateInterface interface {
	StoreChunk(pos world.ChunkPos, col *world.Column) error
	SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool) error
	StoreEntity(id entity.RuntimeID, es *entity.Entity)
	GetEntity(id entity.RuntimeID) *entity.Entity
	AddEntityLink(el protocol.EntityLink)
}

type resourcePackDependency struct {
	UUID    string `json:"pack_id"`
	Version [3]int `json:"version"`
}

type World struct {
	// called when a chunk is added
	ChunkFunc func(world.ChunkPos, *chunk.Chunk)

	BlockRegistry world.BlockRegistry
	BiomeRegistry *world.BiomeRegistry

	dimension            world.Dimension
	dimRange             cube.Range
	dimensionDefinitions map[int]protocol.DimensionDefinition
	StoredChunks         map[world.ChunkPos]bool

	memState *worldState
	provider *mcdb.DB
	onceOpen sync.Once
	opened   bool
	// state to be used while paused
	pausedState *worldState
	// access to states
	stateLock sync.RWMutex

	ResourcePacks            []resource.Pack
	resourcePacksDone        chan error
	resourcePackDependencies []resourcePackDependency

	// closed when this world is done
	finish chan struct{}
	err    error

	players map[uuid.UUID]*player

	VoidGen  bool
	timeSync time.Time
	time     int
	Name     string
	Folder   string

	UseHashedRids    bool
	blockUpdatesLock sync.Mutex
	blockUpdates     map[world.ChunkPos][]blockUpdate
	onChunkUpdate    func(pos world.ChunkPos, chunk *chunk.Chunk, isPaused bool)
	IgnoredChunks    map[world.ChunkPos]bool

	log *logrus.Entry
}

type blockUpdate struct {
	rid   uint32
	pos   protocol.BlockPos
	layer uint8
}

type Map struct {
	Decorations       []any            `nbt:"decorations"`
	Dimension         uint8            `nbt:"dimension"`
	Height            int16            `nbt:"height"`
	Width             int16            `nbt:"width"`
	MapID             int64            `nbt:"mapId"`
	Scale             uint8            `nbt:"scale"`
	UnlimitedTracking bool             `nbt:"unlimitedTracking"`
	ZCenter           int32            `nbt:"zCenter"`
	XCenter           int32            `nbt:"xCenter"`
	FullyExplored     bool             `nbt:"fullyExplored"`
	ParentMapId       int64            `nbt:"parentMapId"`
	Colors            [0xffff + 1]byte `nbt:"colors"`
	MapLocked         bool             `nbt:"mapLocked"`
}

func New(dimensionDefinitions map[int]protocol.DimensionDefinition, onChunkUpdate func(pos world.ChunkPos, chunk *chunk.Chunk, isPaused bool)) (*World, error) {
	w := &World{
		StoredChunks:         make(map[world.ChunkPos]bool),
		dimensionDefinitions: dimensionDefinitions,
		finish:               make(chan struct{}),
		memState:             newWorldState(),
		players:              make(map[uuid.UUID]*player),
		blockUpdates:         make(map[world.ChunkPos][]blockUpdate),
		onChunkUpdate:        onChunkUpdate,
		IgnoredChunks:        make(map[world.ChunkPos]bool),
		log:                  logrus.WithFields(logrus.Fields{"part": "world"}),
	}

	return w, nil
}

func (w *World) currState() *worldState {
	if w.pausedState != nil {
		return w.pausedState
	} else {
		return w.memState
	}
}

func (w *World) storeMemToProvider() error {
	if len(w.memState.chunks) == 0 {
		return nil
	}
	if w.provider == nil {
		w.log.Debugf("Opening provider in %s", w.Folder)
		utils.RemoveTree(w.Folder)
		os.MkdirAll(w.Folder, 0o777)
		w.provider, w.err = mcdb.Config{
			Log:         slog.Default(),
			Compression: opt.DefaultCompression,
			Blocks:      w.BlockRegistry,
			Biomes:      w.BiomeRegistry,
		}.Open(w.Folder)
		if w.err != nil {
			return w.err
		}

		w.resourcePacksDone = make(chan error)
		go func() {
			defer close(w.resourcePacksDone)
			err := w.addResourcePacks()
			if err != nil {
				w.resourcePacksDone <- err
			}
		}()
	}
	for pos, col := range w.memState.chunks {
		// dont put empty chunks in the world db, keep them in memory
		empty := true
		for _, sc := range col.Chunk.Sub() {
			if !sc.Empty() {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		err := w.provider.StoreColumn(pos, w.dimension, col)
		if err != nil {
			w.log.Error("StoreColumn", err)
		}
		delete(w.memState.chunks, pos)
	}
	return nil
}

func (w *World) Dimension() world.Dimension {
	return w.dimension
}

func (w *World) SetDimension(dim world.Dimension) {
	w.dimension = dim

	w.dimRange = dim.Range()
	id, _ := world.DimensionID(dim)
	if d, ok := w.dimensionDefinitions[id]; ok {
		w.dimRange = cube.Range{
			int(d.Range[1]), int(d.Range[0]) - 1,
		}
	}
}

func (w *World) Range() cube.Range {
	return w.dimRange
}

func (w *World) SetTime(real time.Time, ingame int) {
	w.timeSync = real
	w.time = ingame
}

func (w *World) StoreChunk(pos world.ChunkPos, col *world.Column) (err error) {
	w.stateLock.RLock()
	defer w.stateLock.RUnlock()
	return w.storeChunkLocked(pos, col)
}

func (w *World) storeChunkLocked(pos world.ChunkPos, col *world.Column) (err error) {
	var empty = true
	for _, sub := range col.Chunk.Sub() {
		if !sub.Empty() {
			empty = false
			break
		}
	}
	if !empty {
		w.StoredChunks[pos] = true
		w.onChunkUpdate(pos, col.Chunk, w.pausedState != nil)
		// only start saving once a non empty chunk is received
		w.onceOpen.Do(func() {
			go func() {
				t := time.NewTicker(10 * time.Second)
				for {
					select {
					case <-w.finish:
						return
					case <-t.C:
						w.stateLock.Lock()
						w.applyBlockUpdates()
						w.storeMemToProvider()
						w.stateLock.Unlock()
					}
				}
			}()
		})
	}

	w.currState().StoreChunk(pos, col)
	return nil
}

func (w *World) LoadChunk(pos world.ChunkPos) (*world.Column, bool, error) {
	w.stateLock.RLock()
	defer w.stateLock.RUnlock()
	return w.loadChunkLocked(pos)
}

func (w *World) loadChunkLocked(pos world.ChunkPos) (*world.Column, bool, error) {
	if w.pausedState != nil {
		if col, ok := w.pausedState.chunks[pos]; ok {
			return col, true, nil
		}
	}
	if col, ok := w.memState.chunks[pos]; ok {
		return col, true, nil
	}

	if w.provider != nil {
		col, err := w.provider.LoadColumn(pos, w.dimension)
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, err
		}
		w.memState.chunks[pos] = col
		return col, true, nil
	}

	return nil, false, nil
}

func (w *World) QueueBlockUpdate(pos protocol.BlockPos, ridTo uint32, layer uint8) {
	cp := world.ChunkPos{pos.X() >> 4, pos.Z() >> 4}
	w.blockUpdatesLock.Lock()
	defer w.blockUpdatesLock.Unlock()
	w.blockUpdates[cp] = append(w.blockUpdates[cp], blockUpdate{rid: ridTo, pos: pos, layer: layer})
}

func (w *World) SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool) error {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	chunkPos, _ := cubePosInChunk(pos)
	col, ok, err := w.loadChunkLocked(chunkPos)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if merge {
		if prev, ok := col.BlockEntities[pos].(world.UnknownBlock); ok {
			prev.Name = nbt["id"].(string)
			maps.Copy(prev.Properties, nbt)
			return nil
		}
	}
	col.BlockEntities[pos] = world.UnknownBlock{
		BlockState: world.BlockState{
			Name:       nbt["id"].(string),
			Properties: nbt,
		},
	}
	return nil
}

func (w *World) StoreEntity(id entity.RuntimeID, es *entity.Entity) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	w.currState().StoreEntity(id, es)
}

func (w *World) StoreMap(m *packet.ClientBoundMapItemData) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	w.currState().StoreMap(m)
}

func (w *World) GetEntityUniqueID(id entity.UniqueID) *entity.Entity {
	rid := w.currState().uniqueIDsToRuntimeIDs[id]
	return w.GetEntity(rid)
}

func (w *World) GetEntity(id entity.RuntimeID) *entity.Entity {
	w.stateLock.RLock()
	defer w.stateLock.RUnlock()
	if w.pausedState != nil {
		es := w.pausedState.GetEntity(id)
		if es != nil {
			return es
		}
	}
	return w.memState.entities[id]
}

func (w *World) EntityCount() int {
	return len(w.memState.entities)
}

func (w *World) AddEntityLink(el protocol.EntityLink) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	w.currState().AddEntityLink(el)
}

func (w *World) PauseCapture() {
	w.stateLock.Lock()
	w.pausedState = newWorldState()
	w.stateLock.Unlock()
}

func (w *World) UnpauseCapture(around cube.Pos, radius int32) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	if w.pausedState == nil {
		panic("attempt to unpause when not paused")
	}
	w.pausedState.ApplyTo(w, around, radius, func(pos world.ChunkPos, ch *chunk.Chunk) {
		w.onChunkUpdate(pos, ch, false)
	})
	w.pausedState = nil
}

func (w *World) IsPaused() bool {
	return w.pausedState != nil
}

func (w *World) Open(name string, folder string, deferred bool) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	w.Name = name
	w.Folder = folder

	if w.opened {
		panic("trying to open already opened world")
	}
	w.opened = true

	if w.pausedState != nil && !deferred {
		w.pausedState.ApplyTo(w, cube.Pos{}, -1, w.ChunkFunc)
	}
}

func blockPosInChunk(bp protocol.BlockPos) (x uint8, y int16, z uint8) {
	return uint8(bp.X() % 16), int16(bp.Y()), uint8(bp.Z() % 16)
}

func (w *World) BlockByID(rid uint32) (runtimeID uint32, name string, properties map[string]any, found bool) {
	if w.UseHashedRids {
		var ok bool
		rid, ok = w.BlockRegistry.HashToRuntimeID(rid)
		if !ok {
			w.log.Warn("couldnt find block hash for block update")
		}
	}
	name, properties, found = w.BlockRegistry.RuntimeIDToState(rid)
	return rid, name, properties, found
}

func (w *World) applyBlockUpdates() {
	w.blockUpdatesLock.Lock()
	defer w.blockUpdatesLock.Unlock()
	for pos, updates := range w.blockUpdates {
		delete(w.blockUpdates, pos)
		col, ok, err := w.loadChunkLocked(world.ChunkPos(pos))
		if !ok {
			w.log.Warnf("Chunk updates for a chunk we dont have pos = %v", pos)
			continue
		}
		if err != nil {
			w.log.Warnf("Failed loading chunk %v %s", pos, err)
			continue
		}

		for _, update := range updates {
			x, y, z := blockPosInChunk(update.pos)
			col.Chunk.SetBlock(x, y, z, update.layer, update.rid)
		}
		err = w.storeChunkLocked(pos, col)
		if err != nil {
			w.log.Warnf("Failed storing chunk %v %s", pos, err)
			continue
		}
	}
}

// Rename moves the folder and reopens it
func (w *World) Rename(name, folder string) error {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	os.RemoveAll(folder)
	if w.provider != nil {
		err := w.provider.Close()
		if err != nil {
			return err
		}
		err = os.Rename(w.Folder, folder)
		if err != nil {
			return err
		}
		w.provider, w.err = mcdb.Config{
			Log:         slog.Default(),
			Compression: opt.DefaultCompression,
			Blocks:      w.BlockRegistry,
			Biomes:      w.BiomeRegistry,
		}.Open(w.Folder)
		if w.err != nil {
			return w.err
		}
	}
	w.Folder = folder
	w.Name = name
	return nil
}

func (w *World) Finish(playerData map[string]any, excludedMobs []string, withPlayers bool, spawn cube.Pos, gd minecraft.GameData, experimental bool) error {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()
	close(w.finish)

	if withPlayers {
		w.playersToEntities()
		/*
			for _, p := range w.players.players {
				bp.AddEntity(behaviourpack.EntityIn{
					Identifier: "player:" + p.add.UUID.String(),
				})
			}
		*/
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  w.Name,
			State: "Storing Chunks",
		},
	})

	w.applyBlockUpdates()
	err := w.storeMemToProvider()
	if err != nil {
		return err
	}

	messages.Router.Handle(&messages.Message{
		Source: "subcommand",
		Target: "ui",
		Data: messages.ProcessingWorldUpdate{
			Name:  w.Name,
			State: "Storing Entities",
		},
	})

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, entityState := range w.memState.entities {
		var ignore bool
		for _, ex := range excludedMobs {
			if ok, err := path.Match(ex, entityState.EntityType); ok {
				w.log.Debugf("Excluding: %s %v", entityState.EntityType, entityState.Position)
				ignore = true
				break
			} else if err != nil {
				w.log.Warn(err)
			}
		}
		if !ignore {
			cp := world.ChunkPos{int32(entityState.Position.X()) >> 4, int32(entityState.Position.Z()) >> 4}
			links := maps.Keys(w.memState.entityLinks[entityState.UniqueID])
			chunkEntities[cp] = append(chunkEntities[cp], entityState.ToServerEntity(links))
		}
	}

	for cp, v := range chunkEntities {
		err := w.provider.StoreEntities(cp, w.dimension, v)
		if err != nil {
			w.log.Error(err)
		}
	}

	err = w.provider.SaveLocalPlayerData(playerData)
	if err != nil {
		return err
	}

	ldb := w.provider.LDB()
	for id, m := range w.memState.maps {
		d, err := nbt.MarshalEncoding(m, nbt.LittleEndian)
		if err != nil {
			return err
		}
		err = ldb.Put([]byte(fmt.Sprintf("map_%d", id)), d, nil)
		if err != nil {
			return err
		}
	}

	// write metadata
	s := w.provider.Settings()
	s.Spawn = spawn
	s.Name = w.Name

	// set gamerules
	ld := w.provider.LevelDat()
	ld.CheatsEnabled = true
	ld.RandomSeed = int64(gd.WorldSeed)
	for _, gr := range gd.GameRules {
		switch strings.ToLower(gr.Name) {
		case "commandblockoutput":
			ld.CommandBlockOutput = gr.Value.(bool)
		case "maxcommandchainlength":
			ld.MaxCommandChainLength = int32(gr.Value.(uint32))
		case "commandblocksenabled":
			//ld.CommandsEnabled = gr.Value.(bool)
		case "dodaylightcycle":
			ld.DoDayLightCycle = gr.Value.(bool)
		case "doentitydrops":
			ld.DoEntityDrops = gr.Value.(bool)
		case "dofiretick":
			ld.DoFireTick = gr.Value.(bool)
		case "domobloot":
			ld.DoMobLoot = gr.Value.(bool)
		case "domobspawning":
			ld.DoMobSpawning = gr.Value.(bool)
		case "dotiledrops":
			ld.DoTileDrops = gr.Value.(bool)
		case "doweathercycle":
			ld.DoWeatherCycle = gr.Value.(bool)
		case "drowningdamage":
			ld.DrowningDamage = gr.Value.(bool)
		case "doinsomnia":
			ld.DoInsomnia = gr.Value.(bool)
		case "falldamage":
			ld.FallDamage = gr.Value.(bool)
		case "firedamage":
			ld.FireDamage = gr.Value.(bool)
		case "keepinventory":
			ld.KeepInventory = gr.Value.(bool)
		case "mobgriefing":
			ld.MobGriefing = gr.Value.(bool)
		case "pvp":
			ld.PVP = gr.Value.(bool)
		case "showcoordinates":
			ld.ShowCoordinates = gr.Value.(bool)
		case "naturalregeneration":
			ld.NaturalRegeneration = gr.Value.(bool)
		case "tntexplodes":
			ld.TNTExplodes = gr.Value.(bool)
		case "sendcommandfeedback":
			ld.SendCommandFeedback = gr.Value.(bool)
		case "randomtickspeed":
			ld.RandomTickSpeed = int32(gr.Value.(uint32))
		case "doimmediaterespawn":
			ld.DoImmediateRespawn = gr.Value.(bool)
		case "showdeathmessages":
			ld.ShowDeathMessages = gr.Value.(bool)
		case "functioncommandlimit":
			ld.FunctionCommandLimit = int32(gr.Value.(uint32))
		case "spawnradius":
			ld.SpawnRadius = int32(gr.Value.(uint32))
		case "showtags":
			ld.ShowTags = gr.Value.(bool)
		case "freezedamage":
			ld.FreezeDamage = gr.Value.(bool)
		case "respawnblocksexplode":
			ld.RespawnBlocksExplode = gr.Value.(bool)
		case "showbordereffect":
			ld.ShowBorderEffect = gr.Value.(bool)
		case "recipesunlock":
			ld.RecipesUnlock = gr.Value.(bool)
		case "dolimitedcrafting":
			ld.DoLimitedCrafting = gr.Value.(bool)
		case "showdaysplayed":
			ld.ShowDaysPlayed = gr.Value.(bool)
		case "showrecipemessages":
			ld.ShowRecipeMessages = gr.Value.(bool)
		case "playerssleepingpercentage":
			ld.PlayersSleepingPercentage = int32(gr.Value.(uint32))
		case "projectilescanbreakblocks":
			ld.ProjectilesCanBreakBlocks = gr.Value.(bool)
		case "tntexplosiondropdecay":
			ld.TNTExplosionDropDecay = gr.Value.(bool)
		default:
			w.log.Warnf(locale.Loc("unknown_gamerule", locale.Strmap{"Name": gr.Name}))
		}
	}

	// void world
	if w.VoidGen {
		ld.FlatWorldLayers = `{"biome_id":1,"block_layers":[{"block_data":0,"block_id":0,"count":1},{"block_data":0,"block_id":0,"count":2},{"block_data":0,"block_id":0,"count":1}],"encoding_version":3,"structure_options":null}`
		ld.Generator = 2
	}

	ld.RandomTickSpeed = 0
	s.CurrentTick = gd.Time

	ticksSince := int64(time.Since(w.timeSync)/time.Millisecond) / 50
	s.Time = int64(w.time)
	if ld.DoDayLightCycle {
		s.Time += ticksSince
		s.TimeCycle = true
	}

	if experimental {
		if ld.Experiments == nil {
			ld.Experiments = map[string]any{}
		}
		ld.Experiments["data_driven_items"] = true
		ld.Experiments["experiments_ever_used"] = true
		ld.Experiments["saved_with_toggled_experiments"] = true
	}

	w.provider.SaveSettings(s)
	return w.provider.Close()
}
