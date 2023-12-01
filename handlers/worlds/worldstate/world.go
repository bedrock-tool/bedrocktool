package worldstate

import (
	"errors"
	"os"
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
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type worldStateInterface interface {
	StoreChunk(pos world.ChunkPos, ch *chunk.Chunk, blockNBT map[cube.Pos]DummyBlock) error
	SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool)
	StoreEntity(id EntityRuntimeID, es *EntityState)
	GetEntity(id EntityRuntimeID) (*EntityState, bool)
	AddEntityLink(el protocol.EntityLink)
}

type World struct {
	dimension            world.Dimension
	dimRange             cube.Range
	dimensionDefinitions map[int]protocol.DimensionDefinition
	deferredState        *worldStateDefer
	StoredChunks         map[world.ChunkPos]bool
	chunkFunc            func(world.ChunkPos, *chunk.Chunk)

	l             sync.Mutex
	provider      *mcdb.DB
	worldEntities worldEntities
	players       worldPlayers

	VoidGen  bool
	timeSync time.Time
	time     int
	Name     string
	Folder   string
}

func New(cf func(world.ChunkPos, *chunk.Chunk), dimensionDefinitions map[int]protocol.DimensionDefinition) (*World, error) {
	w := &World{
		StoredChunks:         make(map[world.ChunkPos]bool),
		dimensionDefinitions: dimensionDefinitions,
		chunkFunc:            cf,
		worldEntities: worldEntities{
			entities:    make(map[EntityRuntimeID]*EntityState),
			entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
			blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]DummyBlock),
		},
		players: worldPlayers{
			players: make(map[uuid.UUID]*player),
		},
	}
	w.initDeferred()

	return w, nil
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
	empty := true
	for _, sc := range ch.Sub() {
		if !sc.Empty() {
			empty = false
			break
		}
	}
	if !empty {
		w.StoredChunks[pos] = true
	}

	if w.deferredState != nil {
		w.deferredState.StoreChunk(pos, ch, blockNBT)
	} else {
		if w.provider == nil { // open provider on first chunk
			w.provider, err = mcdb.Config{
				Log:         logrus.StandardLogger(),
				Compression: opt.DefaultCompression,
			}.Open(w.Folder)
			if err != nil {
				return err
			}
		}

		if len(blockNBT) > 0 {
			if _, ok := w.worldEntities.blockNBTs[pos]; !ok {
				w.worldEntities.blockNBTs[pos] = blockNBT
			} else {
				maps.Copy(w.worldEntities.blockNBTs[pos], blockNBT)
			}
		}

		w.l.Lock()
		err := w.provider.StoreColumn(pos, w.dimension, &world.Column{
			Chunk: ch,
		})
		if err != nil {
			logrus.Error("storeChunk", err)
		}
		w.l.Unlock()
	}
	return nil
}

func (m *World) LoadChunk(pos world.ChunkPos) (*chunk.Chunk, bool, error) {
	if m.deferredState != nil {
		c, ok := m.deferredState.chunks[pos]
		return c, ok, nil
	} else {
		col, err := m.provider.LoadColumn(pos, m.dimension)
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return col.Chunk, true, nil
	}
}

func (w *World) SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool) {
	if w.deferredState != nil {
		w.deferredState.SetBlockNBT(pos, nbt, merge)
	} else {
		w.worldEntities.SetBlockNBT(pos, nbt, merge)
	}
}

func (w *World) StoreEntity(id EntityRuntimeID, es *EntityState) {
	if w.deferredState != nil {
		w.deferredState.StoreEntity(id, es)
	} else {
		w.worldEntities.StoreEntity(id, es)
	}
}
func (w *World) GetEntity(id EntityRuntimeID) (*EntityState, bool) {
	if w.deferredState != nil {
		return w.deferredState.GetEntity(id)
	} else {
		return w.worldEntities.GetEntity(id)
	}
}

func (w *World) EntityCount() int {
	return len(w.worldEntities.entities)
}

func (w *World) AddEntityLink(el protocol.EntityLink) {
	if w.deferredState != nil {
		w.deferredState.AddEntityLink(el)
	} else {
		w.worldEntities.AddEntityLink(el)
	}
}

func (w *World) initDeferred() {
	w.deferredState = &worldStateDefer{
		chunks: make(map[world.ChunkPos]*chunk.Chunk),
		worldEntities: worldEntities{
			entities:    make(map[EntityRuntimeID]*EntityState),
			entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
			blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]DummyBlock),
		},
	}
}

func (w *World) PauseCapture() {
	w.initDeferred()
}

func (w *World) UnpauseCapture(around cube.Pos, radius int32, cf func(world.ChunkPos, *chunk.Chunk)) {
	w.deferredState.ApplyTo(w, around, radius, cf)
	w.deferredState = nil
}

func (w *World) IsDeferred() bool {
	return w.deferredState != nil
}

func (w *World) Open(name string, folder string, deferred bool) {
	w.Name = name
	w.Folder = folder
	os.RemoveAll(w.Folder)
	os.MkdirAll(w.Folder, 0o777)

	if !deferred && w.deferredState != nil {
		w.deferredState.ApplyTo(w, cube.Pos{}, -1, w.chunkFunc)
		w.deferredState = nil
	}
}

func (w *World) Rename(name, folder string) error {
	w.l.Lock()
	defer w.l.Unlock()
	err := w.provider.Close()
	if err != nil {
		return err
	}
	os.RemoveAll(folder)
	err = os.Rename(w.Folder, folder)
	if err != nil {
		return err
	}
	w.Folder = folder
	w.Name = name

	w.provider, err = mcdb.Config{
		Log:         logrus.StandardLogger(),
		Compression: opt.DefaultCompression,
	}.Open(w.Folder)
	if err != nil {
		return err
	}
	return nil
}

func (w *World) Finish(playerData map[string]any, excludedMobs []string, spawn cube.Pos, gd minecraft.GameData, bp *behaviourpack.BehaviourPack) error {
	w.playersToEntities()

	err := w.saveEntities(excludedMobs)
	if err != nil {
		return err
	}

	err = w.saveBlockNBTs(w.dimension)
	if err != nil {
		return err
	}

	err = w.provider.SaveLocalPlayerData(playerData)
	if err != nil {
		return err
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
