package settings

import (
	"image"
	"image/color"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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
		// pick are the mode cards shown before a mode is picked
		pick map[string]*widget.Clickable
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
	// worlds first: it's what most people come for
	order := map[string]int{"worlds": 0, "skins": 1, "packs": 2}
	sort.Slice(p.cmdMenu.names, func(i, j int) bool {
		return order[p.cmdMenu.names[i]] < order[p.cmdMenu.names[j]]
	})

	p.cmdMenu.pick = make(map[string]*widget.Clickable)
	for _, name := range p.cmdMenu.names {
		p.cmdMenu.pick[name] = &widget.Clickable{}
	}
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
	for k, c := range p.cmdMenu.pick {
		if c.Clicked(gtx) {
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
				if p.cmdMenu.selected == "" {
					return D{} // the mode cards below do this
				}
				maxWidth := gtx.Constraints.Max.X
				maxButtonWidth := min(maxWidth, gtx.Dp(150))

				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    10,
						Bottom: 10,
					}.Layout(gtx, func(gtx C) D {
						var children []layout.FlexChild
						for _, name := range p.cmdMenu.names {
							clickable := p.cmdMenu.clickables[name]
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								button := material.Button(th, clickable.Clickable, modeTitle(clickable.Text))
								button.CornerRadius = popups.Radius
								button.Inset = layout.Inset{Top: 9, Bottom: 9, Left: 14, Right: 14}
								if p.cmdMenu.selected == name {
									button.Background = th.ContrastBg
									button.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
								} else {
									button.Background = surface(th, 22)
									button.Color = th.Fg
								}

								gtx.Constraints.Min.X = maxButtonWidth * 2 / 3
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
					d = layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(520))
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						children := []layout.FlexChild{
							layout.Rigid(func(gtx C) D {
								l := material.Label(th, th.TextSize*1.5, "What do you want to save?")
								return layout.Inset{Bottom: 14}.Layout(gtx, l.Layout)
							}),
						}
						for _, name := range p.cmdMenu.names {
							click := p.cmdMenu.pick[name]
							desc := p.settings[name].cmd.Description()
							children = append(children, layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
									return modeCard(gtx, th, click, modeTitle(name), desc)
								})
							}))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
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
				if p.cmdMenu.selected == "" {
					return D{}
				}
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Inset{
						Top:    15,
						Bottom: 15,
					}.Layout(gtx, func(gtx C) D {
						width := min(gtx.Constraints.Max.X, max(gtx.Dp(260), gtx.Constraints.Max.X/3))
						gtx.Constraints = layout.Exact(image.Pt(width, gtx.Dp(44)))
						b := material.Button(th, &p.startButton, "Start")
						b.CornerRadius = popups.Radius
						b.Background = th.ContrastBg
						b.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
						if !validSettings {
							b.Background = surface(th, 30)
							b.Color = muted(th)
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

// modeTitle is a mode's name as its button shows it ("worlds" -> "Worlds").
func modeTitle(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// modeCard is a mode to pick: its name and what it does, the whole card
// clickable.
func modeCard(gtx C, th *material.Theme, click *widget.Clickable, title, desc string) D {
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				bg := surface(th, 12)
				if click.Hovered() {
					bg = th.ContrastBg
					bg.A = 40
				}
				rr := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(popups.Radius)).Push(gtx.Ops)
				paint.Fill(gtx.Ops, bg)
				rr.Pop()
				return D{Size: gtx.Constraints.Min}
			},
			func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(14).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							l := material.Body1(th, title)
							l.Font.Weight = font.SemiBold
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							l := material.Body2(th, desc)
							l.Color = muted(th)
							return layout.Inset{Top: 2}.Layout(gtx, l.Layout)
						}),
					)
				})
			},
		)
	})
}
