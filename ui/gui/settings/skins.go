package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
)

type skinsSettings struct {
	grid   outlay.Grid
	Filter widget.Editor
	Proxy  widget.Bool
}

func (s *skinsSettings) Init() {
	s.Filter.SingleLine = true
	s.Proxy.Value = true
}

func (s *skinsSettings) Apply(c any) {
	cmd := c.(*skins.SkinCMD)
	cmd.Filter = s.Filter.Text()
	cmd.NoProxy = !s.Proxy.Value
	cmd.ServerAddress = AddressInput.Value()
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
				return layoutOption(gtx, th, func(gtx layout.Context) layout.Dimensions {
					switch col {
					case 0:
						return material.CheckBox(th, &s.Proxy, "Enable Proxy").Layout(gtx)
					case 1:
						return material.Editor(th, &s.Filter, "Player name filter").Layout(gtx)
					}
					panic("unreachable")
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return AddressInput.Layout(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return AddressInput.LayoutRealms(gtx, th)
		}),
	)
}

func init() {
	Settings["skins"] = &skinsSettings{}
}
