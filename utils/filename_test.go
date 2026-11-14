package utils

import "testing"

func TestMakeValidFilename(t *testing.T) {
	for in, want := range map[string]string{
		"Wilderness":              "Wilderness",
		"§aEons§r | §0Black Hole": "_Eons_ _ _Black Hole",
		"world name ":             "world name",
		"Spawn...":                "Spawn",
		"Hub . ":                  "Hub",
		"play.example.net:19132":  "play.example.net_19132",
		"con":                     "_con",
		"NUL.mcworld":             "_NUL.mcworld",
		"Console":                 "Console",
		"...":                     "_",
		"":                        "_",
	} {
		if got := MakeValidFilename(in); got != want {
			t.Errorf("MakeValidFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
