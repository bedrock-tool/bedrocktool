package scripting

import (
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/dop251/goja"
	"github.com/gregwebs/go-recovery"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"

	_ "embed"
)

//go:embed enums.js
var enums_js string

type VM struct {
	vm  *goja.Runtime
	log *logrus.Entry
	CB  struct {
		OnEntityAdd        func(entity any, metadata *goja.Object) (ignore bool)
		OnChunkAdd         func(pos world.ChunkPos) (ignore bool)
		OnEntityDataUpdate func(entity any, metadata *goja.Object)
	}
}

func New() *VM {
	v := &VM{
		vm:  goja.New(),
		log: logrus.WithField("part", "jsvm"),
	}
	console := v.vm.NewObject()
	console.Set("log", func(val goja.Value) {
		if val.SameAs(goja.Undefined()) {
			v.log.Println("undefined")
			return
		}

		if val.ExportType().Kind() == reflect.String {
			v.log.Println(val.String())
			return
		}
		obj := val.ToObject(v.vm)
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			panic(err)
		}
		v.log.Println(string(data))
	})

	v.vm.GlobalObject().Set("console", console)

	_, err := v.vm.RunString(enums_js)
	if err != nil {
		panic(err)
	}

	return v
}

func (v *VM) tryResolveCB(name string, fun any) {
	val := v.vm.Get(name)
	if val == nil {
		return
	}
	err := v.vm.ExportTo(val, fun)
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) Load(script string) error {
	_, err := v.vm.RunScript("script.js", script)
	if err != nil {
		return err
	}

	v.tryResolveCB("OnEntityAdd", &v.CB.OnEntityAdd)
	v.tryResolveCB("OnChunkAdd", &v.CB.OnChunkAdd)
	v.tryResolveCB("OnEntityDataUpdate", &v.CB.OnEntityDataUpdate)
	return nil
}

type entityDataObject struct {
	d protocol.EntityMetadata
	r *goja.Runtime
}

func (m entityDataObject) Get(key string) goja.Value {
	if key == "Flag" {
		f := m.d.Flag
		return m.r.ToValue(&f)
	}
	if key == "SetFlag" {
		f := m.d.SetFlag
		return m.r.ToValue(&f)
	}

	k, err := strconv.Atoi(key)
	if err != nil {
		return nil
	}
	d, ok := m.d[uint32(k)]
	if !ok {
		return nil
	}
	return m.r.ToValue(d)
}

func (m entityDataObject) Set(key string, val goja.Value) bool {
	k, err := strconv.Atoi(key)
	if err != nil {
		return false
	}
	m.d[uint32(k)] = val.Export()
	return true
}

func (m entityDataObject) Has(key string) bool {
	k, err := strconv.Atoi(key)
	if err != nil {
		return false
	}
	_, ok := m.d[uint32(k)]
	return ok
}

func (m entityDataObject) Delete(key string) bool {
	k, err := strconv.Atoi(key)
	if err != nil {
		return false
	}
	delete(m.d, uint32(k))
	return true
}

func (m entityDataObject) Keys() (keys []string) {
	for k := range m.d {
		keys = append(keys, strconv.Itoa(int(k)))
	}
	return
}

func (v *VM) OnEntityAdd(entity any, metadata protocol.EntityMetadata) (ignoreEntity bool) {
	if v.CB.OnEntityAdd == nil {
		return false
	}
	data := v.vm.NewDynamicObject(entityDataObject{metadata, v.vm})
	err := recovery.Call(func() error {
		ignoreEntity = v.CB.OnEntityAdd(entity, data)
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
	return
}

func (v *VM) OnEntityDataUpdate(entity any, metadata protocol.EntityMetadata) {
	if v.CB.OnEntityDataUpdate == nil {
		return
	}
	data := v.vm.NewDynamicObject(entityDataObject{metadata, v.vm})
	err := recovery.Call(func() error {
		v.CB.OnEntityDataUpdate(entity, data)
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnChunkAdd(pos world.ChunkPos) (ignoreChunk bool) {
	if v.CB.OnChunkAdd == nil {
		return false
	}
	err := recovery.Call(func() error {
		ignoreChunk = v.CB.OnChunkAdd(pos)
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
	return
}
