package gui

import (
	"fyne.io/fyne/v2"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type HandlerFunc func(name string, data interface{}) utils.MessageResponse

type CommandUI interface {
	Layout() fyne.CanvasObject
	Handler() HandlerFunc
}

var CommandUIs = map[string]CommandUI{}
