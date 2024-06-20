package worldstate

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
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
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type worldStateInterface interface {
	StoreChunk(pos world.ChunkPos, ch *chunk.Chunk, blockNBT map[cube.Pos]DummyBlock) error
	SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool)
	StoreEntity(id EntityRuntimeID, es *EntityState)
	GetEntity(id EntityRuntimeID) *EntityState
	AddEntityLink(el protocol.EntityLink)
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

	memState *worldStateDefer
	provider *mcdb.DB
	opened   bool
	// state to be used while paused
	paused      bool
	pausedState *worldStateDefer
	// access to states
	l sync.Mutex

	// closed when this world is done
	finish chan struct{}
	err    error

	players worldPlayers

	VoidGen  bool
	timeSync time.Time
	time     int
	Name     string
	Folder   string
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

func New(cf func(world.ChunkPos, *chunk.Chunk), dimensionDefinitions map[int]protocol.DimensionDefinition, br world.BlockRegistry, br2 *world.BiomeRegistry) (*World, error) {
	w := &World{
		StoredChunks:         make(map[world.ChunkPos]bool),
		dimensionDefinitions: dimensionDefinitions,
		finish:               make(chan struct{}),
		memState: &worldStateDefer{
			chunks: make(map[world.ChunkPos]*chunk.Chunk),
			worldEntities: worldEntities{
				entities:    make(map[EntityRuntimeID]*EntityState),
				entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
				blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]DummyBlock),
			},
		},
		players: worldPlayers{
			players: make(map[uuid.UUID]*player),
		},
		BlockRegistry: br,
		BiomeRegistry: br2,
	}

	return w, nil
}

func (w *World) currState() *worldStateDefer {
	if w.paused {
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
		os.RemoveAll(w.Folder)
		os.MkdirAll(w.Folder, 0o777)
		w.provider, w.err = mcdb.Config{
			Log:         logrus.StandardLogger(),
			Compression: opt.DefaultCompression,
			Blocks:      w.BlockRegistry,
			Biomes:      w.BiomeRegistry,
		}.Open(w.Folder)
		if w.err != nil {
			return w.err
		}
	}
	for pos, ch := range w.memState.chunks {
		// dont put empty chunks in the world db, keep them in memory
		empty := true
		for _, sc := range ch.Sub() {
			if !sc.Empty() {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		err := w.provider.StoreColumn(pos, w.dimension, &world.Column{
			Chunk: ch,
		})
		if err != nil {
			logrus.Error("storeChunk", err)
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

func (w *World) StoreChunk(pos world.ChunkPos, ch *chunk.Chunk, blockNBT map[cube.Pos]DummyBlock) (err error) {
	w.l.Lock()
	defer w.l.Unlock()

	var empty = true
	for _, sub := range ch.Sub() {
		if !sub.Empty() {
			empty = false
			break
		}
	}
	if !empty {
		w.StoredChunks[pos] = true
	}

	w.currState().StoreChunk(pos, ch, blockNBT)

	return nil
}

func (w *World) LoadChunk(pos world.ChunkPos) (*chunk.Chunk, bool, error) {
	w.l.Lock()
	defer w.l.Unlock()

	if w.paused {
		ch, ok := w.pausedState.chunks[pos]
		if ok {
			return ch, true, nil
		}
	}
	ch, ok := w.memState.chunks[pos]
	if ok {
		return ch, true, nil
	}

	if w.provider != nil {
		col, err := w.provider.LoadColumn(pos, w.dimension)
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, err
		}
		w.memState.chunks[pos] = col.Chunk
		return col.Chunk, true, nil
	}

	return nil, false, nil
}

func (w *World) SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool) {
	w.l.Lock()
	defer w.l.Unlock()
	w.currState().SetBlockNBT(pos, nbt, merge)
}

func (w *World) StoreEntity(id EntityRuntimeID, es *EntityState) {
	w.l.Lock()
	defer w.l.Unlock()
	w.currState().StoreEntity(id, es)
}

func (w *World) StoreMap(m *packet.ClientBoundMapItemData) {
	w.l.Lock()
	defer w.l.Unlock()
	w.currState().StoreMap(m)
}

func (w *World) GetEntity(id EntityRuntimeID) *EntityState {
	w.l.Lock()
	defer w.l.Unlock()
	if w.paused {
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
	w.l.Lock()
	defer w.l.Unlock()
	w.currState().AddEntityLink(el)
}

func (w *World) PauseCapture() {
	w.l.Lock()
	w.paused = true
	w.pausedState = &worldStateDefer{
		chunks: make(map[world.ChunkPos]*chunk.Chunk),
		worldEntities: worldEntities{
			entities:    make(map[EntityRuntimeID]*EntityState),
			entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
			blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]DummyBlock),
		},
	}
	w.l.Unlock()
}

func (w *World) UnpauseCapture(around cube.Pos, radius int32, cf func(world.ChunkPos, *chunk.Chunk)) {
	w.l.Lock()
	defer w.l.Unlock()
	if !w.paused {
		panic("attempt to unpause when not paused")
	}
	w.pausedState.ApplyTo(w, around, radius, cf)
	w.pausedState = nil
	w.paused = false
}

func (w *World) IsPaused() bool {
	return w.paused
}

func (w *World) Open(name string, folder string, deferred bool) {
	w.l.Lock()
	defer w.l.Unlock()
	w.Name = name
	w.Folder = folder

	if w.opened {
		panic("trying to open already opened world")
	}
	w.opened = true

	if w.paused && !deferred {
		w.pausedState.ApplyTo(w, cube.Pos{}, -1, w.ChunkFunc)
		w.paused = false
	}

	go func() {
		t := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-w.finish:
				return
			case <-t.C:
				w.l.Lock()
				w.storeMemToProvider()
				w.l.Unlock()
			}
		}
	}()
}

// Rename moves the folder and reopens it
func (w *World) Rename(name, folder string) error {
	w.l.Lock()
	defer w.l.Unlock()

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
			Log:         logrus.StandardLogger(),
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

func (w *World) Finish(playerData map[string]any, excludedMobs []string, withPlayers bool, spawn cube.Pos, gd minecraft.GameData, bp *behaviourpack.Pack) error {
	w.l.Lock()
	defer w.l.Unlock()
	close(w.finish)

	if withPlayers {
		w.playersToEntities()

		for _, p := range w.players.players {
			bp.AddEntity(behaviourpack.EntityIn{
				Identifier: "player:" + p.add.UUID.String(),
			})
		}
	}

	err := w.storeMemToProvider()
	if err != nil {
		return err
	}

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.memState.entities {
		var ignore bool
		for _, ex := range excludedMobs {
			if ok, err := path.Match(ex, es.EntityType); ok {
				logrus.Debugf("Excluding: %s %v", es.EntityType, es.Position)
				ignore = true
				break
			} else if err != nil {
				logrus.Warn(err)
			}
		}
		if !ignore {
			cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
			links := maps.Keys(w.memState.entityLinks[es.UniqueID])
			chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity(links))
		}
	}

	for cp, v := range chunkEntities {
		err := w.provider.StoreEntities(cp, w.dimension, v)
		if err != nil {
			logrus.Error(err)
		}
	}

	for cp, v := range w.memState.blockNBTs {
		vv := make(map[cube.Pos]world.Block, len(v))
		for p, db := range v {
			vv[p] = &db
		}
		err := w.provider.StoreBlockNBTs(cp, w.dimension, vv)
		if err != nil {
			return err
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
		switch gr.Name {
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
		// todo
		default:
			logrus.Warnf(locale.Loc("unknown_gamerule", locale.Strmap{"Name": gr.Name}))
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

	if bp.HasContent() {
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
