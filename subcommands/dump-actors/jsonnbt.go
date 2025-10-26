package dumpactors

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type jsonNbtValue struct {
	Value any
}

// serialized form
type jsonNbtValueInternal struct {
	Uint8   *uint8                          `json:"uint8,omitempty"`
	Int16   *int16                          `json:"int16,omitempty"`
	Int32   *int32                          `json:"int32,omitempty"`
	Int64   *int64                          `json:"int64,omitempty"`
	Float32 *float32                        `json:"float32,omitempty"`
	Float64 *float64                        `json:"float64,omitempty"`
	String  *string                         `json:"string,omitempty"`
	Map     map[string]jsonNbtValueInternal `json:"map,omitempty"`
	Slice   []jsonNbtValueInternal          `json:"slice,omitempty"`
}

func (internal *jsonNbtValueInternal) Value() (any, error) {
	if internal.Uint8 != nil {
		return *internal.Uint8, nil
	} else if internal.Int16 != nil {
		return *internal.Int16, nil
	} else if internal.Int32 != nil {
		return *internal.Int32, nil
	} else if internal.Int64 != nil {
		return *internal.Int64, nil
	} else if internal.Float32 != nil {
		return *internal.Float32, nil
	} else if internal.Float64 != nil {
		return *internal.Float64, nil
	} else if internal.String != nil {
		return *internal.String, nil
	} else if internal.Map != nil {
		m := make(map[string]any)
		for key, val := range internal.Map {
			v, err := val.Value()
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
		return m, nil
	} else if internal.Slice != nil {
		if len(internal.Slice) == 0 {
			return make([]any, 0), nil
		}

		v0, err := internal.Slice[0].Value()
		if err != nil {
			return nil, err
		}

		elemType := reflect.TypeOf(v0)
		sl := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(internal.Slice))
		for _, e := range internal.Slice {
			v, err := e.Value()
			if err != nil {
				return nil, err
			}
			sl = reflect.Append(sl, reflect.ValueOf(v))
		}
		return sl.Interface(), nil
	} else {
		return nil, nil // Or handle default case if needed
	}
}

func (jv *jsonNbtValue) MarshalJSON() ([]byte, error) {
	internal := jsonNbtValueInternal{}

	switch v := jv.Value.(type) {
	case uint8:
		internal.Uint8 = &v
	case int16:
		internal.Int16 = &v
	case int32:
		internal.Int32 = &v
	case int64:
		internal.Int64 = &v
	case float32:
		internal.Float32 = &v
	case float64:
		internal.Float64 = &v
	case string:
		internal.String = &v
	case map[string]any:
		internal.Map = make(map[string]jsonNbtValueInternal)
		for key, val := range v {
			nested := jsonNbtValue{Value: val}
			nestedInternal := jsonNbtValueInternal{}
			nestedBytes, err := nested.MarshalJSON()
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(nestedBytes, &nestedInternal); err != nil {
				return nil, err
			}
			internal.Map[key] = nestedInternal
		}
	case []any:
		internal.Slice = make([]jsonNbtValueInternal, len(v))
		for i, val := range v {
			nested := jsonNbtValue{Value: val}
			nestedInternal := jsonNbtValueInternal{}
			nestedBytes, err := nested.MarshalJSON()
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(nestedBytes, &nestedInternal); err != nil {
				return nil, err
			}
			internal.Slice[i] = nestedInternal
		}
	default:
		return nil, fmt.Errorf("unsupported type for MarshalJSON: %T", jv.Value)
	}

	return json.Marshal(internal)
}

func (jv *jsonNbtValue) UnmarshalJSON(data []byte) error {
	var internal jsonNbtValueInternal
	if err := json.Unmarshal(data, &internal); err != nil {
		return err
	}

	var err error
	jv.Value, err = internal.Value()
	return err
}
