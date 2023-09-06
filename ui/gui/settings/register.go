package settings

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
)

type SettingsUI interface {
	Init()
	Apply()
	Layout(layout.Context, *material.Theme) layout.Dimensions
}

var Settings = map[string]SettingsUI{}
