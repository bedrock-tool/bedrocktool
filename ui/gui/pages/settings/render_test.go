package settings

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	_ "github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
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

type popupRecorder struct {
	guim.Guim
	shown  []any
	closed []string
}

func (p *popupRecorder) ShowPopup(popup any) { p.shown = append(p.shown, popup) }
func (p *popupRecorder) Invalidate()         {}

// Switching Record Session off asks first, and leaves it on until the
// user confirms.
func TestCaptureOffAsks(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "dismissed")
	captureWarningFile = func() string { return marker }
	g := &popupRecorder{}
	s := &settingsPage{cmd: commands.Registered["worlds"], g: g}
	s.Init()
	b := s.widgets["capture"].widget.(*widget.Bool)
	if !b.Value {
		t.Fatal("Record Session should be on by default")
	}
	b.Value = false // what the click did
	s.onToggle("capture", b)
	if !b.Value || len(g.shown) != 1 {
		t.Fatalf("turning it off should ask first and stay on: on=%v popups=%d", b.Value, len(g.shown))
	}
	confirm := g.shown[0].(*popups.ConfirmPopup)
	confirm.Close() // closed without answering
	if !b.Value {
		t.Fatal("closing the question should keep recording on")
	}

	b.Value = false
	s.onToggle("capture", b)
	g.shown[1].(*popups.ConfirmPopup).Answer(true)
	if b.Value {
		t.Fatal("confirming should turn it off")
	}

	// "don't show again": the next time it turns off without asking
	b.Value = true
	b.Value = false
	s.onToggle("capture", b)
	if b.Value || len(g.shown) != 2 {
		t.Fatalf("after turning it off anyway it shouldn't ask again: on=%v popups=%d", b.Value, len(g.shown))
	}

	// other options toggle freely
	img := s.widgets["image"].widget.(*widget.Bool)
	img.Value = true
	s.onToggle("image", img)
	if !img.Value || len(g.shown) != 2 {
		t.Fatal("other options shouldn't ask")
	}
}

func TestUpperFirst(t *testing.T) {
	for in, want := range map[string]string{
		"debug mode": "Debug mode", "écrire": "Écrire", "чем": "Чем", "虚空": "虚空", "": "", "x": "X",
	} {
		if got := upperFirst(in); got != want {
			t.Errorf("upperFirst(%q) = %q, want %q", in, got, want)
		}
	}
}
