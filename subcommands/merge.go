//go:build false

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
	outName := c.worlds[0] + "-merged"

	provOut, err := mcdb.New(logrus.StandardLogger(), outName, opt.DefaultCompression)
	if err != nil {
		logrus.Errorf(locale.Loc("failed_to_open_output", locale.Strmap{"Err": err}))
		return 1
	}

	for i, worldName := range c.worlds {
		first := i == 0
		logrus.Infof(locale.Loc("adding_world", locale.Strmap{"World": worldName}))
		s, err := os.Stat(worldName)
		if errors.Is(err, os.ErrNotExist) {
			logrus.Fatalf(locale.Loc("not_found", locale.Strmap{"Name": worldName}), worldName)
		}
		if !s.IsDir() { // if its a zip temporarily unpack it to read it
			f, _ := os.Open(worldName)
			worldName += "_unpack"
			utils.UnpackZip(f, s.Size(), worldName)
		}
		// merge it into the state
		err = c.mergeWorlds(provOut, worldName, first)
		if err != nil {
			logrus.Errorf("%s %s", worldName, err)
			return 1
		}
		if !s.IsDir() { // remove temp folder again
			os.RemoveAll(worldName)
		}
	}

	if err = provOut.Close(); err != nil {
		logrus.Error(err)
		return 1
	}
	time.Sleep(1 * time.Second)

	if err := utils.ZipFolder(outName+".mcworld", outName); err != nil {
		logrus.Infof("zipping: %s", err)
		return 1
	}

	os.RemoveAll(outName)
	return 0
}

func (c *MergeCMD) mergeWorlds(provOut *mcdb.Provider, folder string, first bool) error {
	provIn, err := mcdb.New(logrus.StandardLogger(), folder, opt.DefaultCompression)
	if err != nil {
		return err
	}
	count := 0
	existing := provOut.Chunks(c.legacy)
	new := provIn.Chunks(c.legacy)
	for i := range new {
		if _, ok := existing[i]; !ok {
			d := i.D
			// chunks
			ch, _, err := provIn.LoadChunk(i.P, d)
			if err != nil {
				return err
			}
			if err := provOut.SaveChunk(i.P, ch, i.D); err != nil {
				return err
			}

			// blockNBT
			n, err := provIn.LoadBlockNBT(i.P, i.D)
			if err != nil {
				return err
			}
			if err := provOut.SaveBlockNBT(i.P, n, i.D); err != nil {
				return err
			}

			// entities
			entities, err := provIn.LoadEntities(i.P, i.D, entity.DefaultRegistry)
			if err != nil {
				return err
			}
			if err := provOut.SaveEntities(i.P, entities, i.D); err != nil {
				return err
			}
			count += 1
		}
	}

	if first {
		logrus.Debug("Applying Settings and level.dat")
		provOut.SaveSettings(provIn.Settings())
		outLd := provOut.LevelDat()
		copier.Copy(outLd, provIn.LevelDat())
	}
	logrus.Infof("Added: %d", count)
	return nil
}

func init() {
	utils.RegisterCommand(&MergeCMD{})
}
