package popups

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
)

type Gatherings struct {
	close        widget.Clickable
	l            sync.Mutex
	list         widget.List
	gatherings   []*gatheringButton
	loaded       bool
	loading      bool
	setGathering func(*discovery.Gathering)
	t            *time.Ticker
}

type gatheringButton struct {
	*discovery.Gathering
	widget.Clickable
}

func NewGatherings(setGathering func(*discovery.Gathering)) Popup {
	return &Gatherings{
		setGathering: setGathering,
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

func (g *Gatherings) HandleMessage(msg *messages.Message) *messages.Message {
	switch data := msg.Data.(type) {
	case messages.Close:
		if data.ID != g.ID() {
			return nil
		}
		if g.t != nil {
			g.t.Stop()
		}
	}
	return nil
}

func (*Gatherings) ID() string {
	return "Gatherings"
}

var _ Popup = &Gatherings{}

func (g *Gatherings) Load() error {
	if !utils.Auth.LoggedIn() {
		return errors.New("not Logged In")
	}

	gatheringsClient, err := utils.Auth.Gatherings(context.TODO())
	if err != nil {
		return err
	}

	gatherings, err := gatheringsClient.Gatherings(context.TODO())
	if err != nil {
		return err
	}

	g.gatherings = nil
	for _, gathering := range gatherings {
		gg := &gatheringButton{
			Gathering: gathering,
		}
		g.gatherings = append(g.gatherings, gg)
	}

	g.loading = false
	g.loaded = true

	g.t = time.NewTicker(1 * time.Second)
	go func() {
		for range g.t.C {
			messages.Router.Handle(&messages.Message{
				Source: "Gatherings",
				Target: "ui",
				Data:   messages.Invalidate{},
			})
		}
	}()

	return nil
}

func (g *Gatherings) Layout(gtx C, th *material.Theme) D {
	for _, gg := range g.gatherings {
		if gg.Clicked(gtx) {
			g.setGathering(gg.Gathering)
			g.close.Click()
		}
	}

	if g.close.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: g.ID(),
			Target: "ui",
			Data:   messages.Close{Type: "popup", ID: g.ID()},
		})
	}

	if !g.loaded && !g.loading {
		g.loading = true
		go func() {
			if !utils.Auth.LoggedIn() {
				messages.Router.Handle(&messages.Message{
					Source: g.ID(),
					Target: "ui",
					Data:   messages.RequestLogin{Wait: true},
				})
			}
			err := g.Load()
			if err != nil {
				messages.Router.Handle(&messages.Message{
					Source: g.ID(),
					Target: "ui",
					Data:   messages.Error(err),
				})
				messages.Router.Handle(&messages.Message{
					Source: g.ID(),
					Target: "ui",
					Data: messages.Close{
						Type: "popup",
						ID:   g.ID(),
					},
				})
			}
		}()
	}

	return LayoutPopupBackground(gtx, th, "Realms", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				if g.loading {
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(20, 20)
						return material.Loader(th).Layout(gtx)
					})
				}

				g.l.Lock()
				defer g.l.Unlock()
				if len(g.gatherings) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "there are no gatherings active").Layout)
				}

				l := material.List(th, &g.list)
				l.AnchorStrategy = material.Overlay
				return l.Layout(gtx, len(g.gatherings), func(gtx C, index int) D {
					gathering := g.gatherings[index]

					var start = time.Now().Add(-1 * time.Hour)
					var end = gathering.EndTimeUtc
					var gatheringName = gathering.Title + " (" + gathering.GatheringID + ")"
					for _, segment := range gathering.Segments {
						if segment.UI.CaptionText == "gathering.caption.endsIn" {
							start = segment.StartTimeUtc
						}
					}
					hasStarted := time.Since(start) > 0

					var click *widget.Clickable
					if hasStarted {
						click = &gathering.Clickable
					} else {
						click = &widget.Clickable{}
					}

					//gtx.Constraints.Max.Y = 75
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					b := material.ButtonLayoutStyle{
						Background:   component.WithAlpha(th.Fg, 10),
						Button:       click,
						CornerRadius: 8,
					}

					return layout.Inset{
						Bottom: 8,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return b.Layout(gtx, func(gtx C) D {
							return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return material.Label(th, th.TextSize, gatheringName).Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										var text string
										if hasStarted {
											text = fmt.Sprintf("Ends in %s", time.Until(end).Truncate(time.Second))
										} else {
											text = fmt.Sprintf("Starts in %s", time.Until(start).Truncate(time.Second))
										}
										return material.Label(th, th.TextSize, text).Layout(gtx)
									}),
								)
							})
						})
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &g.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})
}
