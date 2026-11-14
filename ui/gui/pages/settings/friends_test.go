package settings

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/friendworlds"
	"github.com/google/uuid"
	"image/color"
	"image/png"
)

// friendsInput is an address input whose account's friends are playing worlds.
func friendsInput(t *testing.T, worlds []friendworlds.World) (*addressInput, *popupRecorder) {
	t.Helper()
	w := &friendworlds.Watcher{
		Current: func() (any, friendworlds.ListFunc) {
			return "account", func(context.Context) ([]friendworlds.World, error) { return worlds, nil }
		},
		Interval: time.Minute, MinGap: time.Hour, Timeout: time.Second,
	}
	g := &popupRecorder{}
	a := &addressInput{
		g:           g,
		editor:      widget.Editor{SingleLine: true},
		connectInfo: &connectinfo.ConnectInfo{},
		friends:     w,
	}
	return a, g
}

func addressFrame(r *input.Router, a *addressInput) *op.Ops {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	pal := material.Palette{Bg: color.NRGBA{0x12, 0x12, 0x12, 0xff}, Fg: color.NRGBA{227, 227, 227, 0xff}, ContrastBg: color.NRGBA{142, 49, 235, 0xff}, ContrastFg: color.NRGBA{227, 227, 227, 0xff}}
	th2 := th.WithPalette(pal)
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(800, 120)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now(), Source: r.Source()}
	paint.Fill(ops, pal.Bg)
	a.Layout(gtx, &th2)
	r.Frame(ops)
	return ops
}

// buttonLabels are the address buttons' texts, and where each one is.
func buttonLabels(r *input.Router) map[string]f32.Point {
	out := map[string]f32.Point{}
	nodes := r.AppendSemantics(nil)
	for _, n := range nodes {
		if n.Desc.Class != semantic.Button {
			continue
		}
		label := n.Desc.Label
		for _, c := range nodes {
			if c.ParentID == n.ID && c.Desc.Label != "" {
				label = c.Desc.Label
			}
		}
		b := n.Desc.Bounds
		out[label] = f32.Pt(float32(b.Min.X+b.Max.X)/2, float32(b.Min.Y+b.Max.Y)/2)
	}
	return out
}

func waitChecked(t *testing.T, w *friendworlds.Watcher) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s := w.Snapshot(); !s.Checking && !s.Checked.IsZero() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("check never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFriendsButtonOnlyWithJoinableWorld(t *testing.T) {
	friends := locale.Loc("gui_address_friends", locale.Strmap{"Count": "1"})
	for _, tc := range []struct {
		name   string
		worlds []friendworlds.World
		shown  bool
	}{
		{"none", nil, false},
		{"only unjoinable", []friendworlds.World{{HandleID: uuid.New(), Host: "Bob", Status: friendworlds.Full}}, false},
		{"one joinable", []friendworlds.World{
			{HandleID: uuid.New(), Host: "Zed", Name: "Zed's Survival", Status: friendworlds.Joinable},
			{HandleID: uuid.New(), Host: "Bob", Status: friendworlds.Full},
		}, true},
	} {
		a, _ := friendsInput(t, tc.worlds)
		r := new(input.Router)
		addressFrame(r, a) // the first frame starts the check in the background
		waitChecked(t, a.friends)
		ops := addressFrame(r, a)
		labels := buttonLabels(r)
		if _, ok := labels[friends]; ok != tc.shown {
			t.Errorf("%s: Friends button shown=%v, want %v (buttons %v)", tc.name, ok, tc.shown, labels)
		}
		if _, ok := labels[locale.Loc("gui_address_realms", nil)]; !ok {
			t.Errorf("%s: the other address buttons went missing: %v", tc.name, labels)
		}
		if tc.shown {
			screenshot(t, ops, "address_friends")
		}
	}
}

func TestFriendsButtonPicksWorld(t *testing.T) {
	zed := friendworlds.World{HandleID: uuid.New(), Host: "Zed", Name: "Zed's Survival", Status: friendworlds.Joinable}
	a, g := friendsInput(t, []friendworlds.World{zed})
	r := new(input.Router)
	addressFrame(r, a)
	waitChecked(t, a.friends)
	addressFrame(r, a)
	at, ok := buttonLabels(r)[locale.Loc("gui_address_friends", locale.Strmap{"Count": "1"})]
	if !ok {
		t.Fatal("no Friends button")
	}
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
	addressFrame(r, a)
	if len(g.shown) != 1 {
		t.Fatalf("clicking Friends showed %d popups", len(g.shown))
	}
	p, ok := g.shown[0].(popups.Popup)
	if !ok || p.ID() != "FriendWorlds" {
		t.Fatalf("clicking Friends showed a %T", g.shown[0])
	}

	// pick the world in the popup: the address joins it
	pr := new(input.Router)
	popupFrame(pr, p)
	at, ok = buttonWith(pr, "Zed's Survival")
	if !ok {
		t.Fatal("the popup has no button for Zed's world")
	}
	pr.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
	popupFrame(pr, p)
	if len(g.closed) != 1 || g.closed[0] != p.ID() {
		t.Fatalf("picking should close the popup: closed %v", g.closed)
	}
	info := a.GetConnectInfo()
	if info == nil || info.Value != zed.Address() {
		t.Fatalf("address after picking: %v", info)
	}
	kind, err := info.Kind()
	if err != nil || kind != connectinfo.KindXbox {
		t.Fatalf("picked address is a %q (%v), want an xbox session", kind, err)
	}
}

func screenshot(t *testing.T, ops *op.Ops, name string) {
	dir := os.Getenv("RENDER_OUT_DIR")
	if dir == "" {
		return
	}
	sz := image.Pt(800, 120)
	win, err := headless.NewWindow(sz.X, sz.Y)
	if err != nil {
		t.Logf("no headless window: %s", err)
		return
	}
	defer win.Release()
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: sz})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(filepath.Join(dir, name+".png"))
	png.Encode(f, img)
	f.Close()
}

func (p *popupRecorder) ClosePopup(id string) { p.closed = append(p.closed, id) }

func popupFrame(r *input.Router, p popups.Popup) {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(800, 600)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now(), Source: r.Source()}
	p.Layout(gtx, th)
	r.Frame(ops)
}

// buttonWith finds the middle of a button showing text.
func buttonWith(r *input.Router, text string) (f32.Point, bool) {
	nodes := r.AppendSemantics(nil)
	for _, n := range nodes {
		if n.Desc.Class != semantic.Button {
			continue
		}
		for _, c := range nodes {
			if c.ParentID == n.ID && strings.Contains(c.Desc.Label, text) {
				b := n.Desc.Bounds
				return f32.Pt(float32(b.Min.X+b.Max.X)/2, float32(b.Min.Y+b.Max.Y)/2), true
			}
		}
	}
	return f32.Point{}, false
}
