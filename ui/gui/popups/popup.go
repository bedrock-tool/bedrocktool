package popups

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Popup interface {
	ID() string
	Layout(gtx C, th *material.Theme) D
	messages.Handler
}

func LayoutPopupBackground(gtx C, th *material.Theme, tag string, widget layout.Widget) D {
	paint.ColorOp{Color: color.NRGBA{A: 170}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	width := gtx.Constraints.Max.X
	width -= gtx.Dp(unit.Dp(min(float32(width)/1000, 0.5) * 300))
	return layout.Center.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		component.Rect{
			Color: th.Bg,
			Size:  image.Pt(width, 250),
			Radii: gtx.Dp(15),
		}.Layout(gtx)

		gtx.Constraints.Min.X = width
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		gtx.Constraints.Min.Y = 250
		gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
		return layout.UniformInset(8).Layout(gtx, widget)
	})
}
