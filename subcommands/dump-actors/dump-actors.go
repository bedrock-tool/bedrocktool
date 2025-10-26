package dumpactors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/df-mc/goleveldb/leveldb/util"
	"github.com/sirupsen/logrus"
)

type DumpActorsSettings struct {
	WorldPath string `opt:"World Folder" flag:"world"`
	Write     bool   `opt:"Write Back" flag:"write"`
	JsonPath  string `opt:"json file" flag:"json" default:"actors.json"`
}

type DumpActorsCMD struct {
}

func (DumpActorsCMD) Name() string {
	return "dump-actors"
}

func (DumpActorsCMD) Description() string {
	return "dump and write actors"
}

func (DumpActorsCMD) Settings() any {
	return new(DumpActorsSettings)
}

func (c DumpActorsCMD) Run(ctx context.Context, settings any) error {
	dumpSettings := settings.(*DumpActorsSettings)

	if dumpSettings.WorldPath == "" {
		return fmt.Errorf("-world missing")
	}
	if dumpSettings.JsonPath == "" {
		return fmt.Errorf("-json missing")
	}

	if dumpSettings.Write {
		return writeActorsBackToWorld(dumpSettings.WorldPath, dumpSettings.JsonPath)
	} else {
		return extractActorsToJson(dumpSettings.WorldPath, dumpSettings.JsonPath)
	}
}

func worldGetAllActors(world *mcdb.DB) ([]Actor, error) {
	var actors []Actor
	it := world.NewColumnIterator(nil)
	defer it.Release()
	for it.Next() {
		c := it.Column()
		if err := it.Error(); err != nil {
			return nil, err
		}
		for _, e := range c.Entities {
			actors = append(actors, Actor{e: e})
		}
	}
	return actors, nil
}

func extractActorsToJson(worldPath, jsonPath string) error {
	inputWorld, err := mcdb.Config{
		Blocks: &merge.BlockRegistry{
			BlockRegistry: world.DefaultBlockRegistry,
			Rids:          make(map[uint32]merge.Block),
		},
		LDBOptions: &opt.Options{
			ReadOnly: true,
		},
	}.Open(worldPath)
	if err != nil {
		return err
	}
	defer inputWorld.Close()

	actors, err := worldGetAllActors(inputWorld)
	if err != nil {
		return err
	}

	f, err := os.Create(jsonPath)
	if err != nil {
		return err
	}
	defer f.Close()
	je := json.NewEncoder(f)
	je.SetIndent("", "  ")
	if err = je.Encode(ActorsJson{
		SourceWorld: worldPath,
		Actors:      actors,
	}); err != nil {
		return err
	}
	logrus.Infof("Extracted %d actors to json", len(actors))
	return nil
}

func backupWorldDB(worldPath string) error {
	dbPath := path.Join(worldPath, "db")
	return utils.CopyFS(os.DirFS(dbPath), utils.OSWriter{
		Base: dbPath + ".bak",
	})
}

func removeAllDigp(ldb *leveldb.DB) {
	var count int
	it := ldb.NewIterator(util.BytesPrefix([]byte("digp")), nil)
	defer it.Release()
	for it.Next() {
		ldb.Delete(it.Key(), nil)
		count++
	}
	err := it.Error()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("removed %d digp entries", count)
}

func removeAllActorprefix(ldb *leveldb.DB) {
	var count int
	it := ldb.NewIterator(util.BytesPrefix([]byte("actorprefix")), nil)
	defer it.Release()
	for it.Next() {
		ldb.Delete(it.Key(), nil)
		count++
	}
	err := it.Error()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("removed %d actors", count)
}

func writeActorsBackToWorld(worldPath, jsonPath string) error {
	// read actors json
	f, err := os.Open(jsonPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()
	jd := json.NewDecoder(f)
	var aj ActorsJson
	if err := jd.Decode(&aj); err != nil {
		logrus.Fatal(err)
	}

	// split actors by chunk
	var actorsByChunk = make(map[world.ChunkPos][]chunk.Entity)
	for _, actor := range aj.Actors {
		pos := actor.e.Data["Pos"].([]float32)
		cp := world.ChunkPos{
			int32(pos[0]) >> 4,
			int32(pos[2]) >> 4,
		}
		actorsByChunk[cp] = append(actorsByChunk[cp], actor.e)
	}
	logrus.Infof("writing actors in %d chunks", len(actorsByChunk))

	// backup leveldb
	err = backupWorldDB(worldPath)
	if err != nil {
		return err
	}

	// open the world
	outputWorld, err := mcdb.Config{
		Blocks: &merge.BlockRegistry{
			BlockRegistry: world.DefaultBlockRegistry,
			Rids:          make(map[uint32]merge.Block),
		},
		LDBOptions: &opt.Options{
			ReadOnly: false,
		},
	}.Open(worldPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer outputWorld.Close()

	// remove all entities
	ldb := outputWorld.LDB()
	removeAllDigp(ldb)
	removeAllActorprefix(ldb)

	// store new entities
	var count int
	for cp, actors := range actorsByChunk {
		outputWorld.StoreEntities(cp, world.Overworld, actors)
		count += len(actors)
	}
	logrus.Infof("wrote %d actors", count)
	return nil
}

func init() {
	commands.RegisterCommand(&DumpActorsCMD{})
}
