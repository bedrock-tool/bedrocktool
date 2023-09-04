package settings

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

func layoutOption(gtx layout.Context, th *material.Theme, w layout.Widget) layout.Dimensions {
	return component.Surface(&material.Theme{
		Palette: material.Palette{
			Bg: component.WithAlpha(th.ContrastFg, 8),
		},
	}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 5, Bottom: 5, Left: 5, Right: 8}.Layout(gtx, w)
	})
}
