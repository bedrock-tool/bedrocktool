//go:build !android

package settings

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
)

// settingsPlatform is the start screen's platform-only part; the PC has none.
type settingsPlatform struct{}

// platformCards go under the mode cards on the start screen.
func (p *Page) platformCards(th *material.Theme) []layout.FlexChild { return nil }
