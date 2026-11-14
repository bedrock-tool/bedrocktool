package subcommands

import (
	"context"
	"testing"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

func TestWorldsGeneratorFlags(t *testing.T) {
	cases := []struct {
		args []string
		want worldstate.Generator
	}{
		{nil, worldstate.GeneratorVoid},
		{[]string{"-void=false"}, worldstate.GeneratorFlat},
		{[]string{"-generator", "flat"}, worldstate.GeneratorFlat},
		{[]string{"-generator", "normal"}, worldstate.GeneratorNormal},
		{[]string{"-void=false", "-generator", "normal"}, worldstate.GeneratorNormal},
	}
	for _, c := range cases {
		args := append([]string{"-address", "example.com"}, c.args...)
		settings, _, err := commands.ParseArgs(context.Background(), WorldCMD{}, args)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		got, err := settings.(*WorldSettings).generator()
		if err != nil || got != c.want {
			t.Errorf("%v: got %q, %v; want %q", c.args, got, err, c.want)
		}
	}

	if _, _, err := commands.ParseArgs(context.Background(), WorldCMD{}, []string{"-address", "x", "-generator", "amplified"}); err == nil {
		t.Error("-generator amplified: want an error")
	}
}
