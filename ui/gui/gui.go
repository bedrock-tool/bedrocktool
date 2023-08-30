package gui

import (
	"context"
	"errors"
	"image/color"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"
	"gioui.org/x/pref/theme"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	_ "github.com/bedrock-tool/bedrocktool/ui/gui/pages/connect"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/packs"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/settings"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/skins"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/update"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/worlds"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

type GUI struct {
	router *pages.Router
	cancel context.CancelCauseFunc
}

func (g *GUI) Init() bool {
	return true
}

var paletteLight = material.Palette{
	Bg:         color.NRGBA{0xff, 0xff, 0xff, 0xff},
	Fg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	ContrastBg: color.NRGBA{142, 49, 235, 0xff},
	ContrastFg: color.NRGBA{0x00, 0x00, 0x00, 0xff},
}

var paletteDark = material.Palette{
	Bg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	Fg:         color.NRGBA{227, 227, 227, 0xff},
	ContrastBg: color.NRGBA{142, 49, 235, 0xff},
	ContrastFg: color.NRGBA{227, 227, 227, 0xff},
}

func (g *GUI) Start(ctx context.Context, cancel context.CancelCauseFunc) (err error) {
	g.cancel = cancel

	th := material.NewTheme()
	dark, err := theme.IsDarkMode()
	if err != nil {
		logrus.Warn(err)
	}
	if dark || true {
		_th := th.WithPalette(paletteDark)
		th = &_th
	} else {
		_th := th.WithPalette(paletteLight)
		th = &_th
	}

	w := app.NewWindow(
		app.Title("Bedrocktool " + utils.Version),
	)

	g.router = pages.NewRouter(ctx, w.Invalidate, th)
	g.router.UI = g
	g.router.Register(settings.New(g.router))
	g.router.Register(worlds.New(g.router))
	g.router.Register(skins.New(g.router))
	g.router.Register(packs.New(g.router))
	g.router.Register(update.New(g.router))
	utils.Auth.MSHandler = g.router.MSAuth

	g.router.SwitchTo("Settings")

	go func() {
		app.Main()
	}()

	return g.loop(w)
}

func (g *GUI) loop(w *app.Window) error {
	var ops op.Ops
	for {
		select {
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				logrus.Info("Closing")
				g.cancel(errors.New("Closing"))
				g.router.Wg.Wait()
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				g.router.Layout(gtx, g.router.Theme)
				e.Frame(gtx.Ops)
			}
		case <-g.router.Ctx.Done():
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wg.Wait()
			return nil
		}
	}
}

func (g *GUI) Message(data interface{}) messages.MessageResponse {
	r := g.router.Handler(data)
	if r.Ok || r.Data != nil {
		return r
	}

	r = messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch data.(type) {
	case messages.CanShowImages:
		r.Ok = true
	}

	return r
}

func (g *GUI) ServerInput(ctx context.Context, address string) (string, string, error) {
	return utils.ServerInput(ctx, address)
}
