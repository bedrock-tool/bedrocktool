package settings

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	_ "github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

func TestRenderSettings(t *testing.T) {
	out := os.Getenv("RENDER_OUT")
	if out == "" {
		t.Skip()
	}
	sz := image.Pt(900, 1100)
	win, err := headless.NewWindow(sz.X, sz.Y)
	if err != nil {
		t.Skip(err)
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	pal := material.Palette{Bg: color.NRGBA{0x12, 0x12, 0x12, 0xff}, Fg: color.NRGBA{227, 227, 227, 0xff}, ContrastBg: color.NRGBA{142, 49, 235, 0xff}, ContrastFg: color.NRGBA{227, 227, 227, 0xff}}
	th2 := th.WithPalette(pal)
	s := &settingsPage{cmd: commands.Registered["worlds"]}
	s.Init()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(sz), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	paint.Fill(ops, pal.Bg)
	s.Layout(gtx, &th2)
	win.Frame(ops)
	img := image.NewRGBA(image.Rectangle{Max: sz})
	win.Screenshot(img)
	f, _ := os.Create(out)
	png.Encode(f, img)
	f.Close()
}

func TestRenderModePicker(t *testing.T) {
	out := os.Getenv("RENDER_OUT_DIR")
	sz := image.Pt(800, 600)
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	pal := material.Palette{Bg: color.NRGBA{0x12, 0x12, 0x12, 0xff}, Fg: color.NRGBA{227, 227, 227, 0xff}, ContrastBg: color.NRGBA{142, 49, 235, 0xff}, ContrastFg: color.NRGBA{227, 227, 227, 0xff}}
	th2 := th.WithPalette(pal)
	p := New(nil)
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(sz), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	paint.Fill(ops, pal.Bg)
	p.Layout(gtx, &th2)
	if out == "" {
		return
	}
	win, err := headless.NewWindow(sz.X, sz.Y)
	if err != nil {
		t.Skip(err)
	}
	win.Frame(ops)
	img := image.NewRGBA(image.Rectangle{Max: sz})
	win.Screenshot(img)
	f, _ := os.Create(out + "/modes.png")
	png.Encode(f, img)
	f.Close()
}
