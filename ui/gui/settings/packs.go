package settings

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type packsSettings struct {
	packs *subcommands.ResourcePackCMD
}

func (s *packsSettings) Init() {
	s.packs = commands.Registered["packs"].(*subcommands.ResourcePackCMD)
}

func (s *packsSettings) Apply() {
	s.packs.ServerAddress = AddressInput.Value()
}

func (s *packsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(min(300, gtx.Constraints.Max.X)))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return AddressInput.Layout(gtx, th)
		}),
	)
}

func init() {
	Settings["packs"] = &packsSettings{}
}
