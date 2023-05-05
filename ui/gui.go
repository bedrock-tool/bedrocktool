//go:build gui

package ui

import (
	"context"
	"image/color"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"
	"gioui.org/x/pref/theme"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
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
	utils.BaseUI

	router pages.Router
	cancel context.CancelFunc
}

func (g *GUI) Init() bool {
	return true
}

var paletteLight = material.Palette{
	Bg:         color.NRGBA{0xff, 0xff, 0xff, 0xff},
	Fg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	ContrastBg: color.NRGBA{0x7c, 0x00, 0xf8, 0xff},
	ContrastFg: color.NRGBA{0x00, 0x00, 0x00, 0xff},
}

var paletteDark = material.Palette{
	Bg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	Fg:         color.NRGBA{0xff, 0xff, 0xff, 0xff},
	ContrastBg: color.NRGBA{0x7c, 0x00, 0xf8, 0xff},
	ContrastFg: color.NRGBA{0xff, 0xff, 0xff, 0xff},
}

func (g *GUI) Start(ctx context.Context, cancel context.CancelFunc) (err error) {
	g.cancel = cancel

	w := app.NewWindow(
		app.Title("Bedrocktool " + utils.Version),
	)

	th := material.NewTheme(gofont.Collection())
	dark, err := theme.IsDarkMode()
	if err != nil {
		logrus.Warn(err)
	}
	if dark {
		_th := th.WithPalette(paletteDark)
		th = &_th
	} else {
		_th := th.WithPalette(paletteLight)
		th = &_th
	}

	g.router = pages.NewRouter(ctx, w.Invalidate, th)
	g.router.Register("Settings", settings.New(&g.router))
	g.router.Register("worlds", worlds.New(&g.router))
	g.router.Register("skins", skins.New(&g.router))
	g.router.Register("packs", packs.New(&g.router))
	g.router.Register("update", update.New(&g.router))

	g.router.SwitchTo("Settings")

	go func() {
		app.Main()
	}()

	return g.run(w)
}

func (g *GUI) run(w *app.Window) error {
	var ops op.Ops
	for {
		select {
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				logrus.Info("Closing")
				g.cancel()
				g.router.Wg.Wait()
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				g.router.Layout(gtx, g.router.Theme)
				e.Frame(gtx.Ops)
			}
		case <-g.router.Ctx.Done():
			logrus.Info("Closing")
			g.cancel()
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

func init() {
	utils.MakeGui = func() utils.UI {
		return &GUI{}
	}
}
