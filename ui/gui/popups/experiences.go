package popups

import (
	"context"
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/sandertv/gophertunnel/minecraft/service/gatherings"
)

type experienceButton struct {
	*gatherings.Experience
	widget.Clickable
}

type Experiences struct {
	g     guim.Guim
	close widget.Clickable
	list  widget.List
	state loadState

	setExperience func(experience *gatherings.Experience)
	experiences   []*experienceButton
}

func NewExperiences(g guim.Guim, setExperience func(experience *gatherings.Experience)) Popup {
	return &Experiences{
		g:             g,
		setExperience: setExperience,
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

func (*Experiences) ID() string {
	return "Experiences"
}

func (*Experiences) Close() error {
	return nil
}

var _ Popup = &Experiences{}

func (*Experiences) HandleEvent(event any) error {
	return nil
}

func (e *Experiences) Load() error {
	account := auth.Auth.Account()
	if account == nil {
		return auth.ErrNotLoggedIn
	}
	ctx := context.Background()
	gatheringsClient, err := account.Gatherings(ctx)
	if err != nil {
		return err
	}
	experiences, err := gatheringsClient.Experiences(ctx)
	if err != nil {
		return err
	}
	e.experiences = nil
	for _, experience := range experiences {
		e.experiences = append(e.experiences, &experienceButton{
			Experience: experience,
		})
	}
	e.state = loadStateLoaded
	return nil
}

func layoutExperience(gtx layout.Context, th *material.Theme, experience *experienceButton) layout.Dimensions {
	return material.ButtonLayoutStyle{
		Background:   component.WithAlpha(th.Fg, 10),
		Button:       &experience.Clickable,
		CornerRadius: 8,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			name := experience.Item.Title.Neutral()
			if name == "" {
				name = experience.NewsTitle
			}
			if name == "" {
				name = experience.ID.String()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return material.Label(th, th.TextSize, name).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					subtitle := experience.CreatorName
					if subtitle == "" {
						subtitle = experience.NewsTitle
					}
					return material.Label(th, th.TextSize*0.8, subtitle).Layout(gtx)
				}),
			)
		})
	})
}

func (e *Experiences) Layout(gtx C, th *material.Theme) D {
	for _, experience := range e.experiences {
		if experience.Clicked(gtx) {
			e.setExperience(experience.Experience)
			e.g.ClosePopup(e.ID())
			return D{}
		}
	}

	if e.close.Clicked(gtx) {
		e.g.ClosePopup(e.ID())
	}

	if e.state == loadStateInitial {
		e.state = loadStateLoading
		go func() {
			if auth.Auth.Account() == nil {
				auth.Auth.RequestLogin(e.g.AccountName())
			}
			err := e.Load()
			if err != nil {
				e.g.Error(err)
				e.g.ClosePopup(e.ID())
			}
		}()
	}

	return LayoutPopupBackground(gtx, th, "Experiences", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Label(th, th.TextSize*1.4, "Experiences")
				return layout.Inset{
					Top:    8,
					Bottom: 8,
				}.Layout(gtx, t.Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				if e.state == loadStateLoading {
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, 500)
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(20, 20)
						return material.Loader(th).Layout(gtx)
					})
				}
				if len(e.experiences) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "couldnt find any experiences").Layout)
				}

				list := material.List(th, &e.list)
				return list.Layout(gtx, len(e.experiences), func(gtx C, index int) D {
					experience := e.experiences[index]
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutExperience(gtx, th, experience)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &e.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})
}
