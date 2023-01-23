package subcommands

import (
	"context"
	"errors"
	"flag"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/google/subcommands"
	"github.com/jinzhu/copier"
	"github.com/sirupsen/logrus"
)

type MergeCMD struct {
	worlds []string
	legacy bool
}

func (*MergeCMD) Name() string     { return "merge" }
func (*MergeCMD) Synopsis() string { return locale.Loc("merge_synopsis", nil) }

func (c *MergeCMD) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.legacy, "legacy", false, "if the worlds are before 1.18")
}

func (c *MergeCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *MergeCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if f.NArg() == 0 {
		logrus.Error(locale.Loc("need_to_specify_multiple_worlds", nil))
		return 1
	}
	c.worlds = f.Args()
	out_name := c.worlds[0] + "-merged"

	prov_out, err := mcdb.New(logrus.StandardLogger(), out_name, opt.DefaultCompression)
	if err != nil {
		logrus.Errorf(locale.Loc("failed_to_open_output", locale.Strmap{"Err": err}))
		return 1
	}

	for i, world_name := range c.worlds {
		first := i == 0
		logrus.Infof(locale.Loc("adding_world", locale.Strmap{"World": world_name}))
		s, err := os.Stat(world_name)
		if errors.Is(err, os.ErrNotExist) {
			logrus.Fatalf(locale.Loc("not_found", locale.Strmap{"Name": world_name}), world_name)
		}
		if !s.IsDir() { // if its a zip temporarily unpack it to read it
			f, _ := os.Open(world_name)
			world_name += "_unpack"
			utils.UnpackZip(f, s.Size(), world_name)
		}
		// merge it into the state
		err = c.merge_worlds(prov_out, world_name, first)
		if err != nil {
			logrus.Errorf("%s %s", world_name, err)
			return 1
		}
		if !s.IsDir() { // remove temp folder again
			os.RemoveAll(world_name)
		}
	}

	if err = prov_out.Close(); err != nil {
		logrus.Error(err)
		return 1
	}
	time.Sleep(1 * time.Second)

	if err := utils.ZipFolder(out_name+".mcworld", out_name); err != nil {
		logrus.Infof("zipping: %s", err)
		return 1
	}

	os.RemoveAll(out_name)
	return 0
}

func (c *MergeCMD) merge_worlds(prov_out *mcdb.Provider, folder string, first bool) error {
	prov_in, err := mcdb.New(logrus.StandardLogger(), folder, opt.DefaultCompression)
	if err != nil {
		return err
	}
	count := 0
	existing := prov_out.Chunks(c.legacy)
	new := prov_in.Chunks(c.legacy)
	for i := range new {
		if _, ok := existing[i]; !ok {
			d := i.D
			// chunks
			ch, _, err := prov_in.LoadChunk(i.P, d)
			if err != nil {
				return err
			}
			if err := prov_out.SaveChunk(i.P, ch, i.D); err != nil {
				return err
			}

			// blockNBT
			n, err := prov_in.LoadBlockNBT(i.P, i.D)
			if err != nil {
				return err
			}
			if err := prov_out.SaveBlockNBT(i.P, n, i.D); err != nil {
				return err
			}

			// entities
			entities, err := prov_in.LoadEntities(i.P, i.D, entity.DefaultRegistry)
			if err != nil {
				return err
			}
			if err := prov_out.SaveEntities(i.P, entities, i.D); err != nil {
				return err
			}
			count += 1
		}
	}

	if first {
		logrus.Debug("Applying Settings and level.dat")
		prov_out.SaveSettings(prov_in.Settings())
		out_ld := prov_out.LevelDat()
		copier.Copy(out_ld, prov_in.LevelDat())
	}
	logrus.Infof("Added: %d", count)
	return nil
}

func init() {
	utils.RegisterCommand(&MergeCMD{})
}
