package scripting

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/dop251/goja"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/gregwebs/go-recovery"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"

	_ "embed"
)

//go:embed enums.js
var enums_js string

type VM struct {
	vm  *goja.Runtime
	l   sync.Mutex
	log *logrus.Entry
	CB  struct {
		OnEntityAdd        func(entity any, metadata entityDataObject, properties map[string]*entity.EntityProperty, timeReceived float64) (apply bool)
		OnChunkAdd         func(pos world.ChunkPos, timeReceived float64) (apply bool)
		OnEntityDataUpdate func(entity any, metadata entityDataObject, properties map[string]*entity.EntityProperty, timeReceived float64)
		OnBlockUpdate      func(name string, properties map[string]any, pos protocol.BlockPos, timeReceived float64) (apply bool)
		OnSpawnParticle    func(name string, pos mgl32.Vec3, timeReceived float64)
		OnPacket           func(name string, pk packet.Packet, toServer bool, timeReceived float64) (drop bool)
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

	events := v.vm.NewObject()
	events.Set("register", func(name string, callback goja.Value) (err error) {
		switch name {
		case "EntityAdd":
			err = v.vm.ExportTo(callback, &v.CB.OnEntityAdd)
		case "EntityDataUpdate":
			err = v.vm.ExportTo(callback, &v.CB.OnEntityDataUpdate)
		case "ChunkAdd":
			err = v.vm.ExportTo(callback, &v.CB.OnChunkAdd)
		case "BlockUpdate":
			err = v.vm.ExportTo(callback, &v.CB.OnBlockUpdate)
		case "SpawnParticle":
			err = v.vm.ExportTo(callback, &v.CB.OnSpawnParticle)
		case "Packet":
			err = v.vm.ExportTo(callback, &v.CB.OnPacket)
		}
		return err
	})
	v.vm.GlobalObject().Set("events", events)

	_, err := v.vm.RunString(enums_js)
	if err != nil {
		panic(err)
	}

	return v
}

func (v *VM) Load(script string) error {
	_, err := v.vm.RunScript("script.js", script)
	if err != nil {
		return err
	}
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

func (v *VM) OnEntityAdd(entity any, metadata protocol.EntityMetadata, properties map[string]*entity.EntityProperty, timeReceived time.Time) (apply bool) {
	if v.CB.OnEntityAdd == nil {
		return true
	}
	err := recovery.Call(func() error {
		apply = v.CB.OnEntityAdd(entity, entityDataObject{metadata, v.vm}, properties, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
	return
}

func (v *VM) OnEntityDataUpdate(ent *entity.Entity, timeReceived time.Time) {
	if v.CB.OnEntityDataUpdate == nil {
		return
	}
	err := recovery.Call(func() error {
		v.CB.OnEntityDataUpdate(ent, entityDataObject{ent.Metadata, v.vm}, ent.Properties, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnChunkAdd(pos world.ChunkPos, timeReceived time.Time) (apply bool) {
	if v.CB.OnChunkAdd == nil {
		return true
	}
	err := recovery.Call(func() error {
		apply = v.CB.OnChunkAdd(pos, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
		apply = true
	}
	return
}

func (v *VM) OnBlockUpdate(name string, properties map[string]any, pos protocol.BlockPos, timeReceived time.Time) (apply bool) {
	if v.CB.OnBlockUpdate == nil {
		return true
	}

	err := recovery.Call(func() error {
		apply = v.CB.OnBlockUpdate(name, properties, pos, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
		return true
	}

	return apply
}

func (v *VM) OnSpawnParticle(name string, position mgl32.Vec3, timeReceived time.Time) {
	if v.CB.OnSpawnParticle == nil {
		return
	}

	err := recovery.Call(func() error {
		v.CB.OnSpawnParticle(name, position, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnPacket(pk packet.Packet, toServer bool, timeReceived time.Time) (drop bool) {
	if v.CB.OnPacket == nil {
		return false
	}

	v.l.Lock()
	defer v.l.Unlock()
	err := recovery.Call(func() error {
		packetName := strings.Split(reflect.TypeOf(pk).String(), ".")[1]
		drop = v.CB.OnPacket(packetName, pk, toServer, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
		return false
	}
	return drop
}
