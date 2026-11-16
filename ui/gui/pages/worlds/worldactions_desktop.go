//go:build !android

package worlds

import (
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
)

// worldActions are the buttons under a saved world's card. The PC has none
// (the card shows the file's path); Android's are in worldactions_android.go.
type worldActions struct{}

func (*worldActions) Layout(gtx C, th *material.Theme, g guim.Guim, entry *savedWorld) D {
	return D{}
}
