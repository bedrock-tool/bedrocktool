package pages

import (
	"context"
	"errors"
	"image/color"
	"log"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/icons"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Router struct {
	g  guim.Guim
	th *material.Theme

	ctx          context.Context
	cmdCtx       context.Context
	cmdCtxCancel context.CancelFunc

	wg        sync.WaitGroup
	LogWidget func(layout.Context, *material.Theme) layout.Dimensions

	pages       map[string]func(g guim.Guim) Page
	currentPage Page

	ModalNavDrawer *component.ModalNavDrawer
	NavAnim        component.VisibilityAnimation
	AppBar         *component.AppBar
	ModalLayer     *component.ModalLayer
	NonModalDrawer bool

	loginButton     widget.Clickable
	switchButton    widget.Clickable
	updateButton    widget.Clickable
	updateAvailable bool

	logToggle widget.Bool
	showLogs  bool

	popups []popups.Popup
	toasts []*Toast

	ShuttingDown bool
}

func NewRouter(g guim.Guim, ctx context.Context) *Router {
	modal := component.NewModal()
	nav := component.NewNav("Navigation Drawer", "This is an example.")

	r := &Router{
		ctx: ctx,
		g:   g,

		pages:          make(map[string]func(g guim.Guim) Page),
		ModalLayer:     modal,
		ModalNavDrawer: component.ModalNavFrom(&nav, modal),
		AppBar:         component.NewAppBar(modal),
		NavAnim: component.VisibilityAnimation{
			State:    component.Invisible,
			Duration: time.Millisecond * 250,
		},
	}

	return r
}

func (r *Router) Tick(now time.Time) {
	r.toasts = slices.DeleteFunc(r.toasts, func(t *Toast) bool {
		return t.StartTime.Add(t.Duration).Before(now)
	})
}

func (r *Router) Register(p func(g guim.Guim) Page, id string) {
	r.pages[id] = p
}

func (r *Router) SwitchTo(tag string) {
	createPage, ok := r.pages[tag]
	if !ok {
		logrus.Errorf("unknown page %s", tag)
		return
	}
	page := createPage(r.g)

	r.currentPage = page
	r.AppBar.Title = page.NavItem().Name
	r.setActions()
	r.g.Invalidate()
}

func (r *Router) PushPopup(p popups.Popup) bool {
	for _, p2 := range r.popups {
		if p2.ID() == p.ID() {
			return false
		}
	}
	r.popups = append(r.popups, p)
	r.g.Invalidate()
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
		isIt := p.ID() == id
		if isIt {
			p.Close()
		}
		return isIt
	})
	r.g.Invalidate()
}

func (r *Router) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	r.th = th
	if r.updateButton.Clicked(gtx) {
		p := r.GetPopup("update")
		if p == nil {
			r.PushPopup(popups.NewUpdatePopup(r.g))
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
	paint.Fill(gtx.Ops, th.Palette.Bg)

	content := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X /= 3
				return r.ModalNavDrawer.NavDrawer.Layout(gtx, th, &r.NavAnim)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				d := r.currentPage.Layout(gtx, th)

				for _, p := range r.popups {
					p.Layout(gtx, th)
				}

				if r.logToggle.Value {
					r.LogWidget(gtx, th)
				}
				return d
			}),
		)
	})
	bar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return r.AppBar.Layout(gtx, th, "Menu", "Actions")
	})
	layout.Flex{Axis: layout.Vertical}.Layout(gtx, bar, content)
	r.ModalLayer.Layout(gtx, th)

	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		var dims layout.Dimensions
		for _, toast := range r.toasts {
			dims2 := layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return toast.Layout(gtx, th)
			})
			dims.Size.X = max(dims.Size.X, dims2.Size.X)
			dims.Size.X = max(dims.Size.Y, dims2.Size.Y)
		}
		return dims
	})

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (r *Router) layoutLoginButton(gtx layout.Context, fg, bg color.NRGBA) layout.Dimensions {
	account := auth.Auth.Account()
	if r.loginButton.Clicked(gtx) {
		if account == nil {
			go auth.Auth.RequestLogin(r.g.AccountName())
		} else {
			auth.Auth.Logout()
		}
	}

	var text = "Login"
	if account != nil {
		text = "Logout"
	}
	if account != nil && account.Name() != "" {
		text += " (" + account.Name() + ")"
	}
	button := material.Button(r.th, &r.loginButton, text)
	button.Background.R -= 20
	button.Background.G -= 20
	button.Background.B -= 32
	return layout.UniformInset(4).Layout(gtx, button.Layout)
}

func (r *Router) layoutSwitchInstance(gtx layout.Context, fg, bg color.NRGBA) layout.Dimensions {
	if r.switchButton.Clicked(gtx) {
		r.PushPopup(popups.NewSelectAccount(r.g))
	}
	button := material.Button(r.th, &r.switchButton, "Switch")
	button.Background.R -= 20
	button.Background.G -= 20
	button.Background.B -= 32
	return layout.UniformInset(4).Layout(gtx, button.Layout)
}

func (r *Router) setActions() {
	var extra []component.AppBarAction
	extra = append(extra, component.AppBarAction{Layout: r.layoutLoginButton})
	extra = append(extra, component.AppBarAction{Layout: r.layoutSwitchInstance})
	extra = append(extra, AppBarSwitch(&r.logToggle, "Logs", &r.th))

	if r.updateAvailable {
		extra = append(extra, component.SimpleIconAction(&r.updateButton, &icons.ActionUpdate, component.OverflowAction{}))
	}

	r.AppBar.SetActions(append(
		r.currentPage.Actions(r.th),
		extra...,
	), r.currentPage.Overflow())
}

func (r *Router) Execute(cmd commands.Command, settings any) {
	r.wg.Add(1)
	go func() {
		defer func() {
			if err, ok := recover().(error); ok {
				utils.ErrorHandler(err)
			}
		}()
		defer r.wg.Done()
		r.cmdCtx, r.cmdCtxCancel = context.WithCancel(r.ctx)

		err := cmd.Run(r.cmdCtx, settings)
		r.RemovePopup("connect")
		r.cmdCtx = nil
		r.cmdCtxCancel()
		r.cmdCtxCancel = nil
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Error(err)
			r.PushPopup(popups.NewErrorPopup(r.g, err, false, func() {
				r.SwitchTo("settings")
			}))
		}

		if page, ok := r.currentPage.(interface{ HaveFinishScreen() bool }); ok && page.HaveFinishScreen() {
			messages.SendEvent(&messages.EventSetUIState{
				State: messages.UIStateFinished,
			})
		} else {
			r.SwitchTo("settings")
		}
	}()
}

func (r *Router) ExitSubcommand() {
	if r.cmdCtxCancel != nil {
		r.cmdCtxCancel()
	} else {
		r.SwitchTo("settings")
	}
}

//

func (r *Router) HandleEvent(event any) error {
	switch event := event.(type) {
	case *messages.EventDisplayAuthCode:
		r.PushPopup(popups.NewGuiAuth(r.g, event.URI, event.AuthCode))

	case *messages.EventAuthFinished:
		r.RemovePopup("ms-auth")
		if event.Error != nil {
			if !errors.Is(event.Error, context.Canceled) {
				r.PushPopup(popups.NewErrorPopup(r.g, event.Error, false, nil))
			}
		}

	case *messages.EventConnectStateUpdate:
		if event.State == messages.ConnectStateBegin {
			r.PushPopup(popups.NewConnect(r.g, event.ListenAddr))
		}

	case *messages.EventUpdateAvailable:
		r.updateAvailable = true
		r.setActions()
		r.g.Invalidate()
	}

	for _, popup := range r.popups {
		err := popup.HandleEvent(event)
		if err != nil {
			logrus.Error(err)
		}
	}
	err := r.currentPage.HandleEvent(event)
	if err != nil {
		logrus.Error(err)
	}
	return nil
}

func (r *Router) Wait() {
	r.wg.Wait()
}

func (r *Router) Toast(t string) {
	r.toasts = append(r.toasts, &Toast{
		Message:   t,
		Visible:   true,
		StartTime: time.Now(),
		Duration:  5 * time.Second,
	})
}

func (r *Router) CloseLogs() {
	r.logToggle.Value = false
}
