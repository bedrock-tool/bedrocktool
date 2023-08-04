package settings

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type worldSettings struct {
	worlds *world.WorldCMD

	withPacks     widget.Bool
	voidGen       widget.Bool
	saveImage     widget.Bool
	PacketCapture widget.Bool
	serverAddress *addressInput
}

func (s *worldSettings) Init() {
	s.worlds = commands.Registered["worlds"].(*world.WorldCMD)
	s.serverAddress = AddressInput
	s.voidGen.Value = true
	s.PacketCapture.Value = false
}

func (s *worldSettings) Apply() {
	s.worlds.Packs = s.withPacks.Value
	s.worlds.EnableVoid = s.voidGen.Value
	s.worlds.SaveImage = s.saveImage.Value
	s.worlds.ServerAddress = s.serverAddress.Value()
	s.worlds.SaveEntities = true
	s.worlds.SaveInventories = true
	utils.Options.Capture = s.PacketCapture.Value
}

func (s *worldSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.CheckBox(th, &s.withPacks, "with Packs").Layout),
		layout.Rigid(material.CheckBox(th, &s.voidGen, "void Generator").Layout),
		layout.Rigid(material.CheckBox(th, &s.saveImage, "save image").Layout),
		layout.Rigid(material.CheckBox(th, &s.PacketCapture, "packet capture").Layout),
		layout.Rigid(s.serverAddress.Layout(th)),
	)
}

func init() {
	Settings["worlds"] = &worldSettings{}
}
