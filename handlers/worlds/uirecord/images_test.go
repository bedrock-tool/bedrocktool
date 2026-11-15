package uirecord

import (
	"strings"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestImagePath(t *testing.T) {
	for _, c := range []struct {
		url, want string
		ok        bool
	}{
		{"https://cdn.playhive.com/icons/locker/build/cape/sweet-treat.png", "icons/locker/build/cape/sweet-treat", true},
		{"https://cdn.playhive.com/icons/locker/locked.png?v=2", "icons/locker/locked", true},
		{"https://example.com/a/../../b.png", "b", true},
		{"file:///etc/passwd", "", false},
		{"textures/ui/x", "", false},
		{"https://cdn.playhive.com/", "", false},
		{"https://cdn.playhive.com/a%20b.png", "", false},
	} {
		got, ok := ImagePath(c.url)
		if got != c.want || ok != c.ok {
			t.Errorf("ImagePath(%q) = %q, %v; want %q, %v", c.url, got, ok, c.want, c.ok)
		}
	}
}

func TestScriptShowsURLImages(t *testing.T) {
	r := New(nil)
	r.forms["f1"] = &Form{Key: "f1", Data: map[string]any{
		"type":  "form",
		"title": "PETS§m§8",
		"buttons": []any{
			map[string]any{"text": "Cape", "image": map[string]any{"type": "url", "data": "https://cdn.playhive.com/icons/locker/build/cape/sweet-treat.png"}},
			map[string]any{"text": "Vanilla", "image": map[string]any{"type": "path", "data": "textures/ui/x"}},
			map[string]any{"text": "None"},
		},
	}}
	r.formOrder = append(r.formOrder, "f1")

	images := r.Images()
	if len(images) != 1 || images["https://cdn.playhive.com/icons/locker/build/cape/sweet-treat.png"] != "icons/locker/build/cape/sweet-treat" {
		t.Errorf("Images() = %v", images)
	}
	script := r.Script()
	for _, s := range []string{
		`const IMAGES = {`,
		`"https://cdn.playhive.com/icons/locker/build/cape/sweet-treat.png": "icons/locker/build/cape/sweet-treat"`,
		`IMAGES[e.image.data]`,
		`PETS§m§8`,
	} {
		if !strings.Contains(script, s) {
			t.Errorf("script lacks %s", s)
		}
	}
}

// Forms listing their buttons in "elements" (Hive's cosmetics menu) are
// built from them, and their images are saved.
func TestElementsForm(t *testing.T) {
	r := New(nil)
	r.forms["c"] = &Form{Key: "c", Data: map[string]any{
		"type":    "form",
		"title":   "COSMETICS§m§7",
		"content": "39 / 127",
		"elements": []any{
			map[string]any{"type": "header", "text": "Featured"},
			map[string]any{"type": "button", "text": "Pets", "image": map[string]any{"type": "url", "data": "https://cdn.playhive.com/renders/pet/x.png"}},
			map[string]any{"type": "divider", "text": ""},
			map[string]any{"text": "Hats"},
		},
	}}
	r.formOrder = append(r.formOrder, "c")

	if got := r.Images()["https://cdn.playhive.com/renders/pet/x.png"]; got != "renders/pet/x" {
		t.Errorf("element image path = %q", got)
	}
	script := r.Script()
	for _, s := range []string{"f.elements ?? f.buttons", `case "header"`, `case "divider"`} {
		if !strings.Contains(script, s) {
			t.Errorf("script lacks %s", s)
		}
	}
}

// Each saved world keeps its own scoreboard, by its dimension: a game's
// scoreboard doesn't end up on the hub.
func TestScoreboardPerWorld(t *testing.T) {
	r := New(nil)
	board := func(title string) {
		r.OnPacket(&packet.SetDisplayObjective{DisplaySlot: "sidebar", ObjectiveName: "side", DisplayName: title}, false, time.Now())
	}
	board("HUB")
	r.FinishWorld("")
	r.FinishWorld("bedrocktool:world_5")
	if got := r.boards["bedrocktool:world_5"]; len(got) != 1 || got[0].Slots["sidebar"] != 0 || len(got[0].Slots) != 1 {
		t.Errorf("objective shown in the last world isn't shown in the next: %+v", got)
	}
	board("JUST BUILD")
	r.FinishWorld("bedrocktool:world_6")

	if got := r.boards[""]; len(got) != 1 || got[0].DisplayName != "HUB" {
		t.Errorf("main world's board: %+v", got)
	}
	if got := r.boards["bedrocktool:world_6"]; len(got) != 1 || got[0].DisplayName != "JUST BUILD" {
		t.Errorf("world_6's board: %+v", got)
	}
	script := r.Script()
	for _, s := range []string{"const SCOREBOARDS", `"bedrocktool:world_6"`, "playerDimensionChange"} {
		if !strings.Contains(script, s) {
			t.Errorf("script lacks %s", s)
		}
	}
}
