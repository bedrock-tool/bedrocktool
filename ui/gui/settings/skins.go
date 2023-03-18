package settings

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type skinsSettings struct {
	skins *skins.SkinCMD

	Filter        widget.Editor
	Proxy         widget.Bool
	serverAddress widget.Editor
}

func (s *skinsSettings) Init() {
	s.skins = utils.ValidCMDs["skins"].(*skins.SkinCMD)
	s.serverAddress.SingleLine = true
	s.Filter.SingleLine = true
	s.Proxy.Value = true
}

func (s *skinsSettings) Apply() {
	s.skins.Filter = s.Filter.Text()
	s.skins.NoProxy = !s.Proxy.Value
	s.skins.ServerAddress = s.serverAddress.Text()
}

func (s *skinsSettings) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.CheckBox(th, &s.Proxy, "Enable Proxy").Layout),
		layout.Rigid(material.Editor(th, &s.Filter, "Player name filter").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
		layout.Rigid(material.Editor(th, &s.serverAddress, "server Address").Layout),
	)
}

func init() {
	Settings["skins"] = &skinsSettings{}
}
