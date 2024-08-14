package merge

import (
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/sirupsen/logrus"
)

type MergeCMD struct {
	f          *flag.FlagSet
	showBounds bool
	outPath    string
}

type worldInstance struct {
	Name   string
	db     *mcdb.DB
	offset world.ChunkPos
}

func (*MergeCMD) Name() string     { return "merge" }
func (*MergeCMD) Synopsis() string { return "merge worlds" }

func (c *MergeCMD) SetFlags(f *flag.FlagSet) {
	c.f = f
	f.BoolVar(&c.showBounds, "bounds", false, "show bounds instead of merge")
	f.StringVar(&c.outPath, "out", "", "output folder")
}

func (c *MergeCMD) Execute(ctx context.Context) error {
	if c.outPath == "" && !c.showBounds {
		return fmt.Errorf("-out must be specified")
	}

	blockReg := &BlockRegistry{
		BlockRegistry: world.DefaultBlockRegistry,
		Rids:          make(map[uint32]Block),
	}
	entityReg := &EntityRegistry{}

	var worlds []worldInstance
	for _, worldName := range c.f.Args() {
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
			Log:      logrus.StandardLogger(),
			ReadOnly: true,
			Blocks:   blockReg,
			Entities: entityReg,
		}.Open(worldName)
		if err != nil {
			return fmt.Errorf("%s %w", worldName, err)
		}
		defer db.Close()
		worlds = append(worlds, worldInstance{Name: worldName, db: db, offset: offset})
	}

	if c.showBounds {
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

	if _, err := os.Stat(c.outPath + "/level.dat"); err == nil {
		err = os.RemoveAll(c.outPath)
		if err != nil {
			return err
		}
	}
	dbOut, err := mcdb.Config{
		Log:      logrus.StandardLogger(),
		Blocks:   blockReg,
		Entities: entityReg,
	}.Open(c.outPath)
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
	it := w.db.NewColumnIterator(nil, false)
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

var ids = map[int64]*DummyEntity{}

func (c *MergeCMD) processWorld(db *mcdb.DB, out *mcdb.DB, offset world.ChunkPos) error {
	it := db.NewColumnIterator(nil, false)
	defer it.Release()
	for it.Next() {
		column := it.Column()
		pos := it.Position()
		dim := it.Dimension()
		pos[0] += offset[0]
		pos[1] += offset[1]
		for _, ent := range column.Entities {
			ent := ent.(*DummyEntity)
			t := ent.T.(*DummyEntityType)
			pos := t.NBT["Pos"].([]any)
			x := pos[0].(float32)
			y := pos[1].(float32)
			z := pos[2].(float32)
			t.NBT["Pos"] = []any{
				x + float32(offset[0]*16),
				y,
				z + float32(offset[1]*16),
			}
			t.NBT["UniqueID"] = rand.Int64()
			UniqueID := t.NBT["UniqueID"].(int64)
			ent2, ok := ids[UniqueID]
			if ok {
				fmt.Printf("conflict 0x%016x %s %s\n", UniqueID, ent, ent2)
			}
			ids[UniqueID] = ent
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
