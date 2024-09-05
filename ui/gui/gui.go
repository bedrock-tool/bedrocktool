package gui

import (
	"context"
	"errors"
	"image/color"

	"gioui.org/app"
	"gioui.org/font/gofont"
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
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
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
	th     *material.Theme
}

func (g *GUI) Init() bool {
	messages.Router.AddHandler("ui", g.HandleMessage)

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
	g.router = pages.NewRouter(g, ctx)
	g.router.Register(settings.New, settings.ID)
	g.router.Register(worlds.New, worlds.ID)
	g.router.Register(skins.New, skins.ID)
	g.router.Register(packs.New, packs.ID)
	g.logger.router = g.router
	g.router.LogWidget = g.logger.Layout

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
	g.th = th

	var window app.Window
	window.Option(app.Title("Bedrocktool " + updater.Version))
	g.router.Invalidate = window.Invalidate
	logrus.AddHook(&g.logger)
	g.router.SwitchTo(settings.ID)

	isDebug := updater.Version == ""
	if !isDebug {
		go updater.UpdateCheck(g)
	}

	utils.ErrorHandler = func(err error) {
		utils.PrintPanic(err)
		g.router.RemovePopup("connect")
		g.router.PushPopup(popups.NewErrorPopup(err, func() {
			g.router.SwitchTo("settings")
		}, true))
	}

	go func() {
		app.Main()
	}()

	return g.loop(&window)
}

func (g *GUI) loop(window *app.Window) error {
	var closing = false
	var ops op.Ops

	go func() {
		<-g.ctx.Done()
		window.Invalidate()
	}()

	for {
		e := window.Event()
		//fmt.Printf("window.Event %+#v\n", e)

		if g.ctx.Err() != nil && !closing {
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wg.Wait()
			closing = true
		}
		switch e := e.(type) {
		case app.DestroyEvent:
			g.router.ShuttingDown = true
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wg.Wait()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			g.router.Layout(gtx, g.th)
			e.Frame(gtx.Ops)
		}
	}
}

func (g *GUI) HandleMessage(msg *messages.Message) *messages.Message {
	switch data := msg.Data.(type) {
	case messages.Features:
		if data.Request {
			return &messages.Message{
				Source: "ui",
				Target: msg.Source,
				Data: messages.Features{
					Request: false,
					Features: []string{
						"images",
					},
				},
			}
		}
		return nil
	}

	return g.router.HandleMessage(msg)
}
