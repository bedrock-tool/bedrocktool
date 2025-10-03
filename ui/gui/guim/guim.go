package guim

import (
	"gioui.org/layout"
	"gioui.org/x/explorer"
)

type Guim interface {
	ShowPopup(popup any)
	ClosePopup(id string)
	StartSubcommand(subCommand string, settings any)
	ExitSubcommand()
	Invalidate()
	Error(err error) error
	Explorer() *explorer.Explorer
	OpenUrl(uri string)
	Toast(gtx layout.Context, t string)
	CloseLogs()
	AccountName() string
	SetAccountName(name string)
}
