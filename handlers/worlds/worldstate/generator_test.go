package worldstate

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world/mcdb"
)

// TestGeneratorLevelDat writes level.dat with each generator and reads it
// back, as Minecraft would.
func TestGeneratorLevelDat(t *testing.T) {
	cases := []struct {
		gen       Generator
		generator int32
		layers    string
	}{
		{GeneratorVoid, 2, voidLayers},
		{GeneratorFlat, 2, ""},
		{GeneratorNormal, 1, ""},
	}
	for _, c := range cases {
		t.Run(string(c.gen), func(t *testing.T) {
			dir := t.TempDir()
			db, err := mcdb.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			ld := db.LevelDat()
			ld.RandomSeed = 12345
			applyGenerator(ld, c.gen)
			db.SaveSettings(db.Settings())
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			db, err = mcdb.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ld = db.LevelDat()
			if ld.Generator != c.generator {
				t.Errorf("Generator = %d, want %d", ld.Generator, c.generator)
			}
			if ld.FlatWorldLayers != c.layers {
				t.Errorf("FlatWorldLayers = %q, want %q", ld.FlatWorldLayers, c.layers)
			}
			if ld.RandomSeed != 12345 {
				t.Errorf("RandomSeed = %d, want the server's seed kept", ld.RandomSeed)
			}
		})
	}
}

func TestParseGenerator(t *testing.T) {
	for in, want := range map[string]Generator{
		"":        GeneratorVoid,
		"void":    GeneratorVoid,
		" Flat ":  GeneratorFlat,
		"NORMAL":  GeneratorNormal,
		"default": "",
	} {
		got, err := ParseGenerator(in)
		if want == "" {
			if err == nil {
				t.Errorf("ParseGenerator(%q) = %q, want an error", in, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("ParseGenerator(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
