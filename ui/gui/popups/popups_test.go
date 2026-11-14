package popups

import (
	"image"
	"image/color"
	"image/png"
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
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

// fakeGuim records which popups were closed.
type fakeGuim struct {
	closed []string
}

func (f *fakeGuim) ShowPopup(popup any)                {}
func (f *fakeGuim) ClosePopup(id string)               { f.closed = append(f.closed, id) }
func (f *fakeGuim) StartSubcommand(string, any)        {}
func (f *fakeGuim) ExitSubcommand()                    {}
func (f *fakeGuim) Invalidate()                        {}
func (f *fakeGuim) Error(err error) error              { return nil }
func (f *fakeGuim) Explorer() *explorer.Explorer       { return nil }
func (f *fakeGuim) OpenUrl(uri string)                 {}
func (f *fakeGuim) Toast(gtx layout.Context, t string) {}
func (f *fakeGuim) CloseLogs()                         {}
func (f *fakeGuim) AccountName() string                { return "" }
func (f *fakeGuim) SetAccountName(name string)         {}

var winSize = image.Pt(800, 700)

func darkTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	t := th.WithPalette(material.Palette{
		Bg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
		Fg:         color.NRGBA{227, 227, 227, 0xff},
		ContrastBg: color.NRGBA{142, 49, 235, 0xff},
		ContrastFg: color.NRGBA{227, 227, 227, 0xff},
	})
	return &t
}

// frame lays a popup out once over a dark window.
func frame(r *input.Router, th *material.Theme, p Popup) *op.Ops {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Constraints: layout.Exact(winSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         time.Now(),
		Source:      r.Source(),
	}
	paint.Fill(ops, th.Bg)
	p.Layout(gtx, th)
	r.Frame(ops)
	return ops
}

// buttonAt finds a clickable with the label text and its middle.
func buttonAt(t *testing.T, r *input.Router, label string) f32.Point {
	t.Helper()
	nodes := r.AppendSemantics(nil)
	for _, n := range nodes {
		if n.Desc.Class != semantic.Button {
			continue
		}
		var texts []string
		for _, c := range nodes {
			if c.ParentID == n.ID && c.Desc.Label != "" {
				texts = append(texts, c.Desc.Label)
			}
		}
		if n.Desc.Label == label || strings.Contains(strings.Join(texts, " "), label) {
			b := n.Desc.Bounds
			return f32.Pt(float32(b.Min.X+b.Max.X)/2, float32(b.Min.Y+b.Max.Y)/2)
		}
	}
	t.Fatalf("no %q button", label)
	return f32.Point{}
}

func click(r *input.Router, at f32.Point) {
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
}

func screenshot(t *testing.T, ops *op.Ops, name string) {
	dir := os.Getenv("RENDER_OUT_DIR")
	if dir == "" {
		return
	}
	win, err := headless.NewWindow(winSize.X, winSize.Y)
	if err != nil {
		t.Logf("no headless window: %s", err)
		return
	}
	defer win.Release()
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: winSize})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(filepath.Join(dir, name+".png"))
	png.Encode(f, img)
	f.Close()
}

// closes checks that pressing the button labelled label closes the popup.
func closes(t *testing.T, p Popup, label, shot string) {
	t.Helper()
	g := &fakeGuim{}
	setGuim(p, g)
	th := darkTheme()
	r := new(input.Router)
	ops := frame(r, th, p)
	screenshot(t, ops, shot)
	click(r, buttonAt(t, r, label))
	frame(r, th, p)
	frame(r, th, p)
	for _, id := range g.closed {
		if id == p.ID() {
			return
		}
	}
	t.Fatalf("%s: pressing %q didn't close it", p.ID(), label)
}

func setGuim(p Popup, g *fakeGuim) {
	switch p := p.(type) {
	case *RealmsList:
		p.g = g
	case *UpdatePopup:
		p.g = g
	case *StoriesPopup:
		p.g = g
	case *ConnectPopup:
		p.g = g
	case *errorPopup:
		p.g = g
	case *ConfirmPopup:
		p.g = g
	case *guiAuth:
		p.g = g
	}
}

func TestRealmsListCloses(t *testing.T) {
	p := NewRealmsList(nil, func(*realms.Realm) {}).(*RealmsList)
	p.loaded = true
	for _, r := range []realms.Realm{
		{ID: 1, Name: "Magic Realm", State: "OPEN"},
		{ID: 2, Name: "Bedrock Realm", State: "OPEN", Expired: true},
		{ID: 3, Name: "Breaking Hive²", State: "OPEN", Expired: true},
		{ID: 4, Name: "FrostPvP Community", State: "CLOSED"},
	} {
		p.realms = append(p.realms, &realmButton{Realm: &r})
	}
	closes(t, p, "Close", "realms")
}

func TestRealmsListPicksOpenRealm(t *testing.T) {
	var picked *realms.Realm
	p := NewRealmsList(nil, func(r *realms.Realm) { picked = r }).(*RealmsList)
	g := &fakeGuim{}
	p.g = g
	p.loaded = true
	open := realms.Realm{ID: 1, Name: "Magic Realm", State: "OPEN"}
	p.realms = []*realmButton{{Realm: &open}}
	th := darkTheme()
	r := new(input.Router)
	frame(r, th, p)
	click(r, buttonAt(t, r, "Magic Realm"))
	frame(r, th, p)
	if picked == nil || picked.ID != 1 || len(g.closed) == 0 {
		t.Fatalf("clicking a Realm should pick it and close: picked=%v closed=%v", picked, g.closed)
	}
}

func TestUpdatePopupCloses(t *testing.T) {
	p := NewUpdatePopup(nil).(*UpdatePopup)
	p.state = stateAsk
	p.update = &updater.Update{Version: "1.63.0"}
	closes(t, p, "Close", "update")
}

func TestUpdatePopupErrorCloses(t *testing.T) {
	p := NewUpdatePopup(nil).(*UpdatePopup)
	p.state = stateError
	closes(t, p, "Close", "update_error")
}

func TestStoriesPopup(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  messages.StoriesAnswer
	}{{"Opt out", messages.StoriesOptOut}, {"Opt in", messages.StoriesOptIn}, {"Cancel", messages.StoriesCancel}} {
		reply := make(chan messages.StoriesAnswer, 1)
		p := NewStoriesPopup(nil, &messages.EventStoriesPrompt{RealmName: "Magic Realm", Reply: reply})
		closes(t, p, tc.label, "stories")
		select {
		case got := <-reply:
			if got != tc.want {
				t.Fatalf("%s answered %v", tc.label, got)
			}
		default:
			t.Fatalf("%s sent no answer", tc.label)
		}
		p.Close() // closing after answering mustn't block or answer twice
	}
}

func TestConnectPopupCloses(t *testing.T) {
	for _, tc := range []struct{ state, button string }{
		{"", "Cancel"}, {"listening", "Cancel"}, {"connecting-server", "Cancel"}, {"established", "Hide"},
	} {
		p := NewConnect(nil, "0.0.0.0:19132").(*ConnectPopup)
		p.state = tc.state
		p.localIP = "192.168.1.20"
		closes(t, p, tc.button, "connect_"+tc.state)
	}
}

func TestErrorPopupCloses(t *testing.T) {
	p := NewErrorPopup(nil, os.ErrNotExist, false, nil)
	closes(t, p, "Close", "error")
}

func TestConfirmPopup(t *testing.T) {
	for _, tc := range []struct {
		button string
		want   bool
	}{{"Keep recording", false}, {"Turn off anyway", true}} {
		var got *bool
		p := NewConfirm(nil, "confirm-capture-off", "Turn off session recording?",
			[]string{"The .pcap2 recording holds every packet.", "Without it, anything missed is gone for good."},
			"Keep recording", "Turn off anyway", func(proceed bool) { got = &proceed })
		closes(t, p, tc.button, "confirm")
		if got == nil || *got != tc.want {
			t.Fatalf("%s: answered %v, want %v", tc.button, got, tc.want)
		}
		p.Close() // closing after an answer mustn't answer again
	}

	// closed without picking: keep
	var got *bool
	p := NewConfirm(nil, "c", "t", nil, "Keep", "Go", func(proceed bool) { got = &proceed })
	p.Close()
	if got == nil || *got {
		t.Fatal("closing without an answer should keep things as they were")
	}
}

// The login popup shows the #348 anti-cheat note, and closes.
func TestAuthPopup(t *testing.T) {
	p := NewGuiAuth(nil, "https://www.microsoft.com/link", "ABCD1234")
	th := darkTheme()
	r := new(input.Router)
	frame(r, th, p)
	var labels []string
	for _, n := range r.AppendSemantics(nil) {
		labels = append(labels, n.Desc.Label)
	}
	all := strings.Join(labels, "\n")
	if !strings.Contains(all, auth.AntiCheatNotice) {
		t.Error("login popup doesn't show the anti-cheat note")
	}
	closes(t, p, "Close", "auth")
}
