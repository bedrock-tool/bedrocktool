package worlds

import (
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type worldState struct {
	dimension          world.Dimension
	chunks             map[world.ChunkPos]*chunk.Chunk
	blockNBTs          map[cube.Pos]map[string]any
	entities           map[uint64]*entityState
	entityLinks        map[int64]map[int64]struct{}
	openItemContainers map[byte]*itemContainer
	VoidGen            bool
	timeSync           time.Time
	time               int
	Name               string
}

func newWorldState(name string, dim world.Dimension) *worldState {
	if dim == nil {
		dim = world.Overworld
	}
	return &worldState{
		dimension:          dim,
		chunks:             make(map[world.ChunkPos]*chunk.Chunk),
		blockNBTs:          make(map[cube.Pos]map[string]any),
		entities:           make(map[uint64]*entityState),
		entityLinks:        make(map[int64]map[int64]struct{}),
		openItemContainers: make(map[byte]*itemContainer),
		Name:               name,
	}
}

func (w *worldState) cullChunks() {
	for key, ch := range w.chunks {
		var empty = true
		for _, sub := range ch.Sub() {
			if !sub.Empty() {
				empty = false
				break
			}
		}
		if empty {
			delete(w.chunks, key)
		}
	}
}

func (w *worldState) startSave(folder string) (*mcdb.DB, error) {
	provider, err := mcdb.Config{
		Log:         logrus.StandardLogger(),
		Compression: opt.DefaultCompression,
	}.Open(folder)
	if err != nil {
		return nil, err
	}

	chunkBlockNBT := make(map[world.ChunkPos]map[cube.Pos]world.Block)
	for bp, blockNBT := range w.blockNBTs { // 3d to 2d
		cp := world.ChunkPos{int32(bp.X()) >> 4, int32(bp.Z()) >> 4}
		m, ok := chunkBlockNBT[cp]
		if !ok {
			m = make(map[cube.Pos]world.Block)
			chunkBlockNBT[cp] = m
		}
		id := blockNBT["id"].(string)
		m[bp] = &dummyBlock{id, blockNBT}
	}

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.entities {
		cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
		links := maps.Keys(w.entityLinks[es.UniqueID])
		chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity(links))
	}

	// save chunk data
	for cp, c := range w.chunks {
		column := &world.Column{
			Chunk:         c,
			BlockEntities: chunkBlockNBT[cp],
			Entities:      chunkEntities[cp],
		}
		err = provider.StoreColumn(cp, w.dimension, column)
		if err != nil {
			logrus.Error(err)
		}
	}

	return provider, err
}

func (w *worldState) Save(folder string, playerData map[string]any, spawn cube.Pos, gd minecraft.GameData, bp *behaviourpack.BehaviourPack) error {
	// open world
	os.RemoveAll(folder)
	os.MkdirAll(folder, 0o777)
	provider, err := w.startSave(folder)
	if err != nil {
		return err
	}

	err = provider.SaveLocalPlayerData(playerData)
	if err != nil {
		return err
	}

	// write metadata
	s := provider.Settings()
	s.Spawn = spawn
	s.Name = w.Name

	// set gamerules
	ld := provider.LevelDat()
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

	provider.SaveSettings(s)
	err = provider.Close()
	if err != nil {
		return err
	}
	return nil
}
