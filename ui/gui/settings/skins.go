package settings

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type skinsSettings struct {
	skins *skins.SkinCMD

	grid *outlay.Grid

	Filter        widget.Editor
	Proxy         widget.Bool
	serverAddress *addressInput
}

func (s *skinsSettings) Init() {
	s.skins = commands.Registered["skins"].(*skins.SkinCMD)
	s.grid = &outlay.Grid{}
	s.serverAddress = AddressInput
	s.Filter.SingleLine = true
	s.Proxy.Value = true
}

func (s *skinsSettings) Apply() {
	s.skins.Filter = s.Filter.Text()
	s.skins.NoProxy = !s.Proxy.Value
	s.skins.ServerAddress = s.serverAddress.Value()
}

func (s *skinsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.grid.Layout(gtx, 3, 1, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return gtx.Dp(300)
				case layout.Vertical:
					return gtx.Dp(40)
				}
				panic("unreachable")
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				switch row {
				case 0:
					return layout.Center.Layout(gtx, material.CheckBox(th, &s.Proxy, "Enable Proxy").Layout)
				case 1:
					return layout.Center.Layout(gtx, material.Editor(th, &s.Filter, "Player name filter").Layout)
				case 2:
					return s.serverAddress.Layout(gtx, th)
				}
				panic("unreachable")
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(min(300, gtx.Constraints.Max.X)))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return s.serverAddress.LayoutRealms(gtx, Theme)
		}),
	)
}

func init() {
	Settings["skins"] = &skinsSettings{}
}
