package worlds

import (
	"fmt"
	"image"
	"slices"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "worlds"

type Page struct {
	worldMap *Map2

	l                    sync.Mutex
	finishedWorlds       []*messages.SavedWorld
	finishedWorldsList   widget.List
	processingWorlds     []*processingWorld
	processingWorldsList widget.List

	State      messages.UIState
	chunkCount int
	voidGen    bool
	worldName  string
	back       widget.Clickable
}

func New(invalidate func()) pages.Page {
	return &Page{
		worldMap: &Map2{
			images:   make(map[image.Point]*image.RGBA),
			imageOps: make(map[image.Point]paint.ImageOp),
		},
		finishedWorldsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		processingWorldsList: widget.List{
			List: layout.List{
				Axis:        layout.Vertical,
				ScrollToEnd: true,
			},
		},
	}
}

type processingWorld struct {
	Name  string
	State string
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return ID
}

func (p *Page) Actions(th *material.Theme) []component.AppBarAction {
	return []component.AppBarAction{
		//pages.AppBarSwitch(&p.worldMap.mapInput.FollowPlayer, "Follow Player", th),
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "World Downloader",
		//Icon: icon.OtherIcon,
	}
}

func displayWorldEntry(gtx C, th *material.Theme, entry *messages.SavedWorld) D {
	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.ContrastFg, 8),
			},
		}).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
				layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d Chunks", entry.Chunks)).Layout),
				layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d Entities", entry.Entities)).Layout),
			)
		})
	})
}

func displayWorldProcessing(gtx C, th *material.Theme, entry *processingWorld) D {
	return layout.Inset{Top: 0, Bottom: 8, Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.ContrastFg, 8),
			},
		}).Layout(gtx, func(gtx C) D {
			return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Label(th, th.TextSize, "Saving World").Layout),
					layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
					layout.Rigid(material.Label(th, th.TextSize, entry.State).Layout),
				)
			})
		})
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.back.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: "ui",
			Target: "ui",
			Data:   messages.ExitSubcommand{},
		})
	}

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			switch p.State {
			case messages.UIStateMain:
				return p.worldMap.Layout(gtx)
			case messages.UIStateFinished:
				return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
					return layout.Flex{
						Axis:    layout.Vertical,
						Spacing: layout.SpaceBetween,
					}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
								return layout.Flex{
									Axis: layout.Vertical,
								}.Layout(gtx,
									layout.Rigid(material.Label(th, 20, "Worlds Saved").Layout),
									layout.Flexed(1, func(gtx C) D {
										p.l.Lock()
										defer p.l.Unlock()
										return material.List(th, &p.finishedWorldsList).
											Layout(gtx, len(p.finishedWorlds), func(gtx C, index int) D {
												entry := p.finishedWorlds[len(p.finishedWorlds)-index-1]
												return displayWorldEntry(gtx, th, entry)
											})
									}),
								)
							})
						}),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.Y = gtx.Dp(40)
							gtx.Constraints.Max.X = gtx.Constraints.Max.X / 6
							return material.Button(th, &p.back, "Return").Layout(gtx)
						}),
					)
				})
			default:
				return D{}
			}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.processingWorldsList).
				Layout(gtx, len(p.processingWorlds), func(gtx C, index int) D {
					entry := p.processingWorlds[index]
					return displayWorldProcessing(gtx, th, entry)
				})
		}),
	)
}

func (u *Page) HandleMessage(msg *messages.Message) *messages.Message {
	switch m := msg.Data.(type) {
	case messages.HaveFinishScreen:
		return &messages.Message{
			Source: "worlds",
			Data:   true,
		}
	case messages.UIState:
		u.State = m
	case messages.UpdateMap:
		u.chunkCount = m.ChunkCount
		u.worldMap.Update(&m)
		//u.Map3.Update(&m)
	case messages.PlayerPosition:
		u.worldMap.mapInput.playerPosition = m.Position
	case messages.MapLookup:
		//u.Map3.SetLookupTexture(m.Lookup)
	case messages.SetValue:
		switch m.Name {
		case "voidGen":
			switch m.Value {
			case "true":
				u.voidGen = true
			case "false":
				u.voidGen = true
			}
		case "worldName":
			u.worldName = m.Value
		}
	case messages.FinishedSavingWorld:
		u.l.Lock()
		u.finishedWorlds = append(u.finishedWorlds, m.World)
		u.processingWorlds = slices.DeleteFunc(u.processingWorlds, func(p *processingWorld) bool {
			return p.Name == m.World.Name
		})
		u.l.Unlock()
	case messages.ProcessingWorldUpdate:
		u.l.Lock()
		exists := false
		for _, w := range u.processingWorlds {
			if w.Name == m.Name {
				w.State = m.State
				exists = true
				break
			}
		}
		if !exists {
			u.processingWorlds = append(u.processingWorlds, &processingWorld{
				Name:  m.Name,
				State: m.State,
			})
		}
		u.l.Unlock()
	}
	return nil
}
