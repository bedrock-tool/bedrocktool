package pages

import (
	"context"
	"image/color"
	"log"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/gui/icons"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/gregwebs/go-recovery"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Router struct {
	UI         ui.UI
	ctx        context.Context
	Wg         sync.WaitGroup
	MSAuth     *guiAuth
	Invalidate func()
	LogWidget  func(C, *material.Theme) D

	Theme   *material.Theme
	pages   map[string]func(*Router) Page
	current Page

	ModalNavDrawer *component.ModalNavDrawer
	NavAnim        component.VisibilityAnimation
	AppBar         *component.AppBar
	ModalLayer     *component.ModalLayer
	NonModalDrawer bool
	BottomBar      bool

	updateButton    widget.Clickable
	updateAvailable bool

	logToggle widget.Bool
	showLogs  bool

	popups []Popup
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
		ctx:            ctx,
		Invalidate:     invalidate,
		Theme:          th,
		pages:          make(map[string]func(*Router) Page),
		MSAuth:         &guiAuth{},
		ModalLayer:     modal,
		ModalNavDrawer: modalNav,
		AppBar:         bar,
		NavAnim:        na,
	}
	r.MSAuth.router = r
	return r
}

func (r *Router) Register(p func(*Router) Page, id string) {
	r.pages[id] = p
}

func (r *Router) SwitchTo(tag string) {
	pf, ok := r.pages[tag]
	if !ok {
		logrus.Errorf("unknown page %s", tag)
		return
	}
	p := pf(r)

	navItem := p.NavItem()
	r.current = p
	r.AppBar.Title = navItem.Name
	r.setActions()
	r.Invalidate()
}

func (r *Router) PushPopup(p Popup) bool {
	for _, p2 := range r.popups {
		if p2.ID() == p.ID() {
			//logrus.Debugf("Attempted to push popup already open %s", p.ID())
			return false
		}
	}
	r.popups = append(r.popups, p)
	r.Invalidate()
	return true
}

func (r *Router) GetPopup(id string) (p Popup) {
	for _, p := range r.popups {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

func (r *Router) RemovePopup(id string) {
	r.popups = slices.DeleteFunc(r.popups, func(p Popup) bool {
		return p.ID() == id
	})
	r.Invalidate()
}

func (r *Router) Layout(gtx layout.Context) layout.Dimensions {
	if r.updateButton.Clicked(gtx) {
		if p, ok := r.GetPopup("update").(*UpdatePopup); ok {
			if !p.updating {
				r.RemovePopup("update")
			}
		} else {
			r.PushPopup(NewUpdatePopup(r))
		}
	}

	if r.logToggle.Value != r.showLogs {
		r.showLogs = r.logToggle.Value
		r.setActions()
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
	paint.Fill(gtx.Ops, r.Theme.Palette.Bg)

	content := layout.Flexed(1, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 3
				return r.ModalNavDrawer.NavDrawer.Layout(gtx, r.Theme, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx C) D {
				d := r.current.Layout(gtx, r.Theme)

				for _, p := range r.popups {
					p.Layout(gtx, r.Theme)
				}

				if r.logToggle.Value {
					r.LogWidget(gtx, r.Theme)
				}
				return d
			}),
		)
	})
	bar := layout.Rigid(func(gtx C) D {
		return r.AppBar.Layout(gtx, r.Theme, "Menu", "Actions")
	})
	flex := layout.Flex{Axis: layout.Vertical}
	if r.BottomBar {
		flex.Layout(gtx, content, bar)
	} else {
		flex.Layout(gtx, bar, content)
	}
	r.ModalLayer.Layout(gtx, r.Theme)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (r *Router) setActions() {
	var extra []component.AppBarAction
	extra = append(extra, component.AppBarAction{Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
		return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(material.Switch(r.Theme, &r.logToggle, "logs").Layout),
				layout.Rigid(func(gtx C) D {
					l := material.Label(r.Theme, 12, "Logs")
					l.Alignment = text.Middle
					return layout.UniformInset(5).Layout(gtx, l.Layout)
				}),
			)
		})
	}})

	if r.updateAvailable {
		extra = append(extra, component.SimpleIconAction(&r.updateButton, &icons.ActionUpdate, component.OverflowAction{}))
	}

	r.AppBar.SetActions(append(
		r.current.Actions(),
		extra...,
	), r.current.Overflow())
}

func (r *Router) Handler(data interface{}) messages.Response {
	switch data := data.(type) {
	case messages.UpdateAvailable:
		r.updateAvailable = true
		r.setActions()
		r.Invalidate()
	case messages.ConnectState:
		if data == messages.ConnectStateBegin {
			r.PushPopup(NewConnect(r))
		}
	}

	for _, p := range r.popups {
		p.Handler(data)
	}

	return r.current.Handler(data)
}

func (r *Router) Execute(cmd commands.Command) {
	r.Wg.Add(1)
	go func() {
		defer r.Wg.Done()

		recovery.ErrorHandler = func(err error) {
			utils.PrintPanic(err)
			r.PushPopup(NewErrorPopup(r, err, func() {
				r.RemovePopup("connect")
				r.SwitchTo("settings")
			}, true))
		}

		defer func() {
			if err, ok := recover().(error); ok {
				recovery.ErrorHandler(err)
			}
		}()

		err := cmd.Execute(r.ctx, r.UI)
		if err != nil {
			logrus.Error(err)
			r.PushPopup(NewErrorPopup(r, err, func() {
				r.RemovePopup("connect")
				r.SwitchTo("settings")
			}, false))
		}
	}()
}
