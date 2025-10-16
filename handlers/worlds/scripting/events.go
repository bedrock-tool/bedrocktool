package scripting

import (
	"reflect"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/entity"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/dop251/goja"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (v *VM) OnEntityAdd(entity *entity.Entity, timeReceived time.Time) (apply bool) {
	if v.CB.OnEntityAdd == nil {
		return true
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	apply = true
	err := utils.RecoverCall(func() error {
		applyV := v.CB.OnEntityAdd(entity, newEntityDataObject(v.runtime, entity.Metadata), float64(timeReceived.UnixMilli()))
		if !goja.IsUndefined(applyV) {
			apply = applyV.ToBoolean()
		}
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
	return
}

func (v *VM) OnEntityDataUpdate(entity *entity.Entity, timeReceived time.Time) {
	if v.CB.OnEntityDataUpdate == nil {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	err := utils.RecoverCall(func() error {
		v.CB.OnEntityDataUpdate(entity, newEntityDataObject(v.runtime, entity.Metadata), float64(timeReceived.UnixMilli()))
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

	v.lock.Lock()
	defer v.lock.Unlock()
	apply = true
	err := utils.RecoverCall(func() error {
		applyV := v.CB.OnChunkAdd(pos, float64(timeReceived.UnixMilli()))
		apply = goja.IsUndefined(applyV) || applyV.ToBoolean()
		return nil
	})
	if err != nil {
		v.log.Error(err)
		apply = true
	}
	return apply
}

func (v *VM) OnChunkData(pos world.ChunkPos) {
	if v.CB.OnChunkData == nil {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	err := utils.RecoverCall(func() error {
		v.CB.OnChunkData(pos)
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnBlockUpdate(name string, properties map[string]any, pos protocol.BlockPos, timeReceived time.Time) (apply bool) {
	if v.CB.OnBlockUpdate == nil {
		return true
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	apply = true
	err := utils.RecoverCall(func() error {
		applyV := v.CB.OnBlockUpdate(name, properties, pos, float64(timeReceived.UnixMilli()))
		apply = goja.IsUndefined(applyV) || applyV.ToBoolean()
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

	err := utils.RecoverCall(func() error {
		v.CB.OnSpawnParticle(name, position, float64(timeReceived.UnixMilli()))
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnPacket(pk packet.Packet, toServer bool, timeReceived time.Time) (apply bool) {
	if v.CB.OnPacket == nil {
		return true
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	err := utils.RecoverCall(func() error {
		packetName := strings.Split(reflect.TypeOf(pk).String(), ".")[1]
		applyV := v.CB.OnPacket(packetName, pk, toServer, float64(timeReceived.UnixMilli()))
		apply = goja.IsUndefined(applyV) || applyV.ToBoolean()
		return nil
	})
	if err != nil {
		v.log.Error(err)
		return true
	}
	return apply
}
