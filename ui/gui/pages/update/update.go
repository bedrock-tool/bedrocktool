package update

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "update"

type Page struct {
	g guim.Guim

	percentDownload int
}

func New(g guim.Guim) pages.Page {
	return &Page{
		g: g,
	}
}

func (p *Page) Actions(th *material.Theme) []component.AppBarAction {
	return nil
}

func (p *Page) HandleEvent(event any) error {
	switch event := event.(type) {
	case *messages.EventUpdateDownloadProgress:
		p.percentDownload = event.Progress
	}
	return nil
}

func (p *Page) ID() string {
	return ID
}

func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return material.Body1(th, fmt.Sprintf("downloading (%d%%)", p.percentDownload)).Layout(gtx)
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return nil
}
