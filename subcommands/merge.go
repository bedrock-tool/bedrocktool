package subcommands

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/merge"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
)

type MergeSettings struct {
	Bounds      bool     `opt:"Show Bounds" flag:"bounds"`
	OutPath     string   `opt:"Out Path" flag:"out"`
	InputWorlds []string `opt:"Input Worlds" flag:"-args"`
}

type MergeCMD struct{}

func (MergeCMD) Name() string {
	return "merge"
}

func (MergeCMD) Description() string {
	return "merge worlds"
}

func (MergeCMD) Settings() any {
	return new(MergeSettings)
}

type worldInstance struct {
	Name   string
	db     *mcdb.DB
	offset world.ChunkPos
}

func (c MergeCMD) Run(ctx context.Context, settings any) error {
	mergeSettings := settings.(*MergeSettings)
	if mergeSettings.OutPath == "" && !mergeSettings.Bounds {
		return fmt.Errorf("-out must be specified")
	}

	blockReg := &merge.BlockRegistry{
		BlockRegistry: world.DefaultBlockRegistry,
		Rids:          make(map[uint32]merge.Block),
	}

	var worlds []worldInstance
	for _, worldName := range mergeSettings.InputWorlds {
		sp := strings.SplitN(worldName, ";", 3)
		worldName = sp[0]
		var offset world.ChunkPos
		if len(sp) == 3 {
			x, err := strconv.Atoi(sp[1])
			if err != nil {
				return fmt.Errorf("%s %w", worldName, err)
			}
			z, err := strconv.Atoi(sp[2])
			if err != nil {
				return fmt.Errorf("%s %w", worldName, err)
			}
			offset[0] = int32(x)
			offset[1] = int32(z)
		}
		db, err := mcdb.Config{
			Log:    slog.Default(),
			Blocks: blockReg,
			LDBOptions: &opt.Options{
				ReadOnly: true,
			},
		}.Open(utils.PathData(worldName))
		if err != nil {
			return fmt.Errorf("%s %w", worldName, err)
		}
		defer db.Close()
		worlds = append(worlds, worldInstance{Name: worldName, db: db, offset: offset})
	}

	if mergeSettings.Bounds {
		for _, w := range worlds {
			fmt.Printf("\n%s\n", w.Name)
			minBound, maxBound, err := w.getWorldBounds()
			if err != nil {
				return err
			}
			fmt.Printf("Min: %d,%d Max: %d,%d\n", minBound[0], minBound[1], maxBound[0], maxBound[1])
		}
		return nil
	}

	outPath := utils.PathData(mergeSettings.OutPath)

	if _, err := os.Stat(outPath + "/level.dat"); err == nil {
		err = os.RemoveAll(outPath)
		if err != nil {
			return err
		}
	}
	dbOut, err := mcdb.Config{
		Log:    slog.Default(),
		Blocks: blockReg,
	}.Open(outPath)
	if err != nil {
		return err
	}
	defer dbOut.Close()

	for _, w := range worlds {
		err = c.processWorld(w.db, dbOut, w.offset)
		if err != nil {
			return err
		}
	}

	ldat := worlds[0].db.LevelDat()
	*dbOut.LevelDat() = *ldat

	return nil
}

func (w worldInstance) getWorldBounds() (minChunk, maxChunk world.ChunkPos, err error) {
	minChunk = world.ChunkPos{math.MaxInt32, math.MaxInt32}
	maxChunk = world.ChunkPos{math.MinInt32, math.MinInt32}
	it := w.db.NewColumnIterator(nil)
	defer it.Release()
	for it.Next() {
		pos := it.Position()
		minChunk[0] = min(minChunk[0], pos[0])
		minChunk[1] = min(minChunk[1], pos[1])
		maxChunk[0] = max(maxChunk[0], pos[0])
		maxChunk[1] = max(maxChunk[1], pos[1])
	}

	if err := it.Error(); err != nil {
		return minChunk, maxChunk, err
	}

	return
}

func (c *MergeCMD) processWorld(db *mcdb.DB, out *mcdb.DB, offset world.ChunkPos) error {
	it := db.NewColumnIterator(nil)
	defer it.Release()
	for it.Next() {
		column := it.Column()
		pos := it.Position()
		dim := it.Dimension()
		pos[0] += offset[0]
		pos[1] += offset[1]
		for _, ent := range column.Entities {
			pos := ent.Data["Pos"].([]any)
			x := pos[0].(float32)
			y := pos[1].(float32)
			z := pos[2].(float32)
			ent.Data["Pos"] = []any{
				x + float32(offset[0]*16),
				y,
				z + float32(offset[1]*16),
			}
			ent.Data["UniqueID"] = rand.Int64()
		}
		err := out.StoreColumn(pos, dim, column)
		if err != nil {
			return err
		}
	}
	if err := it.Error(); err != nil {
		return err
	}
	return nil
}

func init() {
	commands.RegisterCommand(&MergeCMD{})
}
