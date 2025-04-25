package settings

import (
	"image"
	"runtime"
	"slices"
	"sort"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "settings"

type cmdItem struct {
	Clickable *widget.Clickable
	Text      string
}

type Page struct {
	g guim.Guim

	cmdMenu struct {
		clickables map[string]cmdItem
		names      []string
		state      component.GridState
		selected   string
	}

	settings    map[string]*settingsPage
	startButton widget.Clickable
	closePopup  *component.ScrimStyle
}

func New(g guim.Guim) pages.Page {
	p := &Page{
		g: g,

		settings: make(map[string]*settingsPage),

		closePopup: &component.ScrimStyle{
			ScrimState: &component.ScrimState{
				VisibilityAnimation: component.VisibilityAnimation{
					Duration: 150 * time.Millisecond,
					State:    component.Invisible,
				},
			},
			FinalAlpha: 128,
		},
	}

	for k, cmd := range commands.Registered {
		if !slices.Contains([]string{"worlds", "skins", "packs"}, k) {
			continue
		}
		settingUI := &settingsPage{cmd: cmd, g: g}
		settingUI.Init()
		p.settings[k] = settingUI
		p.cmdMenu.names = append(p.cmdMenu.names, k)
	}
	sort.Strings(p.cmdMenu.names)

	p.cmdMenu.clickables = make(map[string]cmdItem, len(commands.Registered))
	for _, name := range p.cmdMenu.names {
		p.cmdMenu.clickables[name] = cmdItem{
			Clickable: &widget.Clickable{},
			Text:      name,
		}
	}

	return p
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return ID
}

func (p *Page) Actions(th *material.Theme) []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Settings",
		//Icon: icon.OtherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	var validSettings = false
	if p.cmdMenu.selected != "" {
		s := p.settings[p.cmdMenu.selected]
		validSettings = s.Valid()
	}

	if p.startButton.Clicked(gtx) && validSettings {
		if p.cmdMenu.selected != "" {
			settingsUI, ok := p.settings[p.cmdMenu.selected]
			if !ok {
				logrus.Errorf("Cmd %s not found", p.cmdMenu.selected)
				return D{}
			}

			settings, err := settingsUI.Apply()
			if err != nil {
				p.g.ShowPopup(popups.NewErrorPopup(p.g, err, false, nil))
			} else {
				p.g.StartSubcommand(p.cmdMenu.selected, settings)
			}
		}
	}

	for k, c := range p.cmdMenu.clickables {
		if c.Clickable.Clicked(gtx) {
			p.cmdMenu.selected = k
		}
	}

	dims := layout.UniformInset(7).Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceBetween,
		}.Layout(gtx,
			// Select Command Button
			layout.Rigid(func(gtx C) D {
				maxWidth := gtx.Constraints.Max.X
				maxButtonWidth := min(maxWidth, gtx.Dp(150))
				//minButtonWidth := maxButtonWidth

				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    10,
						Bottom: 10,
					}.Layout(gtx, func(gtx C) D {
						var children []layout.FlexChild
						for _, name := range p.cmdMenu.names {
							clickable := p.cmdMenu.clickables[name]
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								button := material.Button(th, clickable.Clickable, clickable.Text)
								if p.cmdMenu.selected == name {
									button.Background = th.Fg
									button.Color = th.Bg
								}

								gtx.Constraints.Min.X = maxButtonWidth / 2
								gtx.Constraints.Max.X = maxButtonWidth
								return layout.Inset{
									Left:  5,
									Right: 5,
								}.Layout(gtx, button.Layout)
							}))
						}
						return layout.Flex{
							Axis: layout.Horizontal,
						}.Layout(gtx, children...)
					})
				})

			}),

			layout.Flexed(1, func(gtx C) (d D) {
				if p.cmdMenu.selected == "" {
					d = layout.Center.Layout(gtx, material.H5(th, "Select a Mode").Layout)
					d.Size.Y = gtx.Constraints.Max.Y
				} else {
					s := p.settings[p.cmdMenu.selected]
					return layout.Inset{
						Left:  min(unit.Dp(gtx.Constraints.Max.X/gtx.Dp(10)), 8),
						Right: min(unit.Dp(gtx.Constraints.Max.X/gtx.Dp(10)), 8),
					}.Layout(gtx, func(gtx C) D {
						return s.Layout(gtx, th)
					})
				}
				return d
			}),

			// Start Button
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Inset{
						Top:    15,
						Bottom: 15,
					}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min = image.Pt(gtx.Dp(300), gtx.Dp(40))
						gtx.Constraints.Max = image.Pt(gtx.Constraints.Max.X/3, gtx.Dp(40))
						b := material.Button(th, &p.startButton, "Start")
						if !validSettings {
							b.Color = th.Bg
							b.Background = th.Fg
						}
						return b.Layout(gtx)
					})
				})
			}),
		)
	})

	s := p.settings[p.cmdMenu.selected]
	if s != nil && (runtime.GOOS == "android") {
		isActive := s.PopupActive()
		if isActive != p.closePopup.Visible() {
			p.closePopup.ToggleVisibility(gtx.Now)
		}

		if p.closePopup.Clicked(gtx) {
			p.closePopup.ToggleVisibility(gtx.Now)
			gtx.Execute(key.FocusCmd{Tag: ""})
		}

		return p.closePopup.Clickable.Layout(gtx, func(gtx C) D {
			if !p.closePopup.Visible() {
				return D{}
			}
			gtx.Constraints.Min = gtx.Constraints.Max
			alpha := p.closePopup.FinalAlpha
			if p.closePopup.Animating() {
				alpha = uint8(float32(p.closePopup.FinalAlpha) * p.closePopup.Revealed(gtx))
			}
			dim := component.Rect{
				Color: component.WithAlpha(p.closePopup.Color, alpha),
				Size:  gtx.Constraints.Max,
			}.Layout(gtx)
			s.LayoutPopupInput(gtx, th)
			return dim
		})
	}

	return dims
}

func (p *Page) HandleEvent(event any) error {
	return nil
}
