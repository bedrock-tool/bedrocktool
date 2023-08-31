package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type worldSettings struct {
	worlds *world.WorldCMD

	grid    outlay.Grid
	options [][]layout.Widget

	withPacks     widget.Bool
	voidGen       widget.Bool
	saveImage     widget.Bool
	packetCapture widget.Bool
}

func (s *worldSettings) Init() {
	s.worlds = commands.Registered["worlds"].(*world.WorldCMD)
	s.voidGen.Value = true
	s.packetCapture.Value = false

	s.options = [][]layout.Widget{
		{
			material.CheckBox(Theme, &s.withPacks, "with Packs").Layout,
			material.CheckBox(Theme, &s.packetCapture, "packet capture").Layout,
		},
		{
			material.CheckBox(Theme, &s.saveImage, "save png").Layout,
			material.CheckBox(Theme, &s.voidGen, "void Generator").Layout,
		},
	}
}

func (s *worldSettings) Apply() {
	s.worlds.Packs = s.withPacks.Value
	s.worlds.EnableVoid = s.voidGen.Value
	s.worlds.SaveImage = s.saveImage.Value
	s.worlds.ServerAddress = AddressInput.Value()
	s.worlds.SaveEntities = true
	s.worlds.SaveInventories = true
	utils.Options.Capture = s.packetCapture.Value
}

func (s *worldSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	gtx.Constraints.Max.X = gtx.Constraints.Max.X / 2
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.grid.Layout(gtx, 2, 2, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return constraint / 2
				case layout.Vertical:
					return gtx.Dp(50)
				}
				panic("unreachable")
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				return component.Surface(&material.Theme{
					Palette: material.Palette{
						Bg: component.WithAlpha(th.ContrastFg, 8),
					},
				}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(5).Layout(gtx, s.options[row][col])
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
	Settings["worlds"] = &worldSettings{}
}
