package merge

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type DummyEntity struct {
	T world.EntityType
}

func (e *DummyEntity) Close() error {
	return nil
}

// Type returns the EntityType of the Entity.
func (e *DummyEntity) Type() world.EntityType {
	return e.T
}

// Position returns the current position of the entity in the world.
func (e *DummyEntity) Position() mgl64.Vec3 {
	return mgl64.Vec3{}
}

// Rotation returns the yaw and pitch of the entity in degrees. Yaw is horizontal rotation (rotation around the
// vertical axis, 0 when facing forward), pitch is vertical rotation (rotation around the horizontal axis, also 0
// when facing forward).
func (e *DummyEntity) Rotation() cube.Rotation {
	return cube.Rotation{}
}

// World returns the current world of the entity. This is always the world that the entity can actually be
// found in.
func (e *DummyEntity) World() *world.World {
	return nil
}

type DummyEntityType struct {
	name string
	NBT  map[string]any
}

func (t *DummyEntityType) EncodeEntity() string {
	return t.name
}

func (t *DummyEntityType) BBox(e world.Entity) cube.BBox {
	return cube.Box(0, 0, 0, 1, 1, 1)
}

func (t *DummyEntityType) DecodeNBT(m map[string]any) world.Entity {
	t.NBT = m
	return &DummyEntity{T: t}
}

func (t *DummyEntityType) EncodeNBT(e world.Entity) map[string]any {
	return t.NBT
}

type EntityRegistry struct{}

// Lookup looks up an EntityType by its name. If found, the EntityType is
// returned and the bool is true. The bool is false otherwise.
func (reg EntityRegistry) Lookup(name string) (world.EntityType, bool) {
	return &DummyEntityType{name: name}, true
}

func (reg EntityRegistry) Config() world.EntityRegistryConfig {
	return world.EntityRegistryConfig{}
}

func (reg EntityRegistry) Types() []world.EntityType {
	return nil
}
