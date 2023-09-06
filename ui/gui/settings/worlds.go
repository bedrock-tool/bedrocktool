package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type worldSettings struct {
	worlds *world.WorldCMD

	grid outlay.Grid

	withPacks     widget.Bool
	voidGen       widget.Bool
	saveImage     widget.Bool
	packetCapture widget.Bool
}

func (s *worldSettings) Init() {
	s.worlds = commands.Registered["worlds"].(*world.WorldCMD)
	s.voidGen.Value = true
	s.packetCapture.Value = false
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
				return layoutOption(gtx, th, func(gtx layout.Context) layout.Dimensions {
					switch row {
					case 0:
						switch col {
						case 0:
							return material.CheckBox(th, &s.withPacks, "with Packs").Layout(gtx)
						case 1:
							return material.CheckBox(th, &s.packetCapture, "packet capture").Layout(gtx)
						}
					case 1:
						switch col {
						case 0:
							return material.CheckBox(th, &s.saveImage, "save png").Layout(gtx)
						case 1:
							return material.CheckBox(th, &s.voidGen, "void Generator").Layout(gtx)
						}
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
	Settings["worlds"] = &worldSettings{}
}
