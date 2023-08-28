package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type skinsSettings struct {
	skins *skins.SkinCMD

	grid *outlay.Grid

	Filter widget.Editor
	Proxy  widget.Bool
}

func (s *skinsSettings) Init() {
	s.skins = commands.Registered["skins"].(*skins.SkinCMD)
	s.grid = &outlay.Grid{}
	s.Filter.SingleLine = true
	s.Proxy.Value = true
}

func (s *skinsSettings) Apply() {
	s.skins.Filter = s.Filter.Text()
	s.skins.NoProxy = !s.Proxy.Value
	s.skins.ServerAddress = AddressInput.Value()
}

func (s *skinsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.grid.Layout(gtx, 1, 2, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return gtx.Dp(300)
				case layout.Vertical:
					return gtx.Dp(40)
				}
				panic("unreachable")
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				return component.Surface(&material.Theme{
					Palette: material.Palette{
						Bg: component.WithAlpha(th.ContrastFg, 8),
					},
				}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						switch col {
						case 0:
							return material.CheckBox(th, &s.Proxy, "Enable Proxy").Layout(gtx)
						case 1:
							return material.Editor(th, &s.Filter, "Player name filter").Layout(gtx)
						}
						panic("unreachable")
					})
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return AddressInput.Layout(gtx, Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return AddressInput.LayoutRealms(gtx, Theme)
		}),
	)
}

func init() {
	Settings["skins"] = &skinsSettings{}
}
