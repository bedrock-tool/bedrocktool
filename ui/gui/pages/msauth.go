package pages

import (
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type msAuth struct {
	router *Router
	p      popups.Popup
}

func (m *msAuth) Success() {
	m.router.RemovePopup(m.p.ID())
	m.p = nil
}

func (m *msAuth) PollError(err error) error {
	m.p.HandleMessage(&messages.Message{
		Source: "msAuth",
		Data:   messages.Error(err),
	})
	m.router.Invalidate()
	return err
}

func (m *msAuth) AuthCode(uri, code string) {
	m.p = popups.NewGuiAuth(m.router.ui, uri, code)
	m.router.PushPopup(m.p)
}
