package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/google/subcommands"
	"github.com/jinzhu/copier"
)

type MergeCMD struct {
	worlds []string
	legacy bool
}

func (*MergeCMD) Name() string     { return "merge" }
func (*MergeCMD) Synopsis() string { return "merge 2 or more worlds" }

func (c *MergeCMD) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.legacy, "legacy", false, "if the worlds are before 1.18")
}
func (c *MergeCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *MergeCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if f.NArg() == 0 {
		fmt.Println("you need to specify 1 or more worlds")
		return 1
	}
	c.worlds = f.Args()
	out_name := c.worlds[0] + "-merged"

	prov_out, err := mcdb.New(out_name, opt.DefaultCompression)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open output %s\n", err)
	}

	for i, world_name := range c.worlds {
		first := i == 0
		fmt.Printf("Adding %s\n", world_name)
		s, err := os.Stat(world_name)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "%s not found\n", world_name)
		}
		if !s.IsDir() { // if its a zip temporarily unpack it to read it
			f, _ := os.Open(world_name)
			world_name += "_unpack"
			unpack_zip(f, s.Size(), world_name)
		}
		// merge it into the state
		err = c.merge_worlds(prov_out, world_name, first)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", world_name, err)
		}
		if !s.IsDir() { // remove temp folder again
			os.RemoveAll(world_name)
		}
	}

	if err = prov_out.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	time.Sleep(1 * time.Second)

	if err := zip_folder(out_name+".mcworld", out_name); err != nil {
		fmt.Fprintf(os.Stderr, "zipping: %s\n", err)
		return 1
	}

	os.RemoveAll(out_name)
	return 0
}

func (c *MergeCMD) merge_worlds(prov_out *mcdb.Provider, folder string, first bool) error {
	prov_in, err := mcdb.New(folder, opt.DefaultCompression)
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
			entities, err := prov_in.LoadEntities(i.P, i.D)
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
		fmt.Print("Applying Settings, level.dat\n\n")
		prov_out.SaveSettings(prov_in.Settings())
		out_ld := prov_out.LevelDat()
		copier.Copy(out_ld, prov_in.LevelDat())
	}
	fmt.Printf("Added: %d\n", count)
	return nil
}

func init() {
	register_command(&MergeCMD{})
}
