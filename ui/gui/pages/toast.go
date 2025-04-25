package pages

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type Toast struct {
	Message   string
	Visible   bool
	StartTime time.Time
	Duration  time.Duration
}

func (t *Toast) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if !t.Visible {
		return layout.Dimensions{}
	}

	bgColor := color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	textColor := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	padding := unit.Dp(12)
	cornerRadius := unit.Dp(8)

	return layout.Inset{
		Top:    padding,
		Bottom: padding,
		Left:   padding,
		Right:  padding,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		rec := op.Record(gtx.Ops)
		dims := layout.UniformInset(cornerRadius).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(0, 0)
			label := material.Body1(th, t.Message)
			label.Color = textColor
			return label.Layout(gtx)
		})
		rep := rec.Stop()

		toastRect := clip.RRect{
			Rect: image.Rect(0, 0, dims.Size.X, dims.Size.Y),
			SE:   int(cornerRadius),
			SW:   int(cornerRadius),
			NW:   int(cornerRadius),
			NE:   int(cornerRadius),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		rep.Add(gtx.Ops)
		toastRect.Pop()
		return dims
	})
}
