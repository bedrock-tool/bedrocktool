package utils

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

func DumpStruct(f io.StringWriter, inputStruct any) {
	dumpValue(f, 0, reflect.ValueOf(inputStruct), true)
}

func dumpValue(f io.StringWriter, level int, value reflect.Value, withType bool) {
	tabs := strings.Repeat("\t", level)

	typeName := value.Type().String()
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			f.WriteString("nil")
			return
		}
		value = value.Elem()
	}
	if stringer := value.MethodByName("String"); stringer.IsValid() {
		v := stringer.Call(nil)
		value = v[0]
	}

	valueType := value.Type()

	if strings.HasPrefix(typeName, "protocol.Optional") {
		v := value.MethodByName("Value").Call(nil)
		val, set := v[0], v[1]
		if !set.Bool() {
			f.WriteString(typeName + " Not Set")
		} else {
			f.WriteString(typeName + "{\n" + tabs + "\t")
			dumpValue(f, level+1, val, false)
			f.WriteString("\n" + tabs + "}")
		}
		return
	}

	switch valueType.Kind() {
	case reflect.Struct:
		f.WriteString(typeName + "{")
		if valueType.NumField() == 0 {
			f.WriteString("}")
			return
		}
		f.WriteString("\n")
		for i := 0; i < valueType.NumField(); i++ {
			fieldType := valueType.Field(i)
			if fieldType.IsExported() {
				f.WriteString(tabs + "\t" + fieldType.Name + ": ")
				dumpValue(f, level+1, value.Field(i), true)
				f.WriteString(",\n")
			} else {
				f.WriteString(tabs + "\t" + fieldType.Name + " (unexported)\n")
			}
		}
		f.WriteString(tabs + "}")

	case reflect.Map:
		mapValueType := valueType.Elem().String()
		isAny := false
		if mapValueType == "interface {}" {
			mapValueType = "any"
			isAny = true
		}
		mapKeyType := valueType.Key().String()
		f.WriteString("map[" + mapKeyType + "]" + mapValueType + "{")
		if value.Len() == 0 {
			f.WriteString("}")
			return
		}
		f.WriteString("\n")
		iter := value.MapRange()
		for iter.Next() {
			f.WriteString(tabs + "\t")
			dumpValue(f, level+1, iter.Key(), false)
			f.WriteString(": ")
			elem := iter.Value()
			if isAny {
				elem = elem.Elem()
			}
			dumpValue(f, level+1, elem, isAny)
			f.WriteString(",\n")
		}
		f.WriteString(tabs + "}")

	case reflect.Slice, reflect.Array:
		elemType := valueType.Elem()
		elemTypeString := elemType.String()
		if elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}

		isAny := false
		if elemType.Kind() == reflect.Interface {
			elemTypeString = "any"
			isAny = true
		}

		f.WriteString("[]" + elemTypeString + "{")
		if value.Len() == 0 {
			f.WriteString("}")
			return
		}
		if value.Len() > 1000 {
			f.WriteString("<slice to long>}")
			return
		}
		isStructish := false
		switch elemType.Kind() {
		case reflect.Struct, reflect.Map, reflect.Slice:
			f.WriteString("\n")
			isStructish = true
		}
		for i := 0; i < value.Len(); i++ {
			if isStructish {
				f.WriteString(tabs + "\t")
			}
			elem := value.Index(i)
			if isAny {
				elem = elem.Elem()
			}
			dumpValue(f, level+1, elem, isAny)
			if isStructish {
				f.WriteString(",\n")
			} else if i == value.Len()-1 {
				f.WriteString("}")
			} else {
				f.WriteString(", ")
			}
		}
		if isStructish {
			f.WriteString(tabs + "}")
		}

	case reflect.String:
		f.WriteString("\"" + value.String() + "\"")

	case reflect.Bool:
		if value.Bool() {
			f.WriteString("true")
		} else {
			f.WriteString("false")
		}

	default:
		if withType {
			f.WriteString(typeName + "(")
		}
		switch valueType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.WriteString(strconv.FormatInt(value.Int(), 10))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.WriteString(strconv.FormatInt(int64(value.Uint()), 10))
		case reflect.Uintptr:
			f.WriteString("0x" + strconv.FormatInt(int64(value.Uint()), 16))
		case reflect.Float32:
			f.WriteString(strconv.FormatFloat(float64(value.Float()), 'g', 9, 32))
		case reflect.Float64:
			f.WriteString(strconv.FormatFloat(float64(value.Float()), 'g', 9, 64))
		default:
			f.WriteString(fmt.Sprintf("%#+v", value.Interface()))
		}
		if withType {
			f.WriteString(")")
		}
	}
}
