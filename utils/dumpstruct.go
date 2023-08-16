package utils

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
)

func DumpStruct(f io.StringWriter, inputStruct any) {
	dumpStruct(f, 0, inputStruct, true, false)
}

func dumpStruct(f io.StringWriter, level int, inputStruct any, withType bool, isInList bool) {
	tBase := strings.Repeat("\t", level)

	if inputStruct == nil {
		f.WriteString("nil")
		return
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
			f.WriteString(typeName + " Not Set")
		} else {
			f.WriteString(typeName + "{\n" + tBase + "\t")
			dumpStruct(f, level+1, val.Interface(), false, false)
			f.WriteString("\n" + tBase + "}")
		}
		return
	}

	switch ii.Kind() {
	case reflect.Struct:
		if ii.NumField() == 0 {
			f.WriteString(typeName + "{}")
		} else {
			f.WriteString(typeName + "{\n")
			for i := 0; i < ii.NumField(); i++ {
				fieldType := ii.Type().Field(i)

				if fieldType.IsExported() {
					f.WriteString(fmt.Sprintf("%s\t%s: ", tBase, fieldType.Name))
					dumpStruct(f, level+1, ii.Field(i).Interface(), true, false)
					f.WriteString(",\n")
				} else {
					f.WriteString(tBase + " " + fieldType.Name + " (unexported)")
				}
			}
			f.WriteString(tBase + "}")
		}
	case reflect.Slice:
		f.WriteString(typeName + "{")

		if ii.Len() > 1000 {
			f.WriteString("<slice too long>")
		} else if ii.Len() == 0 {
		} else {
			e := ii.Index(0)
			t := reflect.TypeOf(e.Interface())
			is_elem_struct := t.Kind() == reflect.Struct || t.Kind() == reflect.Map

			if is_elem_struct {
				f.WriteString("\n")
			}
			for i := 0; i < ii.Len(); i++ {
				if is_elem_struct {
					f.WriteString(tBase + "\t")
				}
				dumpStruct(f, level+1, ii.Index(i).Interface(), false, true)
				if is_elem_struct {
					f.WriteString(",\n")
				} else {
					if i != ii.Len()-1 {
						f.WriteString(", ")
					} else {
						f.WriteString(",")
					}
				}
			}
			if is_elem_struct {
				f.WriteString(tBase)
			}
		}
		f.WriteString("}")
	case reflect.Map:
		it := reflect.TypeOf(inputStruct)
		valType := it.Elem().String()
		if valType == "interface {}" {
			valType = "any"
		}
		keyType := it.Key().String()

		f.WriteString(fmt.Sprintf("map[%s]%s{", keyType, valType))
		if ii.Len() > 0 {
			f.WriteString("\n")
		}

		iter := ii.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			f.WriteString(fmt.Sprintf("%s\t%#v: ", tBase, k.Interface()))
			dumpStruct(f, level+1, v.Interface(), true, false)
			f.WriteString(",\n")
		}

		if ii.Len() > 0 {
			f.WriteString(tBase)
		}
		f.WriteString("}")
	default:
		is_array := ii.Kind() == reflect.Array
		add_type := !isInList && !is_array && len(typeString) > 0
		if add_type {
			f.WriteString(typeString + "(")
		}
		f.WriteString(fmt.Sprintf("%#v", ii.Interface()))
		if add_type {
			f.WriteString(")")
		}
	}
}
