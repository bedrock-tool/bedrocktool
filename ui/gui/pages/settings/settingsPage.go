package settings

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"reflect"
	"strings"
	"unsafe"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type settingsArg struct {
	widget any
	name   string
	usage  string
	// advanced is a proxy setting (connection, debugging), shown last
	advanced bool
}

type settingsPage struct {
	g        guim.Guim
	cmd      commands.Command
	settings any
	args     []commands.Arg
	widgets  map[string]*settingsArg

	flagsNamesOrdered []string

	list widget.List
}

func (s *settingsPage) Init() {
	s.list.Axis = layout.Vertical
	s.list.Alignment = layout.Middle
	s.widgets = make(map[string]*settingsArg)
	s.settings = s.cmd.Settings()

	args, err := commands.ParseArgsType(reflect.ValueOf(s.settings), nil, nil)
	if err != nil {
		logrus.Fatal(err)
	}
	s.args = args

	hasAddress := false
	for _, arg := range args {
		extType := strings.Split(arg.ExtType, ",")
		if len(extType) > 0 && extType[0] == "file" {
			var ext string
			if len(extType) > 1 {
				ext = extType[1]
			}
			s.widgets[arg.Flag] = &settingsArg{
				widget: &fileInputWidget{g: s.g, textField: component.TextField{
					Editor: widget.Editor{
						SingleLine: true,
					},
				}, Hint: arg.Name, Ext: ext},
				name:  argName(arg),
				usage: argDesc(arg),
			}
			s.flagsNamesOrdered = append(s.flagsNamesOrdered, arg.Flag)
			continue
		}
		switch arg.Type {
		case "connectInfo":
			s.widgets[arg.Flag] = &settingsArg{
				widget: AddressInput,
				usage:  "target server address",
			}
			hasAddress = true
			continue

		case "bool":
			e := &widget.Bool{}
			e.Value = arg.Default == "true"
			s.widgets[arg.Flag] = &settingsArg{widget: e, name: argName(arg), usage: argDesc(arg)}

		default:
			e := &component.TextField{
				Editor: widget.Editor{
					SingleLine: true,
				},
			}
			e.SetText(arg.Default)
			s.widgets[arg.Flag] = &settingsArg{widget: e, name: argName(arg), usage: argDesc(arg)}
		}
		s.flagsNamesOrdered = append(s.flagsNamesOrdered, arg.Flag)
	}
	for _, arg := range args {
		if w, ok := s.widgets[arg.Flag]; ok && arg.Type != "connectInfo" && len(arg.Path) > 1 && arg.Path[0] == "ProxySettings" {
			w.advanced = true
		}
	}
	if hasAddress {
		s.flagsNamesOrdered = append([]string{"address"}, s.flagsNamesOrdered...)
	}
}

func argName(arg commands.Arg) string {
	if arg.Name != "" {
		return arg.Name
	}
	return arg.Flag
}

func (s *settingsPage) Valid() bool {
	addr, ok := s.widgets["address"].widget.(*addressInput)
	if !ok {
		return true
	}
	if addr.GetConnectInfo() == nil {
		return false
	}
	return true
}

type settingError struct {
	name string
	err  error
}

func (e settingError) Error() string {
	return fmt.Sprintf("%s: %s", e.name, e.err)
}

func (s *settingsPage) Apply() (any, error) {
	for _, arg := range s.args {
		flagWidget := s.widgets[arg.Flag]
		var value string
		switch flagWidget := flagWidget.widget.(type) {
		case *widget.Bool:
			value = "false"
			if flagWidget.Value {
				value = "true"
			}

		case *component.TextField:
			value = flagWidget.Text()

		case *fileInputWidget:
			value = flagWidget.textField.Text()

		case *addressInput:
			arg.SetConnectInfo(flagWidget.GetConnectInfo())
			continue

		default:
			return nil, errors.New("unknown widget " + arg.Name)
		}
		if arg.Type == "int" && value == "" {
			value = arg.Default
			if value == "" {
				value = "0"
			}
		}
		err := arg.Set(value)
		if err != nil {
			return nil, settingError{name: arg.Name, err: err}
		}
	}
	return s.settings, nil
}

func textFieldIsActive(w *component.TextField) bool {
	return *(*uint8)(unsafe.Pointer(reflect.ValueOf(w).Elem().FieldByName("state").UnsafeAddr())) == 3
}

func (s *settingsPage) PopupActive() bool {
	for _, name := range s.flagsNamesOrdered {
		w := s.widgets[name]
		switch w := w.widget.(type) {
		case *widget.Bool:
			continue
		case *component.TextField:
			if textFieldIsActive(w) {
				return true
			}
		case *fileInputWidget:
			if textFieldIsActive(&w.textField) {
				return true
			}
		}
	}
	return false
}

func (s *settingsPage) LayoutPopupInput(gtx C, th *material.Theme) D {
	cornerRadius := 4
	return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
				toastRect := clip.RRect{
					Rect: image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y+cornerRadius),
					SE:   int(cornerRadius),
					SW:   int(cornerRadius),
					NW:   int(cornerRadius),
					NE:   int(cornerRadius),
				}.Push(gtx.Ops)
				paint.ColorOp{Color: th.Bg}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				toastRect.Pop()
				return D{Size: gtx.Constraints.Max}
			}),
			layout.Stacked(func(gtx C) D {
				return layout.Inset{
					Left: unit.Dp(cornerRadius),
				}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max.X -= cornerRadius
					for _, name := range s.flagsNamesOrdered {
						w := s.widgets[name]
						switch w := w.widget.(type) {
						case *widget.Bool:
							continue
						case *component.TextField:
							if textFieldIsActive(w) {
								hint := s.widgets[name].name
								w.Update(gtx, th, hint)
								return w.Layout(gtx, th, hint)
							}
						case *fileInputWidget:
							if textFieldIsActive(&w.textField) {
								return w.Layout(gtx, th)
							}
						}
					}
					return D{}
				})
			}),
		)
	})
}

// muted is th.Fg faded, for secondary text.
func muted(th *material.Theme) color.NRGBA {
	c := th.Fg
	c.A = 170
	return c
}

// surface is a card background a little apart from th.Bg.
func surface(th *material.Theme, alpha uint8) color.NRGBA {
	c := th.Fg
	c.A = alpha
	return c
}

func sectionTitle(gtx C, th *material.Theme, title string) D {
	return layout.Inset{Top: 18, Bottom: 8}.Layout(gtx, func(gtx C) D {
		l := material.Subtitle1(th, title)
		l.Font.Weight = font.Bold
		return l.Layout(gtx)
	})
}

// optionCard is a bool setting: its name, and its description below, which
// wraps instead of being cut off.
func optionCard(gtx C, th *material.Theme, name, desc string, value bool) D {
	return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				icon := th.Icon.CheckBoxUnchecked
				col := muted(th)
				if value {
					icon = th.Icon.CheckBoxChecked
					col = th.ContrastBg
				}
				size := gtx.Dp(22)
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				return icon.Layout(gtx, col)
			}),
			layout.Rigid(layout.Spacer{Width: 10}.Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						l := material.Body1(th, name)
						l.Font.Weight = font.SemiBold
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if desc == "" || desc == name {
							return D{}
						}
						return layout.Inset{Top: 3}.Layout(gtx, func(gtx C) D {
							l := material.Body2(th, desc)
							l.Color = muted(th)
							return l.Layout(gtx)
						})
					}),
				)
			}),
		)
	})
}

// layoutOptions lays bool settings out as cards, two to a row when there's
// room, each row as tall as its tallest card. An option that's on gets a
// tint and an accent bar down its left side.
func (s *settingsPage) layoutOptions(gtx C, th *material.Theme, names []string) D {
	if len(names) == 0 {
		return D{}
	}

	gap := gtx.Dp(8)
	width := gtx.Constraints.Max.X
	cols := 1
	if width >= gtx.Dp(480) {
		cols = 2
	}
	colW := (width - gap*(cols-1)) / cols
	radius := gtx.Dp(popups.Radius)
	bar := gtx.Dp(3)

	type card struct {
		call op.CallOp
		b    *widget.Bool
	}

	y := 0
	for i := 0; i < len(names); i += cols {
		var cards []card
		rowH := 0
		for j := i; j < min(i+cols, len(names)); j++ {
			w := s.widgets[names[j]]
			b := w.widget.(*widget.Bool)
			b.Update(gtx)
			cgtx := gtx
			cgtx.Constraints = layout.Constraints{Min: image.Pt(colW, 0), Max: image.Pt(colW, gtx.Constraints.Max.Y)}
			m := op.Record(gtx.Ops)
			d := optionCard(cgtx, th, w.name, w.usage, b.Value)
			cards = append(cards, card{call: m.Stop(), b: b})
			rowH = max(rowH, d.Size.Y)
		}
		for k, c := range cards {
			off := op.Offset(image.Pt(k*(colW+gap), y)).Push(gtx.Ops)
			rect := image.Rect(0, 0, colW, rowH)
			bg := surface(th, 10)
			if c.b.Value {
				bg = th.ContrastBg
				bg.A = 18
			}
			if c.b.Hovered() {
				bg.A += 12
			}
			rr := clip.UniformRRect(rect, radius).Push(gtx.Ops)
			paint.Fill(gtx.Ops, bg)
			if c.b.Value {
				accent := clip.Rect(image.Rect(0, 0, bar, rowH)).Push(gtx.Ops)
				paint.Fill(gtx.Ops, th.ContrastBg)
				accent.Pop()
			}
			rr.Pop()
			cgtx := gtx
			cgtx.Constraints = layout.Exact(rect.Size())
			c.b.Layout(cgtx, func(gtx C) D {
				c.call.Add(gtx.Ops)
				return D{Size: rect.Size()}
			})
			off.Pop()
		}
		y += rowH + gap
	}
	return D{Size: image.Pt(width, y)}
}

// layoutField is a text or file setting with its name and description
// above it.
func (s *settingsPage) layoutField(gtx C, th *material.Theme, w *settingsArg) D {
	return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				l := material.Body1(th, w.name)
				l.Font.Weight = font.SemiBold
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if w.usage == "" || w.usage == w.name {
					return D{}
				}
				l := material.Body2(th, w.usage)
				l.Color = muted(th)
				return layout.Inset{Top: 2, Bottom: 2}.Layout(gtx, l.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				switch ww := w.widget.(type) {
				case *component.TextField:
					ww.Update(gtx, th, w.name)
					return ww.Layout(gtx, th, w.name)
				case *fileInputWidget:
					return ww.Layout(gtx, th)
				}
				return D{}
			}),
		)
	})
}

func (s *settingsPage) Layout(gtx C, th *material.Theme) D {
	// the command's own settings first; the proxy's connection and
	// debugging ones last
	var bools, advBools []string
	var fields, advFields []*settingsArg
	for _, name := range s.flagsNamesOrdered {
		w := s.widgets[name]
		switch w.widget.(type) {
		case *widget.Bool:
			if w.advanced {
				advBools = append(advBools, name)
			} else {
				bools = append(bools, name)
			}
		case *component.TextField, *fileInputWidget:
			if w.advanced {
				advFields = append(advFields, w)
			} else {
				fields = append(fields, w)
			}
		}
	}

	var items []layout.Widget
	section := func(title string) {
		items = append(items, func(gtx C) D { return sectionTitle(gtx, th, title) })
	}
	if addr, ok := s.widgets["address"]; ok {
		if input, ok := addr.widget.(*addressInput); ok {
			section("Server")
			items = append(items, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return input.Layout(gtx, th)
			})
		}
	}
	if len(bools) > 0 || len(fields) > 0 {
		section("Options")
		items = append(items, func(gtx C) D { return s.layoutOptions(gtx, th, bools) })
		if len(fields) > 0 {
			items = append(items, layout.Spacer{Height: 10}.Layout)
		}
		for _, f := range fields {
			items = append(items, func(gtx C) D { return s.layoutField(gtx, th, f) })
		}
	}
	if len(advBools) > 0 || len(advFields) > 0 {
		section("Connection & debugging")
		items = append(items, func(gtx C) D { return s.layoutOptions(gtx, th, advBools) })
		if len(advFields) > 0 {
			items = append(items, layout.Spacer{Height: 10}.Layout)
		}
		for _, f := range advFields {
			items = append(items, func(gtx C) D { return s.layoutField(gtx, th, f) })
		}
	}

	return layout.N.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(760))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return material.List(th, &s.list).Layout(gtx, len(items), func(gtx C, index int) D {
			return layout.Inset{Right: 12, Left: 2}.Layout(gtx, items[index])
		})
	})
}

// argDesc is an argument's description starting with a capital letter.
func argDesc(arg commands.Arg) string {
	d := strings.TrimSpace(arg.Desc)
	if d == "" {
		return d
	}
	return strings.ToUpper(d[:1]) + d[1:]
}
