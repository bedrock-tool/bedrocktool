package worlds

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

// TestRenderStates lays the page out in each state, and with
// RENDER_OUT_DIR set saves what it looks like.
func TestRenderStates(t *testing.T) {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	dark := th.WithPalette(material.Palette{
		Bg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
		Fg:         color.NRGBA{227, 227, 227, 0xff},
		ContrastBg: color.NRGBA{142, 49, 235, 0xff},
		ContrastFg: color.NRGBA{227, 227, 227, 0xff},
	})

	for _, tc := range []struct {
		name  string
		setup func(p *Page)
	}{
		{"waiting", func(p *Page) {
			p.HandleEvent(&messages.EventSetUIState{State: messages.UIStateMain})
		}},
		{"saving", func(p *Page) {
			p.HandleEvent(&messages.EventSetUIState{State: messages.UIStateMain})
			p.HandleEvent(&messages.EventProcessingWorldUpdate{WorldName: "Magic Realm", State: "Adding Resourcepack Furniture Pack"})
		}},
		{"saved", func(p *Page) {
			p.HandleEvent(&messages.EventSetUIState{State: messages.UIStateFinished})
			for i, n := range []int{9, 9, 303, 50, 9, 99} {
				name := "world-" + string(rune('1'+i))
				p.HandleEvent(&messages.EventFinishedSavingWorld{WorldName: name, Filepath: "worlds/HIVE-O-WEEN _ BEDWARS S5-2026-11-13_03-16-39.pcap2/" + name + ".mcworld", Chunks: n * 25, Entities: n})
			}
		}},
	} {
		p := New(nil).(*Page)
		tc.setup(p)
		size := image.Pt(800, 640)
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Constraints: layout.Exact(size), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
		paint.Fill(ops, dark.Bg)
		p.Layout(gtx, &dark)

		dir := os.Getenv("RENDER_OUT_DIR")
		if dir == "" {
			continue
		}
		win, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			t.Skip(err)
		}
		win.Frame(ops)
		img := image.NewRGBA(image.Rectangle{Max: size})
		win.Screenshot(img)
		win.Release()
		f, _ := os.Create(filepath.Join(dir, "worlds_"+tc.name+".png"))
		png.Encode(f, img)
		f.Close()
	}
}
