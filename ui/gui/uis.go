package gui

import (
	"gioui.org/layout"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type C = layout.Context
type D = layout.Dimensions

type HandlerFunc = func(name string, data interface{}) utils.MessageResponse
