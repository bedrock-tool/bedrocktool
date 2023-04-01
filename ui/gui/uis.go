package gui

import (
	"gioui.org/layout"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type C = layout.Context
type D = layout.Dimensions

type HandlerFunc = func(data interface{}) messages.MessageResponse
