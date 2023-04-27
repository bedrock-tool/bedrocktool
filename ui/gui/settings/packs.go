package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type packsSettings struct {
	packs *subcommands.ResourcePackCMD

	serverAddress widget.Editor
}

func (s *packsSettings) Init() {
	s.packs = utils.ValidCMDs["packs"].(*subcommands.ResourcePackCMD)
	s.serverAddress.SingleLine = true
}

func (s *packsSettings) Apply() {
	s.packs.ServerAddress = s.serverAddress.Text()
}

func (s *packsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Editor(th, &s.serverAddress, "Server Address").Layout),
	)
}

func init() {
	Settings["packs"] = &packsSettings{}
}
