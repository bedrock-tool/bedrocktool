package utils

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
)

func DumpStruct(level int, inputStruct any, withType bool, isInList bool) (s string) {
	tBase := strings.Repeat("\t", level)

	if inputStruct == nil {
		return "nil"
	}

	ii := reflect.Indirect(reflect.ValueOf(inputStruct))
	typeName := reflect.TypeOf(inputStruct).String()
	if typeName == "[]interface {}" {
		typeName = "[]any"
	}
	typeString := ""
	if withType {
		if slices.Contains([]string{"bool", "string"}, typeName) {
		} else {
			typeString = typeName
		}
	}

	if strings.HasPrefix(typeName, "protocol.Optional") {
		v := ii.MethodByName("Value").Call(nil)
		val, set := v[0], v[1]
		if !set.Bool() {
			s += typeName + " Not Set"
		} else {
			s += typeName + "{\n" + tBase + "\t"
			s += DumpStruct(level+1, val.Interface(), false, false)
			s += "\n" + tBase + "}"
		}
		return
	}

	switch ii.Kind() {
	case reflect.Struct:
		if ii.NumField() == 0 {
			s += typeName + "{}"
		} else {
			s += typeName + "{\n"
			for i := 0; i < ii.NumField(); i++ {
				fieldType := ii.Type().Field(i)

				if fieldType.IsExported() {
					s += fmt.Sprintf("%s\t%s: %s,\n", tBase, fieldType.Name, DumpStruct(level+1, ii.Field(i).Interface(), true, false))
				} else {
					s += tBase + " " + fieldType.Name + " (unexported)"
				}
			}
			s += tBase + "}"
		}
	case reflect.Slice:
		s += typeName + "{"

		if ii.Len() > 1000 {
			s += "<slice too long>"
		} else if ii.Len() == 0 {
		} else {
			e := ii.Index(0)
			t := reflect.TypeOf(e.Interface())
			is_elem_struct := t.Kind() == reflect.Struct

			if is_elem_struct {
				s += "\n"
			}
			for i := 0; i < ii.Len(); i++ {
				if is_elem_struct {
					s += tBase + "\t"
				}
				s += DumpStruct(level+1, ii.Index(i).Interface(), false, true) + ","
				if is_elem_struct {
					s += "\n"
				} else {
					if i != ii.Len()-1 {
						s += " "
					}
				}
			}
			if is_elem_struct {
				s += tBase
			}
		}
		s += "}"
	case reflect.Map:
		it := reflect.TypeOf(inputStruct)
		valType := it.Elem().String()
		if valType == "interface {}" {
			valType = "any"
		}
		keyType := it.Key().String()

		s += fmt.Sprintf("map[%s]%s{", keyType, valType)
		if ii.Len() > 0 {
			s += "\n"
		}

		iter := ii.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			s += fmt.Sprintf("%s\t%#v: %s,\n", tBase, k.Interface(), DumpStruct(level+1, v.Interface(), true, false))
		}

		if ii.Len() > 0 {
			s += tBase
		}
		s += "}"
	default:
		is_array := ii.Kind() == reflect.Array
		add_type := !isInList && !is_array && len(typeString) > 0
		if add_type {
			s += typeString + "("
		}
		s += fmt.Sprintf("%#v", ii.Interface())
		if add_type {
			s += ")"
		}
	}
	return s
}
