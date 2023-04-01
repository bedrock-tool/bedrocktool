package pages

import (
	"context"
	"log"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type HandlerFunc = func(data interface{}) messages.MessageResponse

type Page interface {
	Actions() []component.AppBarAction
	Overflow() []component.OverflowAction
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	NavItem() component.NavItem

	// handle events from program
	Handler() HandlerFunc
}

type Router struct {
	Ctx        context.Context
	Wg         sync.WaitGroup
	Invalidate func()

	Theme *material.Theme

	pages   map[string]Page
	current string
	*component.ModalNavDrawer
	NavAnim component.VisibilityAnimation
	*component.AppBar
	*component.ModalLayer
	NonModalDrawer, BottomBar bool
}

func NewRouter(ctx context.Context, invalidate func(), th *material.Theme) Router {
	modal := component.NewModal()

	nav := component.NewNav("Navigation Drawer", "This is an example.")
	modalNav := component.ModalNavFrom(&nav, modal)

	bar := component.NewAppBar(modal)
	//bar.NavigationIcon = icon.MenuIcon

	na := component.VisibilityAnimation{
		State:    component.Invisible,
		Duration: time.Millisecond * 250,
	}
	return Router{
		Ctx:            ctx,
		Invalidate:     invalidate,
		Theme:          th,
		pages:          make(map[string]Page),
		ModalLayer:     modal,
		ModalNavDrawer: modalNav,
		AppBar:         bar,
		NavAnim:        na,
	}
}

func (r *Router) Register(tag string, p Page) {
	r.pages[tag] = p
	navItem := p.NavItem()
	navItem.Tag = tag
	if r.current == "" {
		r.current = tag
		r.AppBar.Title = navItem.Name
		r.AppBar.SetActions(p.Actions(), p.Overflow())
	}
	r.ModalNavDrawer.AddNavItem(navItem)
}

func (r *Router) SwitchTo(tag string) {
	p, ok := r.pages[tag]
	if !ok {
		return
	}
	navItem := p.NavItem()
	r.current = tag
	r.AppBar.Title = navItem.Name
	r.AppBar.SetActions(p.Actions(), p.Overflow())
}

func (r *Router) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
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
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X /= 3
				return r.NavDrawer.Layout(gtx, th, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return r.pages[r.current].Layout(gtx, th)
			}),
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
	page, ok := r.pages[r.current]
	if ok {
		return page.Handler()(data)
	}
	return messages.MessageResponse{}
}

var Pages = map[string]func(*Router) Page{}

func Register(name string, fun func(*Router) Page) {
	Pages[name] = fun
}
