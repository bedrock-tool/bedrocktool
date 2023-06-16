package settings

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type packsSettings struct {
	packs *subcommands.ResourcePackCMD

	serverAddress *addressInput
}

func (s *packsSettings) Init() {
	s.packs = utils.ValidCMDs["packs"].(*subcommands.ResourcePackCMD)
	s.serverAddress = AddressInput
}

func (s *packsSettings) Apply() {
	s.packs.ServerAddress = s.serverAddress.Value()
}

func (s *packsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.serverAddress.Layout(th)),
	)
}

func init() {
	Settings["packs"] = &packsSettings{}
}
