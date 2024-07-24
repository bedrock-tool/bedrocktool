package merge

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/sirupsen/logrus"
)

type MergeCMD struct {
	f *flag.FlagSet

	offsetsString string
	offsets       []world.ChunkPos
}

func (*MergeCMD) Name() string     { return "merge" }
func (*MergeCMD) Synopsis() string { return "merge worlds" }

func (c *MergeCMD) SetFlags(f *flag.FlagSet) {
	c.f = f
	f.StringVar(&c.offsetsString, "offsets", "", "offsets")
}

func (c *MergeCMD) Execute(ctx context.Context) error {
	if len(c.offsetsString) > 0 {
		offsetParts := strings.Split(c.offsetsString, ";")
		for _, part := range offsetParts {
			xyz := strings.Split(part, ",")
			var offset world.ChunkPos
			x, err := strconv.Atoi(xyz[0])
			if err != nil {
				return err
			}
			z, err := strconv.Atoi(xyz[1])
			if err != nil {
				return err
			}
			offset[0] = int32(x)
			offset[1] = int32(z)
			c.offsets = append(c.offsets, offset)
		}
	}

	blockReg := blockRegistry{
		BlockRegistry: world.DefaultBlockRegistry,
		rids:          make(map[uint32]block),
	}
	entityReg := &EntityRegistry{}

	var worlds []*mcdb.DB
	for _, worldName := range c.f.Args() {
		sp := strings.SplitN(worldName, ";", 3)
		worldName = sp[0]
		var offset world.ChunkPos
		if len(sp) == 3 {
			x, err := strconv.Atoi(sp[1])
			if err != nil {
				return err
			}
			z, err := strconv.Atoi(sp[2])
			if err != nil {
				return err
			}
			offset[0] = int32(x)
			offset[1] = int32(z)
		}
		c.offsets = append(c.offsets, offset)
		db, err := mcdb.Config{
			Log:      logrus.StandardLogger(),
			ReadOnly: true,
			Blocks:   blockReg,
			Entities: entityReg,
		}.Open(worldName)
		if err != nil {
			return err
		}
		defer db.Close()
		worlds = append(worlds, db)
	}

	os.RemoveAll("merge_out")
	dbOut, err := mcdb.Config{
		Log:      logrus.StandardLogger(),
		Blocks:   blockReg,
		Entities: entityReg,
	}.Open("merge_out")
	if err != nil {
		return err
	}
	defer dbOut.Close()

	for i, db := range worlds {
		err = c.processWorld(db, dbOut, c.offsets[i])
		if err != nil {
			return err
		}
	}

	ldat := worlds[0].LevelDat()
	*dbOut.LevelDat() = *ldat

	return nil
}

func (c *MergeCMD) processWorld(db *mcdb.DB, out *mcdb.DB, offset world.ChunkPos) error {
	it := db.NewColumnIterator(nil, false)
	defer it.Release()
	for it.Next() {
		column := it.Column()
		pos := it.Position()
		dim := it.Dimension()
		pos[0] += offset[0]
		pos[1] += offset[1]
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
