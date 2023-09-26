package settings

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
)

type addressInput struct {
	Editor         widget.Editor
	showRealmsList widget.Bool
	l              sync.Mutex
	realmsList     widget.List
	realms         []realms.Realm
	realmsButtons  map[int]*widget.Clickable
	loading        bool
}

var AddressInput = &addressInput{
	Editor: widget.Editor{
		SingleLine: true,
	},
	realmsList: widget.List{
		List: layout.List{
			Axis: layout.Vertical,
		},
	},
}

func (a *addressInput) Value() string {
	return a.Editor.Text()
}

func (a *addressInput) getRealms() {
	var err error
	a.loading = true
	a.realms, err = utils.GetRealmsAPI().Realms(context.Background())
	a.realmsButtons = make(map[int]*widget.Clickable)
	for _, r := range a.realms {
		a.realmsButtons[r.ID] = &widget.Clickable{}
	}
	a.loading = false
	if err != nil {
		logrus.Error(err)
	}
}

func MulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 0xFF)
	return c
}

func (a *addressInput) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.ContrastFg, 8),
			},
		}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				e := material.Editor(th, &a.Editor, "Enter Server Address")
				e.LineHeight += 4
				return e.Layout(gtx)
			})
		})

	})
}

func (a *addressInput) LayoutRealms(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for k, c := range a.realmsButtons {
		if c.Clicked() {
			for _, r := range a.realms {
				if r.ID == k {
					a.Editor.SetText(fmt.Sprintf("realm:%s:%d", r.Name, r.ID))
				}
			}
		}
	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(15).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(material.Label(th, th.TextSize, "list realms").Layout),
					layout.Rigid(material.Switch(th, &a.showRealmsList, "realms").Layout),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.loading {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max = image.Pt(20, 20)
					return material.Loader(th).Layout(gtx)
				})
			}

			if a.showRealmsList.Value {
				if a.showRealmsList.Changed() {
					go a.getRealms()
				}
				a.l.Lock()
				defer a.l.Unlock()
				if len(a.realms) == 0 {
					return material.Label(th, th.TextSize, "you have no realms").Layout(gtx)
				}
				return material.List(th, &a.realmsList).Layout(gtx, len(a.realms), func(gtx layout.Context, index int) layout.Dimensions {
					entry := a.realms[index]
					return material.ButtonLayoutStyle{
						Background:   MulAlpha(th.Palette.Bg, 0x60),
						Button:       a.realmsButtons[entry.ID],
						CornerRadius: 3,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(15).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
							)
						})
					})
				})
			}
			return layout.Dimensions{}
		}),
	)
}
