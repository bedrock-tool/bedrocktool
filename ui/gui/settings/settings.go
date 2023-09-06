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
		return layout.Inset{Top: 3, Bottom: 3, Left: 3, Right: 5}.Layout(gtx, w)
	})
}
