package worlds

import (
	"fmt"
	"image"
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
	//Map3       *Map3
	worlds     []*messages.SavedWorld
	worldsList widget.List
	l          sync.Mutex

	State      messages.UIState
	chunkCount int
	voidGen    bool
	worldName  string
	back       widget.Clickable
}

func New() pages.Page {
	return &Page{
		worldMap: &Map2{
			images:   make(map[image.Point]*image.RGBA),
			imageOps: make(map[image.Point]paint.ImageOp),
		},
		//Map3: NewMap3(),
		worldsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
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

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.back.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: "ui",
			Target: "ui",
			Data:   messages.ExitSubcommand{},
		})
	}

	switch p.State {
	case messages.UIStateMain:
		return p.worldMap.Layout(gtx)
	case messages.UIStateFinished:
		return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceBetween,
			}.Layout(gtx,
				layout.Flexed(0.9, func(gtx C) D {
					return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
						return layout.Flex{
							Axis: layout.Vertical,
						}.Layout(gtx,
							layout.Rigid(material.Label(th, 20, "Worlds Saved").Layout),
							layout.Flexed(1, func(gtx C) D {
								p.l.Lock()
								defer p.l.Unlock()
								return material.List(th, &p.worldsList).Layout(gtx, len(p.worlds), func(gtx C, index int) D {
									entry := p.worlds[len(p.worlds)-index-1]
									return displayWorldEntry(gtx, th, entry)
								})
							}),
						)
					})
				}),
				layout.Flexed(0.1, func(gtx C) D {
					gtx.Constraints.Max.Y = gtx.Dp(40)
					gtx.Constraints.Max.X = gtx.Constraints.Max.X / 6
					return material.Button(th, &p.back, "Return").Layout(gtx)
				}),
			)
		})
	default:
		return D{}
	}
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
	case messages.SavingWorld:
		u.l.Lock()
		u.worlds = append(u.worlds, m.World)
		u.l.Unlock()
	}
	return nil
}
