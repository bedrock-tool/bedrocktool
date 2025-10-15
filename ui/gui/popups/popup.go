package popups

import (
	"image"
	"image/color"
	"io"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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
	io.Closer
	messages.EventHandler
}

func LayoutPopupBackground(gtx C, th *material.Theme, tag string, widget layout.Widget) D {
	paint.ColorOp{Color: color.NRGBA{A: 170}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	width := gtx.Constraints.Max.X
	if width > gtx.Dp(300) {
		width -= gtx.Dp(30)
	}
	if width > gtx.Dp(600) {
		width = gtx.Dp(600)
	}
	//width -= gtx.Dp(unit.Dp(min(float32(width)/1000, 0.5) * 300))
	return layout.Center.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, tag)
		for {
			_, ok := gtx.Event(pointer.Filter{Target: tag})
			if !ok {
				break
			}
		}

		// set width constant, height min 250, max 80%
		gtx.Constraints.Min.X = width
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		gtx.Constraints.Min.Y = gtx.Dp(250)
		gtx.Constraints.Max.Y = int(float32(gtx.Constraints.Max.Y) * 0.8)

		macro := op.Record(gtx.Ops)
		wdims := layout.UniformInset(8).Layout(gtx, widget)
		call := macro.Stop()

		component.Rect{
			Color: th.Bg,
			Size:  image.Pt(width, wdims.Size.Y),
			Radii: gtx.Dp(15),
		}.Layout(gtx)

		call.Add(gtx.Ops)
		return wdims
	})
}
