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

	grid    *outlay.Grid
	options [][]layout.Widget

	withPacks     widget.Bool
	voidGen       widget.Bool
	saveImage     widget.Bool
	packetCapture widget.Bool
	serverAddress *addressInput
}

func nullWidget(layout.Context) layout.Dimensions {
	return layout.Dimensions{}
}

func (s *worldSettings) Init() {
	s.worlds = commands.Registered["worlds"].(*world.WorldCMD)
	s.grid = &outlay.Grid{}
	s.serverAddress = AddressInput
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
		{
			func(gtx layout.Context) layout.Dimensions {
				return s.serverAddress.Layout(gtx, Theme)
			},
			nullWidget,
		},
	}
}

func (s *worldSettings) Apply() {
	s.worlds.Packs = s.withPacks.Value
	s.worlds.EnableVoid = s.voidGen.Value
	s.worlds.SaveImage = s.saveImage.Value
	s.worlds.ServerAddress = s.serverAddress.Value()
	s.worlds.SaveEntities = true
	s.worlds.SaveInventories = true
	utils.Options.Capture = s.packetCapture.Value
}

func (s *worldSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.grid.Layout(gtx, 3, 2, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					switch index {
					case 4:
						return gtx.Dp(300)
					case 5:
						return gtx.Dp(0)
					default:
						return gtx.Dp(150)
					}
				case layout.Vertical:
					return gtx.Dp(40)
				}
				panic("unreachable")
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				return s.options[row][col](gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.serverAddress.LayoutRealms(gtx, Theme)
		}),
	)
}

func init() {
	Settings["worlds"] = &worldSettings{}
}
