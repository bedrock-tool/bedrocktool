package gui

import (
	"context"
	"errors"
	"image/color"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/pref/theme"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/packs"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/settings"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/skins"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/worlds"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"
)

type GUI struct {
	router *pages.Router
	ctx    context.Context
	cancel context.CancelCauseFunc
	logger logger
}

func (g *GUI) Init() bool {
	g.logger.list = widget.List{
		List: layout.List{
			Axis: layout.Vertical,
		},
	}
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
	g.ctx = ctx
	g.cancel = cancel

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
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

	w := app.NewWindow(
		app.Title("Bedrocktool " + updater.Version),
	)

	g.router = pages.NewRouter(ctx, w.Invalidate, th)
	g.router.UI = g
	g.router.Register(settings.New, settings.ID)
	g.router.Register(worlds.New, worlds.ID)
	g.router.Register(skins.New, skins.ID)
	g.router.Register(packs.New, packs.ID)
	g.router.SwitchTo(settings.ID)

	g.logger.router = g.router
	g.router.LogWidget = g.logger.Layout
	logrus.AddHook(&g.logger)

	utils.Auth.MSHandler = g.router.MSAuth

	go func() {
		app.Main()
	}()

	return g.loop(w)
}

func (g *GUI) loop(w *app.Window) error {
	var closing = false
	var ops op.Ops

	go func() {
		<-g.ctx.Done()
		w.Invalidate()
	}()

	for {
		e := w.NextEvent()
		if g.ctx.Err() != nil && !closing {
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wg.Wait()
			closing = true
		}
		switch e := e.(type) {
		case system.DestroyEvent:
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wg.Wait()
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			g.router.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (g *GUI) Message(data interface{}) messages.Response {
	switch data.(type) {
	case messages.CanShowImages:
		return messages.Response{Ok: true}
	}

	return g.router.Handler(data)
}

func (g *GUI) ServerInput(ctx context.Context, address string) (string, string, error) {
	return utils.ServerInput(ctx, address)
}
