package pages

import (
	"context"
	"errors"
	"image/color"
	"log"
	"reflect"
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
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/gregwebs/go-recovery"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Router struct {
	ui           ui.UI
	Ctx          context.Context
	cmdCtx       context.Context
	cmdCtxCancel context.CancelFunc
	Wg           sync.WaitGroup
	msAuth       *msAuth
	Invalidate   func()
	LogWidget    func(layout.Context, *material.Theme) layout.Dimensions

	Theme       *material.Theme
	pages       map[string]func(ui.UI) Page
	currentPage Page

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

	popups []popups.Popup
}

func NewRouter(uii ui.UI) *Router {
	modal := component.NewModal()
	nav := component.NewNav("Navigation Drawer", "This is an example.")

	r := &Router{
		ui:             uii,
		pages:          make(map[string]func(ui.UI) Page),
		msAuth:         &msAuth{},
		ModalLayer:     modal,
		ModalNavDrawer: component.ModalNavFrom(&nav, modal),
		AppBar:         component.NewAppBar(modal),
		NavAnim: component.VisibilityAnimation{
			State:    component.Invisible,
			Duration: time.Millisecond * 250,
		},
	}
	r.msAuth.router = r
	utils.Auth.MSHandler = r.msAuth

	return r
}

func (r *Router) Register(p func(ui.UI) Page, id string) {
	r.pages[id] = p
}

func (r *Router) SwitchTo(tag string) {
	createPage, ok := r.pages[tag]
	if !ok {
		logrus.Errorf("unknown page %s", tag)
		return
	}
	page := createPage(r.ui)

	r.currentPage = page
	r.AppBar.Title = page.NavItem().Name
	r.setActions()
	r.Invalidate()
}

func (r *Router) PushPopup(p popups.Popup) bool {
	for _, p2 := range r.popups {
		if p2.ID() == p.ID() {
			return false
		}
	}
	r.popups = append(r.popups, p)
	r.Invalidate()
	return true
}

func (r *Router) GetPopup(id string) (p popups.Popup) {
	for _, p := range r.popups {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

func (r *Router) RemovePopup(id string) {
	r.popups = slices.DeleteFunc(r.popups, func(p popups.Popup) bool {
		return p.ID() == id
	})
	r.Invalidate()
}

func (r *Router) Layout(gtx layout.Context) layout.Dimensions {
	if r.updateButton.Clicked(gtx) {
		p := r.GetPopup("update")
		if p == nil {
			r.PushPopup(popups.NewUpdatePopup(r.ui))
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

	content := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X /= 3
				return r.ModalNavDrawer.NavDrawer.Layout(gtx, r.Theme, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				d := r.currentPage.Layout(gtx, r.Theme)

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
	bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
		return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				layout.Rigid(material.Switch(r.Theme, &r.logToggle, "logs").Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
		r.currentPage.Actions(),
		extra...,
	), r.currentPage.Overflow())
}

func (r *Router) HandleMessage(msg *messages.Message) *messages.Message {
	if true {
		tm := reflect.TypeOf(msg.Data)
		logrus.Debugf("Message from: %s, %s", msg.Source, tm.String())
	}

	switch data := msg.Data.(type) {
	case messages.UpdateAvailable:
		r.updateAvailable = true
		r.setActions()
		if r.Invalidate != nil {
			r.Invalidate()
		}
	case messages.ConnectState:
		if data == messages.ConnectStateBegin {
			r.PushPopup(popups.NewConnect(r.ui))
		}

	case messages.ShowPopup:
		r.PushPopup(data.Popup.(popups.Popup))

	case messages.StartSubcommand:
		cmd := data.Command.(commands.Command)
		r.SwitchTo(cmd.Name())
		r.Execute(cmd)
	case messages.ExitSubcommand:
		r.ExitCommand()

	case messages.Close:
		switch msg.SourceType {
		case "popup":
			r.RemovePopup(msg.Source)
		}
	}

	for _, p := range r.popups {
		p.HandleMessage(msg)
	}

	resp := r.currentPage.HandleMessage(msg)
	r.Invalidate()
	return resp
}

func (r *Router) Execute(cmd commands.Command) {
	r.Wg.Add(1)
	go func() {
		defer r.Wg.Done()
		r.cmdCtx, r.cmdCtxCancel = context.WithCancel(r.Ctx)

		recovery.ErrorHandler = func(err error) {
			utils.PrintPanic(err)
			r.RemovePopup("connect")
			r.PushPopup(popups.NewErrorPopup(r.ui, err, func() {
				r.SwitchTo("settings")
			}, true))
		}

		defer func() {
			if err, ok := recover().(error); ok {
				recovery.ErrorHandler(err)
			}
		}()

		err := cmd.Execute(r.cmdCtx, r.ui)
		r.RemovePopup("connect")
		r.cmdCtx = nil
		r.cmdCtxCancel = nil
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Error(err)
			r.PushPopup(popups.NewErrorPopup(r.ui, err, func() {
				r.SwitchTo("settings")
			}, false))
		}

		resp := r.HandleMessage(&messages.Message{
			Source: "router",
			Data:   messages.HaveFinishScreen{},
		})
		if resp != nil {
			r.HandleMessage(&messages.Message{
				Source: "router",
				Data:   messages.UIStateFinished,
			})
		} else {
			r.SwitchTo("settings")
		}
	}()
}

func (r *Router) ExitCommand() {
	if r.cmdCtxCancel != nil {
		r.cmdCtxCancel()
	} else {
		r.SwitchTo("settings")
	}
}
