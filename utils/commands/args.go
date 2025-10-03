package commands

import (
	"context"
	"flag"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
)

type StringSliceFlag struct {
	A *[]string
}

func (s *StringSliceFlag) Set(v string) error {
	vals := strings.Split(v, ",")
	for _, val := range vals {
		val = strings.TrimSpace(val)
		*s.A = append(*s.A, val)
	}
	return nil
}

func (s StringSliceFlag) String() string {
	return strings.Join(*s.A, ",")
}

var connectInfoType = reflect.TypeFor[*connectinfo.ConnectInfo]()

type Arg struct {
	Name    string
	Desc    string
	Flag    string
	Default string
	Type    string
	Path    []string
	ExtType string
	field   reflect.Value
}

func (a *Arg) Set(v string) error {
	switch a.field.Kind() {
	case reflect.String:
		a.field.Set(reflect.ValueOf(v))
		return nil
	case reflect.Int:
		vi, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		a.field.Set(reflect.ValueOf(vi))
		return nil
	case reflect.Bool:
		switch v {
		case "true":
			a.field.Set(reflect.ValueOf(true))
		case "false":
			a.field.Set(reflect.ValueOf(false))
		default:
			return fmt.Errorf("invalid bool %s %s", a.Name, v)
		}
		return nil
	case reflect.Slice:
		sp := strings.Split(v, ",")
		if sp[0] == "" {
			sp = sp[:0]
		}
		a.field.Set(reflect.ValueOf(sp))
		return nil
	}
	panic("unimplemented")
}

func (a *Arg) SetConnectInfo(connectInfo *connectinfo.ConnectInfo) {
	if a.Type != "connectInfo" {
		panic("invalid")
	}
	a.field.Set(reflect.ValueOf(connectInfo))
}

func (a *Arg) String() string {
	return a.field.String()
}

func ParseArgsType(settings reflect.Value, path []string, without []string) ([]Arg, error) {
	if settings.Kind() == reflect.Pointer && settings.IsNil() || !settings.IsValid() {
		return nil, nil
	}
	var args []Arg
	settings = reflect.Indirect(settings)
	st := settings.Type()
	for i := range st.NumField() {
		fieldType := st.Field(i)
		field := settings.Field(i)
		flag := fieldType.Tag.Get("flag")
		name := fieldType.Tag.Get("opt")
		desc := fieldType.Tag.Get("desc")
		defaultStr := fieldType.Tag.Get("default")
		extType := fieldType.Tag.Get("type")

		if slices.Contains(without, flag) {
			continue
		}
		if desc == "" {
			desc = name
		}
		if strings.HasPrefix(desc, "locale.") {
			desc = locale.Loc(desc[7:], nil)
		}

		var arg = Arg{
			Name:    name,
			Desc:    desc,
			Flag:    flag,
			Path:    append(path, fieldType.Name),
			Default: defaultStr,
			field:   field,
			ExtType: extType,
		}

		if fieldType.Type == connectInfoType {
			arg.Type = "connectInfo"
			args = append(args, arg)
			continue
		}

		switch fieldType.Type.Kind() {
		case reflect.String:
			arg.Type = "string"
		case reflect.Int:
			arg.Type = "int"
		case reflect.Bool:
			arg.Type = "bool"
		case reflect.Slice:
			arg.Type = "slice"
		case reflect.Struct:
			// nothing
		default:
			panic("invalid field type")
		}

		switch fieldType.Type.Kind() {
		case reflect.Struct:
			without := strings.Split(fieldType.Tag.Get("without"), ",")
			args2, err := ParseArgsType(field, append(path, fieldType.Name), without)
			if err != nil {
				return nil, err
			}
			args = append(args, args2...)
			continue

		case reflect.String, reflect.Bool, reflect.Int:
			args = append(args, arg)
			continue

		case reflect.Slice:
			elemType := fieldType.Type.Elem()
			if elemType.Kind() != reflect.String {
				panic(fmt.Errorf("unhandled slice type"))
			}
			args = append(args, arg)
			continue

		default:
			panic(fmt.Errorf("unhandled field type"))
		}
	}
	return args, nil
}

func ParseArgs(ctx context.Context, cmd Command, argValues []string) (any, *flag.FlagSet, error) {
	settings := cmd.Settings()

	args, err := ParseArgsType(reflect.ValueOf(settings), nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var connectInfo *connectinfo.ConnectInfo
	var consumer *Arg
	flags := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	for _, arg := range args {
		if arg.Type == "connectInfo" {
			connectInfo = &connectinfo.ConnectInfo{}
			flags.StringVar(&connectInfo.Value, arg.Flag, "", arg.Desc)
			arg.SetConnectInfo(connectInfo)
			continue
		}
		if arg.Type == "bool" {
			def := arg.Default == "true"
			flags.BoolVar(arg.field.Addr().Interface().(*bool), arg.Flag, def, arg.Desc)
			continue
		}
		if arg.Flag == "-args" {
			if arg.field.Kind() != reflect.Slice {
				panic("non slice arg consumer")
			}
			consumer = &arg
			continue
		}
		arg.Set(arg.Default)
		flags.Var(&arg, arg.Flag, arg.Desc)
	}
	if err := flags.Parse(argValues); err != nil {
		return nil, nil, err
	}
	if consumer != nil {
		consumer.field.Set(reflect.ValueOf(flags.Args()))
	}

	if connectInfo != nil {
		if connectInfo.Value == "" {
			var cancelled bool
			addressInput, cancelled := utils.UserInput(ctx, locale.Loc("enter_server", nil), nil)
			if cancelled {
				return nil, nil, context.Canceled
			}
			connectInfo.Value = addressInput
		}
	}

	return settings, flags, nil
}
