package scripting

import (
	"encoding/json"
	"reflect"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

type Callbacks struct {
	OnEntityAdd func(entity any) (ignore bool)
	OnChunkAdd  func(pos world.ChunkPos) (ignore bool)
}

type VM struct {
	vm *goja.Runtime
	CB Callbacks
}

func New() *VM {
	v := &VM{
		vm: goja.New(),
	}
	console := v.vm.NewObject()
	console.Set("log", func(val goja.Value) {
		if val.ExportType().Kind() == reflect.String {
			println(val.String())
			return
		}
		obj := val.ToObject(v.vm)
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			panic(err)
		}
		println(string(data))
	})

	v.vm.GlobalObject().Set("console", console)

	return v
}

func (v *VM) tryResolveCB(name string, fun any) {
	val := v.vm.Get(name)
	if val == nil {
		return
	}
	err := v.vm.ExportTo(val, fun)
	if err != nil {
		logrus.Error(err)
	}
}

func (v *VM) Load(script string) error {
	_, err := v.vm.RunScript("script.js", script)
	if err != nil {
		return err
	}

	v.tryResolveCB("OnEntityAdd", &v.CB.OnEntityAdd)
	v.tryResolveCB("OnChunkAdd", &v.CB.OnChunkAdd)
	return nil
}
