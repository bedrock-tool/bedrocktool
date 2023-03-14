package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type worldSettings struct {
	worlds *world.WorldCMD

	withPacks     widget.Bool
	voidGen       widget.Bool
	saveImage     widget.Bool
	serverAddress widget.Editor
}

func (s *worldSettings) Init() {
	s.worlds = utils.ValidCMDs["worlds"].(*world.WorldCMD)
	s.serverAddress.SingleLine = true
}

func (s *worldSettings) Apply() {
	s.worlds.Packs = s.withPacks.Value
	s.worlds.EnableVoid = s.voidGen.Value
	s.worlds.SaveImage = s.saveImage.Value
	s.worlds.ServerAddress = s.serverAddress.Text()
}

func (s *worldSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.CheckBox(th, &s.withPacks, "with Packs").Layout),
		layout.Rigid(material.CheckBox(th, &s.voidGen, "void Generator").Layout),
		layout.Rigid(material.CheckBox(th, &s.saveImage, "save image").Layout),
		layout.Rigid(material.Editor(th, &s.serverAddress, "server Address").Layout),
	)
}

func init() {
	Settings["worlds"] = &worldSettings{}
}
