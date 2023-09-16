package worlds

import (
	"os"
	"slices"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
	"github.com/thomaso-mirodin/intmath/i32"
	"golang.org/x/exp/maps"
)

type EntityRuntimeID = uint64
type EntityUniqueID = int64

type worldStateInt interface {
	storeChunk(pos world.ChunkPos, dim world.Dimension, ch *chunk.Chunk, blockNBT map[cube.Pos]dummyBlock)
	storeEntity(id EntityRuntimeID, es *entityState)
	haveEntity(id EntityRuntimeID) bool
	getEntity(id EntityRuntimeID) (*entityState, bool)
	addEntityLink(el protocol.EntityLink)
	SetBlockNBT(pos cube.Pos, nbt map[string]any, merge bool)
}

type worldStateEnt struct {
	entities    map[EntityRuntimeID]*entityState
	entityLinks map[EntityUniqueID]map[EntityUniqueID]struct{}
	blockNBTs   map[world.ChunkPos]map[cube.Pos]dummyBlock
}

func (w *worldStateEnt) storeEntity(id EntityRuntimeID, es *entityState) {
	w.entities[id] = es
}

func (w *worldStateEnt) haveEntity(id EntityRuntimeID) bool {
	_, ok := w.entities[id]
	return ok
}

func (w *worldStateEnt) getEntity(id EntityRuntimeID) (*entityState, bool) {
	e, ok := w.entities[id]
	return e, ok
}

func (w *worldStateEnt) addEntityLink(el protocol.EntityLink) {
	switch el.Type {
	case protocol.EntityLinkPassenger:
		fallthrough
	case protocol.EntityLinkRider:
		if _, ok := w.entityLinks[el.RiddenEntityUniqueID]; !ok {
			w.entityLinks[el.RiddenEntityUniqueID] = make(map[int64]struct{})
		}
		w.entityLinks[el.RiddenEntityUniqueID][el.RiderEntityUniqueID] = struct{}{}
	case protocol.EntityLinkRemove:
		delete(w.entityLinks[el.RiddenEntityUniqueID], el.RiderEntityUniqueID)
	}
}

func cubePosInChunk(pos cube.Pos) (p world.ChunkPos, sp int16) {
	p[0] = int32(pos.X() >> 4)
	sp = int16(pos.Y() >> 4)
	p[1] = int32(pos.Z() >> 4)
	return
}

func (w *worldStateEnt) SetBlockNBT(pos cube.Pos, m map[string]any, merge bool) {
	cp, _ := cubePosInChunk(pos)
	chunkNBTs, ok := w.blockNBTs[cp]
	if !ok {
		chunkNBTs = make(map[cube.Pos]dummyBlock)
		w.blockNBTs[cp] = chunkNBTs
	}
	b, ok := chunkNBTs[pos]
	if !ok {
		b = dummyBlock{
			id:  m["id"].(string),
			nbt: m,
		}
	}

	if merge {
		maps.Copy(b.nbt, m)
	} else {
		b.nbt = m
	}
	chunkNBTs[pos] = b
}

type worldStateInternal struct {
	l        *sync.Mutex
	provider *mcdb.DB
	worldStateEnt
}

func (w *worldStateInternal) storeChunk(pos world.ChunkPos, dim world.Dimension, ch *chunk.Chunk, blockNBT map[cube.Pos]dummyBlock) {
	w.l.Lock()
	defer w.l.Unlock()
	if len(blockNBT) > 0 {
		if _, ok := w.blockNBTs[pos]; !ok {
			w.blockNBTs[pos] = blockNBT
		} else {
			maps.Copy(w.blockNBTs[pos], blockNBT)
		}
	}

	err := w.provider.StoreColumn(pos, dim, &world.Column{
		Chunk: ch,
	})
	if err != nil {
		logrus.Error("storeChunk", err)
	}
}

func (w *worldStateInternal) saveEntities(exclude []string, dimension world.Dimension) error {
	w.l.Lock()
	defer w.l.Unlock()

	chunkEntities := make(map[world.ChunkPos][]world.Entity)
	for _, es := range w.entities {
		if slices.Contains(exclude, es.EntityType) {
			continue
		}
		cp := world.ChunkPos{int32(es.Position.X()) >> 4, int32(es.Position.Z()) >> 4}
		links := maps.Keys(w.entityLinks[es.UniqueID])
		chunkEntities[cp] = append(chunkEntities[cp], es.ToServerEntity(links))
	}

	for cp, v := range chunkEntities {
		err := w.provider.StoreEntities(cp, dimension, v)
		if err != nil {
			logrus.Error(err)
		}
	}

	return nil
}

func (w *worldStateInternal) saveBlockNBTs(dim world.Dimension) error {
	for cp, v := range w.blockNBTs {
		vv := make(map[cube.Pos]world.Block, len(v))
		for p, db := range v {
			vv[p] = &db
		}
		err := w.provider.StoreBlockNBTs(cp, dim, vv)
		if err != nil {
			return err
		}
	}
	return nil
}

type worldStateDefer struct {
	chunks map[world.ChunkPos]*chunk.Chunk
	worldStateEnt
}

func (w *worldStateDefer) storeChunk(pos world.ChunkPos, dim world.Dimension, ch *chunk.Chunk, blockNBT map[cube.Pos]dummyBlock) {
	w.chunks[pos] = ch
	w.blockNBTs[pos] = blockNBT
}

func (w *worldStateDefer) cullChunks() {
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

func (w *worldStateDefer) ApplyTo(w2 worldStateInt, dimension world.Dimension, around cube.Pos, radius int32, cf func(world.ChunkPos, *chunk.Chunk)) {
	w.cullChunks()
	for cp, c := range w.chunks {
		dist := i32.Sqrt(i32.Pow(cp.X()-int32(around.X()/16), 2) + i32.Pow(cp.Z()-int32(around.Z()/16), 2))
		blockNBT := w.blockNBTs[cp]
		if dist <= radius || radius < 0 {
			w2.storeChunk(cp, dimension, c, blockNBT)
			cf(cp, c)
		} else {
			cf(cp, nil)
		}
	}

	for k, es := range w.entities {
		x := int(es.Position[0])
		z := int(es.Position[2])
		dist := i32.Sqrt(i32.Pow(int32(x-around.X()), 2) + i32.Pow(int32(z-around.Z()), 2))
		if dist < radius*16 || w2.haveEntity(k) || radius < 0 {
			w2.storeEntity(k, es)
		}
	}
}

type worldState struct {
	l             sync.Mutex
	dimension     world.Dimension
	state         *worldStateInternal
	deferredState *worldStateDefer
	storedChunks  map[world.ChunkPos]bool
	chunkFunc     func(world.ChunkPos, *chunk.Chunk)
	useDeferred   bool

	excludeMobs []string
	VoidGen     bool
	timeSync    time.Time
	time        int
	Name        string
	folder      string
	provider    *mcdb.DB
}

func newWorldState(cf func(world.ChunkPos, *chunk.Chunk)) (*worldState, error) {
	w := &worldState{
		state: &worldStateInternal{
			worldStateEnt: worldStateEnt{
				entities:    make(map[EntityRuntimeID]*entityState),
				entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
				blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]dummyBlock),
			},
		},
		storedChunks: make(map[world.ChunkPos]bool),
		chunkFunc:    cf,
	}
	w.state.l = &w.l
	w.initDeferred()
	w.useDeferred = true

	return w, nil
}

func (w *worldState) storeChunk(pos world.ChunkPos, ch *chunk.Chunk, blockNBT map[cube.Pos]dummyBlock) {
	empty := true
	for _, sc := range ch.Sub() {
		if !sc.Empty() {
			empty = false
			break
		}
	}
	if !empty {
		w.storedChunks[pos] = true
	}
	w.State().storeChunk(pos, w.dimension, ch, blockNBT)
}

func (w *worldState) initDeferred() {
	w.deferredState = &worldStateDefer{
		chunks: make(map[world.ChunkPos]*chunk.Chunk),
		worldStateEnt: worldStateEnt{
			entities:    make(map[EntityRuntimeID]*entityState),
			entityLinks: make(map[EntityUniqueID]map[EntityUniqueID]struct{}),
			blockNBTs:   make(map[world.ChunkPos]map[cube.Pos]dummyBlock),
		},
	}
}

func (w *worldState) State() worldStateInt {
	if w.useDeferred {
		return w.deferredState
	}
	return w.state
}

func (w *worldState) PauseCapture() {
	w.initDeferred()
	w.useDeferred = true
}

func (w *worldState) UnpauseCapture(around cube.Pos, radius int32, cf func(world.ChunkPos, *chunk.Chunk)) {
	w.deferredState.ApplyTo(w.state, w.dimension, around, radius, cf)
	w.useDeferred = false
	w.deferredState = nil
}

func (w *worldState) newProvider() error {
	provider, err := mcdb.Config{
		Log:         logrus.StandardLogger(),
		Compression: opt.DefaultCompression,
	}.Open(w.folder)
	if err != nil {
		return err
	}
	w.provider = provider
	w.state.provider = provider
	return nil
}

func (w *worldState) Open(name string, folder string, dim world.Dimension, deferred bool) error {
	w.Name = name
	w.folder = folder
	w.dimension = dim
	os.RemoveAll(folder)
	os.MkdirAll(folder, 0o777)
	err := w.newProvider()
	if err != nil {
		return err
	}

	if !deferred && w.deferredState != nil {
		w.deferredState.ApplyTo(w.state, w.dimension, cube.Pos{}, -1, w.chunkFunc)
		w.useDeferred = false
		w.deferredState = nil
	}

	return nil
}

func (w *worldState) Rename(name, folder string) error {
	w.l.Lock()
	defer w.l.Unlock()
	err := w.provider.Close()
	if err != nil {
		return err
	}
	err = os.Rename(w.folder, folder)
	if err != nil {
		return err
	}
	w.folder = folder
	w.Name = name
	err = w.newProvider()
	if err != nil {
		return err
	}
	return nil
}

func (w *worldState) Finish(playerData map[string]any, spawn cube.Pos, gd minecraft.GameData, bp *behaviourpack.BehaviourPack) error {
	err := w.state.saveEntities(w.excludeMobs, w.dimension)
	if err != nil {
		return err
	}

	err = w.state.saveBlockNBTs(w.dimension)
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
