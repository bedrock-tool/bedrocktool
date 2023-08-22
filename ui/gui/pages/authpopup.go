package pages

import (
	"image/color"
	"math"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type guiAuth struct {
	router *Router
	show   bool
	uri    string
	code   string
	err    error
}

func (g *guiAuth) AuthCode(uri, code string) {
	g.show = true
	g.uri = uri
	g.code = code
	g.router.Invalidate()
}

func (g *guiAuth) Success() {
	g.show = false
}

func (g *guiAuth) PollError(err error) error {
	g.err = err
	return err
}

func (g *guiAuth) Layout(gtx layout.Context) layout.Dimensions {
	if !g.show {
		return layout.Dimensions{}
	}

	// block events to other stacked below this
	pointer.InputOp{
		Tag:   "guiAuth",
		Types: pointer.Press | pointer.Release,
	}.Add(gtx.Ops)
	for _, ev := range gtx.Queue.Events("guiAuth") {
		_ = ev
	}

	paint.ColorOp{Color: color.NRGBA{A: 170}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			width := 500
			if width > gtx.Constraints.Max.X {
				width = int(math.Floor(float64(gtx.Constraints.Max.X)/50)) * 50
			}

			gtx.Constraints.Min.X = gtx.Dp(unit.Dp(width))
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(250))
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y

			component.Rect{
				Color: g.router.Theme.Bg,
				Size:  gtx.Constraints.Max,
				Radii: gtx.Dp(15),
			}.Layout(gtx)

			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(material.Body1(g.router.Theme, "Authenticate at: "+g.uri).Layout),
					layout.Rigid(material.Body1(g.router.Theme, "Using Code: "+g.code).Layout),
				)
			})
		})
	})
}
