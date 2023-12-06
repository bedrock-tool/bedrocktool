package pages

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type Popup interface {
	ID() string
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	Handler(data any) messages.Response
}

func LayoutPopupBackground(gtx layout.Context, th *material.Theme, tag string, widget layout.Widget) layout.Dimensions {
	// block events to other stacked below this
	pointer.InputOp{
		Tag:   tag,
		Kinds: pointer.Press | pointer.Release,
	}.Add(gtx.Ops)
	for _, ev := range gtx.Queue.Events(tag) {
		_ = ev
	}

	paint.ColorOp{Color: color.NRGBA{A: 170}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	width := min(500, gtx.Constraints.Max.X)
	return layout.Center.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		component.Rect{
			Color: th.Bg,
			Size:  image.Pt(width, 250),
			Radii: gtx.Dp(15),
		}.Layout(gtx)

		gtx.Constraints.Min.X = gtx.Dp(unit.Dp(width))
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(250))
		gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
		return layout.UniformInset(8).Layout(gtx, widget)
	})
}
