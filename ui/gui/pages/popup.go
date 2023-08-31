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
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type Popup interface {
	ID() string
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	Handler(data any) messages.MessageResponse
}

func layoutPopupBackground(gtx layout.Context, th *material.Theme, tag string, widget layout.Widget) layout.Dimensions {
	// block events to other stacked below this
	pointer.InputOp{
		Tag:   tag,
		Types: pointer.Press | pointer.Release,
	}.Add(gtx.Ops)
	for _, ev := range gtx.Queue.Events(tag) {
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
				Color: th.Bg,
				Size:  gtx.Constraints.Max,
				Radii: gtx.Dp(15),
			}.Layout(gtx)

			return widget(gtx)
		})
	})
}
