//go:build gui || android

package ui

import (
	"context"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/settings"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/worlds"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

type GUI struct {
	utils.BaseUI

	router pages.Router
	cancel context.CancelFunc
}

func (g *GUI) Init() bool {
	utils.SetCurrentUI(g)
	return true
}

func (g *GUI) Start(ctx context.Context, cancel context.CancelFunc) (err error) {
	g.cancel = cancel

	w := app.NewWindow(
		app.Title("Bedrocktool"),
	)

	g.router = pages.NewRouter(ctx, w.Invalidate)

	g.router.Register("Settings", settings.New(&g.router))
	g.router.Register("worlds", worlds.New(&g.router))

	g.router.SwitchTo("Settings")

	go func() {
		err = g.run(w)
	}()

	go func() {
		app.Main()
	}()

	<-ctx.Done()

	return err
}

func (g *GUI) run(w *app.Window) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops
	for {
		select {
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				g.router.Layout(gtx, th)
				e.Frame(gtx.Ops)
			case *system.DestroyEvent:
				g.cancel()
				g.router.Wg.Wait()
				return nil
			}
		case <-g.router.Ctx.Done():
			logrus.Info("Closing")
			g.cancel()
			g.router.Wg.Wait()
			return nil
		}
	}
}

func (g *GUI) Message(name string, data interface{}) utils.MessageResponse {
	r := g.router.Handler(name, data)
	if r.Ok || r.Data != nil {
		return r
	}

	r = utils.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case "can_show_images":
		r.Ok = true
	}

	return r
}

func init() {
	utils.MakeGui = func() utils.UI {
		return &GUI{}
	}
}
