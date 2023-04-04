package seconduser

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type entityType struct {
	name string
}

func (t *entityType) EncodeEntity() string {
	return t.name
}

func (t *entityType) BBox(e world.Entity) cube.BBox {
	return cube.BBox{}
}

type serverEntity struct {
	t   world.EntityType
	pos mgl64.Vec3
	rot cube.Rotation
}

func (s *serverEntity) Close() error {
	return nil
}
func (s *serverEntity) Position() mgl64.Vec3 {
	return s.pos
}
func (s *serverEntity) Rotation() cube.Rotation {
	return s.rot
}
func (e *serverEntity) World() *world.World {
	w, _ := world.OfEntity(e)
	return w
}
func (s *serverEntity) Type() world.EntityType {
	return s.t
}

func newServerEntity(typename string) *serverEntity {
	return &serverEntity{
		t: &entityType{name: typename},
	}
}
