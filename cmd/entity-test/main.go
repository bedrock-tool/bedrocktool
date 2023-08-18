package main

import (
	"fmt"
	"os"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sirupsen/logrus"
)

type entityReg struct{}

func (r *entityReg) Config() world.EntityRegistryConfig {
	return world.EntityRegistryConfig{}
}
func (r *entityReg) Lookup(name string) (world.EntityType, bool) {
	return &serverEntityType{Encoded: name}, true
}
func (r *entityReg) Types() []world.EntityType {
	return []world.EntityType{nil}
}

type serverEntityType struct {
	Encoded string
}

func (t serverEntityType) EncodeEntity() string {
	return t.Encoded
}

func (t serverEntityType) BBox(e world.Entity) cube.BBox {
	return cube.Box(-0.5, 0, -0.5, 0.5, 1, 0.5)
}

func (t serverEntityType) DecodeNBT(m map[string]any) world.Entity {
	return &serverEntity{
		EntityType: t,
		NBT:        m,
	}
}

func (t serverEntityType) EncodeNBT(e world.Entity) map[string]any {
	se := e.(*serverEntity)
	return se.NBT
}

var _ world.SaveableEntityType = &serverEntityType{}

type serverEntity struct {
	EntityType serverEntityType
	NBT        map[string]any
}

func (e serverEntity) Type() world.EntityType {
	return e.EntityType
}

func (e serverEntity) Position() mgl64.Vec3 {
	return mgl64.Vec3{}
}
func (e serverEntity) Rotation() cube.Rotation {
	return cube.Rotation{}
}

func (e serverEntity) World() *world.World {
	return nil
}

func (e serverEntity) Close() error {
	return nil
}

func main() {
	world, err := mcdb.Config{
		Entities: &entityReg{},
		ReadOnly: true,
	}.Open(os.Args[1])
	if err != nil {
		logrus.Fatal(err)
	}

	it := world.NewColumnIterator(nil, false)
	defer it.Release()
	for it.Next() {
		c := it.Column()
		if err = it.Error(); err != nil {
			logrus.Fatal(err)
		}

		for _, e := range c.Entities {
			se := e.(*serverEntity)
			fmt.Printf("%s\n", e.Type().EncodeEntity())
			utils.DumpStruct(os.Stdout, se.NBT)
			fmt.Print("\n\n")
		}
	}

}
