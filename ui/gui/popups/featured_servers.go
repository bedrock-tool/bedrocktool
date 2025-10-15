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
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
)

type loadState int

const (
	loadStateInitial = 0
	loadStateLoading = 1
	loadStateLoaded  = 2
)

type serverButton struct {
	*discovery.FeaturedServer
	widget.Clickable
}

type FeaturedServers struct {
	g     guim.Guim
	close widget.Clickable
	list  widget.List
	state loadState

	setAddress func(server *discovery.FeaturedServer)
	servers    []*serverButton
}

func NewFeaturedServers(g guim.Guim, setAddress func(server *discovery.FeaturedServer)) Popup {
	return &FeaturedServers{
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

func (*FeaturedServers) ID() string {
	return "FeaturedServers"
}

func (fs *FeaturedServers) Close() error {
	return nil
}

var _ Popup = &FeaturedServers{}

func (fs *FeaturedServers) HandleEvent(event any) error {
	return nil
}

func (fs *FeaturedServers) Load() error {
	account := auth.Auth.Account()
	if account == nil {
		return auth.ErrNotLoggedIn
	}
	ctx := context.Background()
	gatheringsClient, err := account.Gatherings(ctx)
	if err != nil {
		return err
	}

	servers, err := gatheringsClient.GetFeaturedServers(ctx)
	if err != nil {
		return err
	}
	fs.servers = nil
	for _, server := range servers {
		fs.servers = append(fs.servers, &serverButton{
			FeaturedServer: &server,
		})
	}
	fs.state = loadStateLoaded
	return nil
}

func layoutFeaturedServer(gtx layout.Context, th *material.Theme, server *serverButton) layout.Dimensions {
	return material.ButtonLayoutStyle{
		Background:   component.WithAlpha(th.Fg, 10),
		Button:       &server.Clickable,
		CornerRadius: 8,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return material.Label(th, th.TextSize, server.Name).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					address := server.Address
					if server.ExperienceId != "" {
						address = "id: " + server.ExperienceId
					}
					return material.Label(th, th.TextSize*0.8, address).Layout(gtx)
				}),
			)
		})
	})
}

func (fs *FeaturedServers) Layout(gtx C, th *material.Theme) D {
	for _, server := range fs.servers {
		if server.Clicked(gtx) {
			fs.setAddress(server.FeaturedServer)
			fs.g.ClosePopup(fs.ID())
		}
	}

	if fs.close.Clicked(gtx) {
		fs.g.ClosePopup(fs.ID())
	}

	if fs.state == loadStateInitial {
		fs.state = loadStateLoading
		go func() {
			if auth.Auth.Account() == nil {
				auth.Auth.RequestLogin(fs.g.AccountName())
			}
			err := fs.Load()
			if err != nil {
				fs.g.Error(err)
				fs.g.ClosePopup(fs.ID())
			}
		}()
	}

	return LayoutPopupBackground(gtx, th, "Featured Servers", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t := material.Label(th, th.TextSize*1.4, "Featured Servers")
				return layout.Inset{
					Top:    8,
					Bottom: 8,
				}.Layout(gtx, t.Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				if fs.state == loadStateLoading {
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, 500)
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(20, 20)
						return material.Loader(th).Layout(gtx)
					})
				}
				if len(fs.servers) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "couldnt find any featured servers ?!?").Layout)
				}

				list := material.List(th, &fs.list)
				return list.Layout(gtx, len(fs.servers), func(gtx C, index int) D {
					server := fs.servers[index]
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutFeaturedServer(gtx, th, server)
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &fs.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})
}
