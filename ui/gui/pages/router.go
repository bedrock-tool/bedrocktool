package pages

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/gui/icons"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type Router struct {
	UI         ui.UI
	Ctx        context.Context
	Wg         sync.WaitGroup
	MSAuth     *guiAuth
	Invalidate func()

	Theme *material.Theme

	pages   map[string]Page
	current string
	*component.ModalNavDrawer
	NavAnim component.VisibilityAnimation
	*component.AppBar
	*component.ModalLayer
	NonModalDrawer, BottomBar bool

	UpdateButton    *widget.Clickable
	updateAvailable bool
}

func NewRouter(ctx context.Context, invalidate func(), th *material.Theme) *Router {
	modal := component.NewModal()

	nav := component.NewNav("Navigation Drawer", "This is an example.")
	modalNav := component.ModalNavFrom(&nav, modal)

	bar := component.NewAppBar(modal)
	//bar.NavigationIcon = icon.MenuIcon

	na := component.VisibilityAnimation{
		State:    component.Invisible,
		Duration: time.Millisecond * 250,
	}
	r := &Router{
		Ctx:            ctx,
		Invalidate:     invalidate,
		Theme:          th,
		MSAuth:         &guiAuth{},
		pages:          make(map[string]Page),
		ModalLayer:     modal,
		ModalNavDrawer: modalNav,
		AppBar:         bar,
		NavAnim:        na,

		UpdateButton: &widget.Clickable{},
	}
	r.MSAuth.router = r
	return r
}

func (r *Router) Register(p Page) {
	r.pages[p.ID()] = p
	navItem := p.NavItem()
	navItem.Tag = p.ID()
	if r.current == "" {
		r.current = p.ID()
		r.AppBar.Title = navItem.Name
		r.AppBar.SetActions(p.Actions(), p.Overflow())
	}
	r.ModalNavDrawer.AddNavItem(navItem)
}

func (r *Router) SwitchToPageTemp(p Page) {
	r.pages[p.ID()+"_temp"] = p
	r.SwitchTo(p.ID() + "_temp")
}

func (r *Router) SwitchTo(tag string) {
	if strings.HasSuffix(r.current, "_temp") {
		delete(r.pages, r.current)
	}
	p, ok := r.pages[tag]
	if !ok {
		return
	}
	navItem := p.NavItem()
	r.current = tag
	r.AppBar.Title = navItem.Name
	actions := p.Actions()
	if r.updateAvailable {
		actions = append(actions, component.SimpleIconAction(r.UpdateButton, &icons.ActionUpdate, component.OverflowAction{}))
	}
	r.AppBar.SetActions(actions, p.Overflow())
}

func (r *Router) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if r.UpdateButton.Clicked() {
		r.SwitchTo("update")
	}

	for _, event := range r.AppBar.Events(gtx) {
		switch event := event.(type) {
		case component.AppBarNavigationClicked:
			if r.NonModalDrawer {
				r.NavAnim.ToggleVisibility(gtx.Now)
			} else {
				r.ModalNavDrawer.Appear(gtx.Now)
				r.NavAnim.Disappear(gtx.Now)
			}
		case component.AppBarContextMenuDismissed:
			log.Printf("Context menu dismissed: %v", event)
		case component.AppBarOverflowActionClicked:
			log.Printf("Overflow action selected: %v", event)
		}
	}
	if r.ModalNavDrawer.NavDestinationChanged() {
		r.SwitchTo(r.ModalNavDrawer.CurrentNavDestination().(string))
	}
	paint.Fill(gtx.Ops, th.Palette.Bg)
	content := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X /= 3
						return r.NavDrawer.Layout(gtx, th, &r.NavAnim)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return r.pages[r.current].Layout(gtx, th)
					}),
				)
			}),
			layout.Stacked(r.MSAuth.Layout),
		)
	})
	bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return r.AppBar.Layout(gtx, th, "Menu", "Actions")
	})
	flex := layout.Flex{Axis: layout.Vertical}
	if r.BottomBar {
		flex.Layout(gtx, content, bar)
	} else {
		flex.Layout(gtx, bar, content)
	}
	r.ModalLayer.Layout(gtx, th)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (r *Router) Handler(data interface{}) messages.MessageResponse {
	switch data.(type) {
	case messages.UpdateAvailable:
		r.updateAvailable = true
		p, ok := r.pages[r.current]
		if ok {
			r.AppBar.SetActions(append(p.Actions(), component.SimpleIconAction(r.UpdateButton, &icons.ActionUpdate, component.OverflowAction{})), p.Overflow())
		}
		r.Invalidate()
	case messages.ConnectState:
		if r.current != "connect_temp" {
			r.SwitchToPageTemp(NewConnect(r, r.pages[r.current]))
		}
	}

	page, ok := r.pages[r.current]
	if ok {
		return page.Handler(data)
	}
	return messages.MessageResponse{}
}

func (r *Router) Execute(cmd commands.Command) {
	r.Wg.Add(1)
	go func() {
		defer r.Wg.Done()

		err := cmd.Execute(r.Ctx, r.UI)
		if err != nil {
			logrus.Error(err)
		}
	}()
}

var NewConnect func(router *Router, afterEstablish Page) Page
