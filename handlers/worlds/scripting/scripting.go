package scripting

import (
	"fmt"
	"os"
	"sync"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"

	_ "embed"
)

type VM struct {
	runtime *goja.Runtime
	lock    sync.Mutex
	log     *logrus.Entry

	CB struct {
		OnEntityAdd        func(entity *entity.Entity, metadata *goja.Object, timeReceived float64) (apply goja.Value)
		OnChunkData        func(pos world.ChunkPos)
		OnChunkAdd         func(pos world.ChunkPos, timeReceived float64) (apply goja.Value)
		OnEntityDataUpdate func(entity *entity.Entity, metadata *goja.Object, timeReceived float64)
		OnBlockUpdate      func(name string, properties map[string]any, pos protocol.BlockPos, timeReceived float64) (apply goja.Value)
		OnSpawnParticle    func(name string, pos mgl32.Vec3, timeReceived float64)
		OnPacket           func(name string, pk packet.Packet, toServer bool, timeReceived float64) (apply goja.Value)
	}

	GetWorld           func() *worldstate.World
	DisplayChatMessage func(msg string)
	SetIngameMap       func(enabled bool)
}

func New() *VM {
	v := &VM{
		runtime: goja.New(),
		log:     logrus.WithField("part", "jsvm"),
	}

	registry := new(require.Registry)
	registry.Enable(v.runtime)
	console.Enable(v.runtime)

	events := v.runtime.NewObject()
	events.Set("register", func(name string, callback goja.Value) (err error) {
		switch name {
		case "EntityAdd":
			err = v.runtime.ExportTo(callback, &v.CB.OnEntityAdd)
		case "EntityDataUpdate":
			err = v.runtime.ExportTo(callback, &v.CB.OnEntityDataUpdate)
		case "ChunkAdd":
			err = v.runtime.ExportTo(callback, &v.CB.OnChunkAdd)
		case "ChunkData":
			err = v.runtime.ExportTo(callback, &v.CB.OnChunkData)
		case "BlockUpdate":
			err = v.runtime.ExportTo(callback, &v.CB.OnBlockUpdate)
		case "SpawnParticle":
			err = v.runtime.ExportTo(callback, &v.CB.OnSpawnParticle)
		case "Packet":
			err = v.runtime.ExportTo(callback, &v.CB.OnPacket)
		}
		return err
	})
	v.runtime.GlobalObject().Set("events", events)

	fs := v.runtime.NewObject()
	fs.Set("create", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		file, err := os.Create(utils.PathData(name))
		if err != nil {
			return v.runtime.ToValue(fmt.Errorf("failed to create file '%s': %w", name, err))
		}

		obj := v.runtime.NewObject()
		obj.Set("write", func(call goja.FunctionCall) goja.Value {
			data := call.Argument(0).String()
			_, err := file.WriteString(data)
			if err != nil {
				return v.runtime.ToValue(fmt.Errorf("failed to write to file '%s': %w", name, err))
			}
			return goja.Undefined()
		})
		obj.Set("close", func(call goja.FunctionCall) goja.Value {
			err := file.Close()
			if err != nil {
				return v.runtime.ToValue(fmt.Errorf("failed to close file '%s': %w", name, err))
			}
			return goja.Undefined()
		})

		return obj
	})

	v.runtime.GlobalObject().Set("fs", fs)

	v.runtime.Set("displayChatMessage", func(call goja.FunctionCall) goja.Value {
		msg := call.Argument(0).ToString().String()
		v.DisplayChatMessage(msg)
		return goja.Undefined()
	})

	v.runtime.Set("setIngameMap", func(call goja.FunctionCall) goja.Value {
		enabled := call.Argument(0).ToBoolean()
		v.SetIngameMap(enabled)
		return goja.Undefined()
	})

	chunks := v.runtime.NewObject()
	chunks.Set("get", func(call goja.FunctionCall) goja.Value {
		x := call.Argument(0).ToInteger()
		z := call.Argument(1).ToInteger()

		obj := v.runtime.NewObject()

		ch, ok, err := v.GetWorld().LoadChunk(world.ChunkPos{int32(x), int32(z)})
		if err != nil {
			v.log.Error("loadChunk", err)
		}
		if !ok {
			return goja.Null()
		}

		// getBlockAt(x, y, z)
		obj.Set("getBlockAt", func(call goja.FunctionCall) goja.Value {
			bx := int(call.Argument(0).ToInteger())
			by := int(call.Argument(1).ToInteger())
			bz := int(call.Argument(2).ToInteger())

			name, state, ok := ch.BlockRegistry.RuntimeIDToState(ch.Block(uint8(bx), int16(by), uint8(bz), 0))
			if !ok {
				return goja.Null()
			}

			blockObj := v.runtime.NewObject()
			blockObj.Set("name", name)
			blockObj.Set("state", state)
			return blockObj
		})

		// getHighestBlockAt(x, z)
		obj.Set("getHighestBlockAt", func(call goja.FunctionCall) goja.Value {
			bx := int(call.Argument(0).ToInteger())
			bz := int(call.Argument(1).ToInteger())
			y := ch.HighestBlock(uint8(bx), uint8(bz))
			return v.runtime.ToValue(y)
		})

		// getBlockEntities()
		obj.Set("getBlockEntities", func(call goja.FunctionCall) goja.Value {
			var objs []any
			for pos, data := range ch.BlockEntities {
				objs = append(objs, map[string]any{
					"x":    pos.X(),
					"y":    pos.Y(),
					"z":    pos.Z(),
					"id":   data["id"],
					"data": data,
				})
			}
			return v.runtime.NewArray(objs...)
		})

		return obj
	})
	v.runtime.GlobalObject().Set("chunks", chunks)

	return v
}

func (v *VM) Load(script string) error {
	_, err := v.runtime.RunScript("script.js", script)
	if err != nil {
		return err
	}
	return nil
}
