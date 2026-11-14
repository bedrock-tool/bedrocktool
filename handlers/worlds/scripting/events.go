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

func (v *VM) OnEntityAdd(entity *entity.Entity, isNew bool, timeReceived time.Time) (apply bool) {
	if v.CB.OnEntityAdd == nil {
		return true
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	apply = true
	err := utils.RecoverCall(func() error {
		applyV := v.CB.OnEntityAdd(
			entity,
			newEntityDataObject(v.runtime, entity.Metadata),
			float64(timeReceived.UnixMilli()),
			isNew,
		)
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

func (v *VM) OnEntityUpdate(
	entity *entity.Entity,
	prevPosition *mgl32.Vec3,
	changedProperties map[string]any,
	timeReceived time.Time,
) {
	if v.CB.OnEntityUpdate == nil {
		return
	}
	v.lock.Lock()
	defer v.lock.Unlock()
	err := utils.RecoverCall(func() error {
		update := v.runtime.NewObject()
		update.Set("Entity", entity)
		update.Set("Metadata", newEntityDataObject(v.runtime, entity.Metadata))
		if prevPosition != nil {
			update.Set("PreviousPosition", prevPosition)
		}
		if changedProperties != nil {
			update.Set("ChangedProperties", changedProperties)
		}
		v.CB.OnEntityUpdate(
			update,
			float64(timeReceived.UnixMilli()),
		)
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
}

func (v *VM) OnEntityPropertyChange(
	entity *entity.Entity,
	name string,
	previous any,
	newValue any,
) bool {
	if v.CB.OnEntityPropertyChange == nil {
		return true
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	apply := true
	err := utils.RecoverCall(func() error {
		applyV := v.CB.OnEntityPropertyChange(entity, name, previous, newValue)
		apply = goja.IsUndefined(applyV) || applyV.ToBoolean()
		return nil
	})
	if err != nil {
		v.log.Error(err)
	}
	return apply
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

// PlayerInfo is what a PlayerAdd callback gets to decide whether a player is
// saved.
type PlayerInfo struct {
	UUID      string
	Username  string
	XUID      string
	IsNPC     bool
	HasSkin   bool
	RuntimeID uint64
	UniqueID  int64
	Position  mgl32.Vec3
	Metadata  protocol.EntityMetadata
}

// OnPlayerAdd asks the script whether to save a player. ok is false when
// the script leaves it to the usual rules (no callback, or it returned
// nothing).
func (v *VM) OnPlayerAdd(p PlayerInfo, timeReceived time.Time) (save bool, ok bool) {
	if v.CB.OnPlayerAdd == nil {
		return false, false
	}

	v.lock.Lock()
	defer v.lock.Unlock()
	err := utils.RecoverCall(func() error {
		obj := v.runtime.NewObject()
		obj.Set("UUID", p.UUID)
		obj.Set("Username", p.Username)
		obj.Set("XUID", p.XUID)
		obj.Set("IsNPC", p.IsNPC)
		obj.Set("HasSkin", p.HasSkin)
		obj.Set("RuntimeID", p.RuntimeID)
		obj.Set("UniqueID", p.UniqueID)
		obj.Set("Position", p.Position)
		res := v.CB.OnPlayerAdd(obj, newEntityDataObject(v.runtime, p.Metadata), float64(timeReceived.UnixMilli()))
		if res != nil && !goja.IsUndefined(res) && !goja.IsNull(res) {
			save, ok = res.ToBoolean(), true
		}
		return nil
	})
	if err != nil {
		v.log.Error(err)
		return false, false
	}
	return save, ok
}
