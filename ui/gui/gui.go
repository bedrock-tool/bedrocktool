package gui

import (
	"bufio"
	"context"
	"errors"
	"image/color"
	"io"

	"gioui.org/app"
	"gioui.org/font/gofont"
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
	router        pages.Router
	cancel        context.CancelCauseFunc
	authPopup     bool
	authPopupText string
}

func (g *GUI) Init() bool {
	utils.Auth.LoginWithMicrosoftCallback = g.LoginWithMicrosoftCallback
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

func (g *GUI) Start(ctx context.Context, cancel context.CancelCauseFunc) (err error) {
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
	g.router.UI = g
	g.router.Register(settings.New(&g.router))
	g.router.Register(worlds.New(&g.router))
	g.router.Register(skins.New(&g.router))
	g.router.Register(packs.New(&g.router))
	g.router.Register(update.New(&g.router))

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
				g.cancel(errors.New("Closing"))
				g.router.Wg.Wait()
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				layout.Stack{
					Alignment: layout.Center,
				}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return g.router.Layout(gtx, g.router.Theme)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if g.authPopup {
							return g.AuthPopup(gtx)
						}
						return layout.Dimensions{}
					}),
				)

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

func (g *GUI) AuthPopup(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max = gtx.Constraints.Max.Div(2)
	return layout.Center.Layout(gtx, material.Body1(g.router.Theme, g.authPopupText).Layout)
}

func (g *GUI) LoginWithMicrosoftCallback(r io.Reader) {
	g.authPopup = true
	b := bufio.NewReader(r)
	for {
		line, _, err := b.ReadLine()
		if err != nil {
			panic(err)
		}
		println(string(line))
		g.authPopupText += string(line) + "\n"
		g.router.Invalidate()
		if string(line) == "Authentication successful." {
			break
		}
	}
	g.authPopup = false
}
