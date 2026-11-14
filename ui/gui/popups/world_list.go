package popups

import (
	"context"
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
)

type worldButton struct {
	*p2p.World
	widget.Clickable
}

type WorldList struct {
	g     guim.Guim
	close widget.Clickable
	list  widget.List
	state loadState

	setAddress func(world *p2p.World)
	worlds     []*worldButton
}

func NewWorldsList(g guim.Guim, setAddress func(world *p2p.World)) Popup {
	return &WorldList{
		g:          g,
		state:      loadStateInitial,
		setAddress: setAddress,
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

func (*WorldList) ID() string {
	return "FeaturedServers"
}

func (wl *WorldList) Close() error {
	return nil
}

var _ Popup = &WorldList{}

func (wl *WorldList) HandleEvent(event any) error {
	return nil
}

func (wl *WorldList) Load() error {
	account := auth.Auth.Account()
	if account == nil {
		return auth.ErrNotLoggedIn
	}
	ctx := context.Background()

	xblClient, err := account.XBLClient(ctx)
	if err != nil {
		return err
	}
	p2pClient := p2p.NewClient(xblClient)
	worlds, err := p2pClient.Worlds(ctx)
	if err != nil {
		return err
	}
	wl.worlds = nil
	for _, world := range worlds {
		wl.worlds = append(wl.worlds, &worldButton{
			World: &world,
		})
	}
	wl.state = loadStateLoaded
	return nil
}

func layoutp2pWorld(gtx layout.Context, th *material.Theme, world *worldButton) layout.Dimensions {
	return material.ButtonLayoutStyle{
		Background:   component.WithAlpha(th.Fg, 10),
		Button:       &world.Clickable,
		CornerRadius: Radius,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return material.Label(th, th.TextSize, fmt.Sprintf("%s\n%s", world.WorldName, world.HostName)).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return material.Label(th, th.TextSize*0.8, world.HandleID().String()).Layout(gtx)
				}),
			)
		})
	})
}

func (wl *WorldList) Layout(gtx C, th *material.Theme) D {
	for _, world := range wl.worlds {
		if world.Clicked(gtx) {
			wl.setAddress(world.World)
			wl.g.ClosePopup(wl.ID())
		}
	}

	if wl.close.Clicked(gtx) {
		wl.g.ClosePopup(wl.ID())
	}

	if wl.state == loadStateInitial {
		wl.state = loadStateLoading
		go func() {
			if auth.Auth.Account() == nil {
				auth.Auth.RequestLogin(wl.g.AccountName())
			}
			err := wl.Load()
			if err != nil {
				wl.g.Error(err)
				wl.g.ClosePopup(wl.ID())
			}
		}()
	}

	return LayoutPopupBackground(gtx, th, "Worlds", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Label(th, th.TextSize*1.4, "Worlds")
				return layout.Inset{
					Top:    8,
					Bottom: 8,
				}.Layout(gtx, t.Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				if wl.state == loadStateLoading {
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, 500)
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(20, 20)
						return material.Loader(th).Layout(gtx)
					})
				}
				if len(wl.worlds) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "couldnt find any worlds").Layout)
				}

				list := material.List(th, &wl.list)
				return list.Layout(gtx, len(wl.worlds), func(gtx C, index int) D {
					world := wl.worlds[index]
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutp2pWorld(gtx, th, world)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &wl.close, "Close")
				b.CornerRadius = Radius
				return b.Layout(gtx)
			}),
		)
	})
}
