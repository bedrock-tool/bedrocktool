package settings

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
)

// choiceInput is a setting with a few named values, shown as a row of
// buttons with the chosen one filled in.
type choiceInput struct {
	choices []string
	enum    widget.Enum
}

func newChoiceInput(choices []string, value string) *choiceInput {
	c := &choiceInput{choices: choices}
	c.enum.Value = value
	if c.enum.Value == "" && len(choices) > 0 {
		c.enum.Value = choices[0]
	}
	return c
}

func (c *choiceInput) Value() string {
	return c.enum.Value
}

// SetValue picks value if it is one of the choices.
func (c *choiceInput) SetValue(value string) bool {
	for _, name := range c.choices {
		if name == value {
			c.enum.Value = value
			return true
		}
	}
	return false
}

func (c *choiceInput) Layout(gtx C, th *material.Theme) D {
	c.enum.Update(gtx)
	radius := gtx.Dp(popups.Radius)
	hovered, _ := c.enum.Hovered()
	var children []layout.FlexChild
	for i, name := range c.choices {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return c.enum.Layout(gtx, name, func(gtx C) D {
				selected := c.enum.Value == name
				m := op.Record(gtx.Ops)
				d := layout.Inset{Top: 6, Bottom: 6, Left: 16, Right: 16}.Layout(gtx, func(gtx C) D {
					l := material.Body1(th, name)
					if selected {
						l.Color = th.ContrastFg
						l.Font.Weight = font.SemiBold
					}
					return l.Layout(gtx)
				})
				call := m.Stop()
				bg := surface(th, 14)
				switch {
				case selected:
					bg = th.ContrastBg
				case hovered == name:
					bg = surface(th, 30)
				}
				rr := clip.UniformRRect(image.Rectangle{Max: d.Size}, radius).Push(gtx.Ops)
				paint.Fill(gtx.Ops, bg)
				rr.Pop()
				call.Add(gtx.Ops)
				return d
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}
