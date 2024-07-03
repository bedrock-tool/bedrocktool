package settings

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type settingsPage struct {
	cmd     commands.Command
	f       *flag.FlagSet
	widgets map[string]any

	grid outlay.Grid
	list widget.List

	tooltips          map[string]*component.TipArea
	flagsNamesOrdered []string
	flags             map[string]*flag.Flag
}

func (s *settingsPage) Init() {
	s.list.Axis = layout.Vertical
	s.widgets = make(map[string]any)
	s.tooltips = make(map[string]*component.TipArea)
	s.flags = make(map[string]*flag.Flag)
	s.f = flag.NewFlagSet("", flag.ContinueOnError)
	s.cmd.SetFlags(s.f)
	hasAddress := false

	visitFunc := func(f *flag.Flag) {
		if f.Name == "v" {
			f.Name = "verbose"
		}

		t := reflect.ValueOf(f.Value).Type().String()
		t = strings.Split(t, ".")[1]
		switch t {
		case "stringValue", "intValue":
			e := &component.TextField{
				Helper: f.Name + ": " + f.Usage,
			}
			/*if f.DefValue != "" && f.DefValue != "0" {
				e.Helper = fmt.Sprintf("%s (Default: '%s')", f.Usage, f.DefValue)
			}*/

			e.SetText(f.DefValue)
			e.SingleLine = true
			if t == "intValue" {
				e.Filter = "0123456789"
				e.InputHint = key.HintNumeric
			}
			s.widgets[f.Name] = e
		case "boolValue":
			e := &widget.Bool{}
			if f.DefValue == "true" {
				e.Value = true
			}
			s.widgets[f.Name] = e
		default:
			logrus.Warnf("%s unknown flag type", t)
		}

		s.tooltips[f.Name] = &component.TipArea{}
		s.flags[f.Name] = f

		if f.Name == "address" {
			s.widgets[f.Name] = AddressInput
			hasAddress = true
		} else {
			s.flagsNamesOrdered = append(s.flagsNamesOrdered, f.Name)
		}
	}

	s.f.VisitAll(visitFunc)

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if f.Name == "debug" || f.Name == "capture" {
			visitFunc(f)
		}
	})

	if hasAddress {
		s.flagsNamesOrdered = append([]string{"address"}, s.flagsNamesOrdered...)
	}
}

func (s *settingsPage) Valid() bool {
	addr, ok := s.widgets["address"].(*addressInput)
	if !ok {
		return true
	}
	if addr.Value() == "" {
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

func (s *settingsPage) Apply() error {
	for flagName, flag := range s.flags {
		flagWidget := s.widgets[flagName]
		var value string
		switch flagWidget := flagWidget.(type) {
		case *widget.Bool:
			value = "false"
			if flagWidget.Value {
				value = "true"
			}
		case *component.TextField:
			value = flagWidget.Text()
		case *addressInput:
			value = flagWidget.Value()
		default:
			return errors.New("unknown widget " + flagName)
		}

		vt := strings.Split(reflect.ValueOf(flag).Type().String(), ".")[1]

		if value == "" && vt == "intValue" {
			value = flag.DefValue
		}
		err := flag.Value.Set(value)
		if err != nil {
			return settingError{name: flagName, err: err}
		}
	}
	return nil
}

func (s *settingsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return material.List(th, &s.list).Layout(gtx, len(s.flagsNamesOrdered)+2, func(gtx layout.Context, index int) layout.Dimensions {
		if index == 0 { // address input
			w, ok := s.widgets["address"].(*addressInput)
			if ok {
				return w.Layout(gtx, th)
			}
			return layout.Dimensions{}
		}
		if index == 1 { // bool flags
			var widgets []layout.Widget
			for _, name := range s.flagsNamesOrdered {
				name := name
				if w, ok := s.widgets[name].(*widget.Bool); ok {
					widgets = append(widgets, func(gtx C) D {
						return s.tooltips[name].Layout(gtx,
							component.PlatformTooltip(th, s.flags[name].Usage),
							material.CheckBox(th, w, name).Layout,
						)
					})
				}
			}

			rows, cols := max(1, len(widgets)/4), min(len(widgets), 4)
			return s.grid.Layout(gtx, rows, cols, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return gtx.Dp(130)
				case layout.Vertical:
					return gtx.Dp(40)
				}
				return 0
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				idx := col + cols*row
				return widgets[idx](gtx)
			})
		}

		name := s.flagsNamesOrdered[index-2]
		w := s.widgets[name]

		switch w := w.(type) {
		case *widget.Bool:
			return layout.Dimensions{}
		case *component.TextField:
			return w.Layout(gtx, th, w.Helper)
		default:
			return layout.Dimensions{}
		}
	})
}
