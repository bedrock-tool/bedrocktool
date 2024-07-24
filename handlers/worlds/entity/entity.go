package entity

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type RuntimeID = uint64
type UniqueID = int64

const (
	PropertyTypeInt = iota
	PropertyTypeFloat
	PropertyTypeBool
	PropertyTypeEnum
)

type Entity struct {
	RuntimeID  RuntimeID
	UniqueID   UniqueID
	EntityType string

	Position            mgl32.Vec3
	Pitch, Yaw, HeadYaw float32
	Velocity            mgl32.Vec3
	HasMoved            bool

	Metadata   protocol.EntityMetadata
	Properties map[string]*EntityProperty

	Inventory  map[byte]map[byte]protocol.ItemInstance
	Helmet     *protocol.ItemInstance
	Chestplate *protocol.ItemInstance
	Leggings   *protocol.ItemInstance
	Boots      *protocol.ItemInstance
}

type EntityProperty struct {
	Type  int32
	Name  string
	Min   float32
	Max   float32
	Enum  []any
	Value any
}

type serverEntityType struct {
	Encoded string
	NBT     map[string]any
}

func (t serverEntityType) EncodeEntity() string {
	return t.Encoded
}

func (t serverEntityType) BBox(e world.Entity) cube.BBox {
	return cube.Box(-0.5, 0, -0.5, 0.5, 1, 0.5)
}

func (t serverEntityType) DecodeNBT(m map[string]any) world.Entity {
	return nil // not implemented, and never should
}

func (t serverEntityType) EncodeNBT(e world.Entity) map[string]any {
	return t.NBT
}

type serverEntity struct {
	world.Entity
	EntityType serverEntityType
}

var _ world.SaveableEntityType = &serverEntityType{}

func (e serverEntity) Type() world.EntityType {
	return e.EntityType
}

var flagNames = map[uint8]string{
	protocol.EntityDataFlagSheared:      "Sheared",
	protocol.EntityDataFlagCaptain:      "IsIllagerCaptain",
	protocol.EntityDataFlagSitting:      "Sitting",
	protocol.EntityDataFlagBaby:         "IsBaby",
	protocol.EntityDataFlagTamed:        "IsTamed",
	protocol.EntityDataFlagTrusting:     "IsTrusting",
	protocol.EntityDataFlagOrphaned:     "IsOrphaned",
	protocol.EntityDataFlagAngry:        "IsAngry",
	protocol.EntityDataFlagOutOfControl: "IsOutOfControl",
	protocol.EntityDataFlagSaddled:      "Saddled",
	protocol.EntityDataFlagChested:      "Chested",
	protocol.EntityDataFlagShowBottom:   "ShowBottom",
	protocol.EntityDataFlagGliding:      "IsGliding",
	protocol.EntityDataFlagSwimming:     "IsSwimming",
	protocol.EntityDataFlagEating:       "IsEating",
	protocol.EntityDataFlagScared:       "IsScared",
	protocol.EntityDataFlagStunned:      "IsStunned",
	protocol.EntityDataFlagRoaring:      "IsRoaring",
}

func (s *Entity) toNBT(nbt map[string]any) {
	metadata := s.Metadata

	nbt["Persistent"] = true

	if variant, ok := metadata[protocol.EntityDataKeyVariant]; ok {
		nbt["Variant"] = variant
	}
	if markVariant, ok := metadata[protocol.EntityDataKeyMarkVariant]; ok {
		nbt["MarkVariant"] = markVariant
	}
	if color, ok := metadata[protocol.EntityDataKeyColorIndex]; ok {
		nbt["Color"] = color
	}
	if color2, ok := metadata[protocol.EntityDataKeyColorTwoIndex]; ok {
		nbt["Color2"] = color2
	}
	if skinID, ok := metadata[protocol.EntityDataKeySkinID]; ok {
		nbt["SkinID"] = skinID
	}

	if name, ok := metadata[protocol.EntityDataKeyName]; ok {
		nbt["CustomName"] = name
	}

	if rawName, ok := metadata[protocol.EntityDataKeyNameRawText].(string); ok {
		nbt["CustomName"] = rawName
	}

	if ShowNameTag, ok := metadata[protocol.EntityDataKeyAlwaysShowNameTag]; ok {
		if ShowNameTag != 0 {
			nbt["CustomNameVisible"] = true
		} else {
			nbt["CustomNameVisible"] = false
		}
	}

	speed := 0.25
	if !s.HasMoved {
		speed = 0
	}

	attributes := []any{
		map[string]any{
			"Name":       "minecraft:movement",
			"Base":       float32(speed),
			"Current":    float32(speed),
			"DefaultMax": float32(3.4028235e+38),
			"DefaultMin": float32(0),
			"Max":        float32(3.4028235e+38),
			"Min":        float32(0),
		},
		map[string]any{
			"Current":    float32(0.02),
			"DefaultMax": float32(3.4028235e+38),
			"DefaultMin": float32(0),
			"Max":        float32(3.4028235e+38),
			"Min":        float32(0),
			"Name":       "minecraft:underwater_movement",
			"Base":       float32(0.02),
		},
		map[string]any{
			"Min":        float32(0),
			"Name":       "minecraft:lava_movement",
			"Base":       float32(0.02),
			"Current":    float32(0.02),
			"DefaultMax": float32(3.4028235e+38),
			"DefaultMin": float32(0),
			"Max":        float32(3.4028235e+38),
		},
	}

	/*
		if !s.HasMoved {
			attributes = append(attributes, map[string]any{
				"Min":        float32(-1),
				"Max":        float32(1),
				"Name":       "minecraft:gravity",
				"Base":       float32(0),
				"Current":    float32(0),
				"DefaultMax": float32(1),
				"DefaultMin": float32(-1),
			})
		}

		scale, ok := metadata[protocol.EntityDataKeyScale]
		if ok {
			attributes = append(attributes, map[string]any{
				"Min":        float32(0),
				"Max":        math.MaxFloat32,
				"Name":       "minecraft:scale",
				"Base":       scale.(float32),
				"Current":    scale.(float32),
				"DefaultMax": math.MaxFloat32,
				"DefaultMin": float32(0),
			})
		}
	*/

	if _, ok := metadata[protocol.EntityDataKeyFlags]; ok {
		if metadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagNoAI) {
			nbt["IsAutonomous"] = false
		}
		for k, v := range flagNames {
			nbt[v] = metadata.Flag(protocol.EntityDataKeyFlags, k)
		}

		AlwaysShowName := metadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagAlwaysShowName)
		if AlwaysShowName {
			nbt["CustomNameVisible"] = true
		}

		type effect struct {
			Id                              byte
			Duration                        int32
			DurationEasy                    int32
			DurationNormal                  int32
			DurationHard                    int32
			FactorCalculationData           map[string]any
			ShowParticles                   bool
			Ambient                         bool
			Amplifier                       byte
			DisplayOnScreenTextureAnimation bool
		}

		activeEffects := []effect{}
		addEffect := func(id int, showParticles bool) {
			activeEffects = append(activeEffects, effect{
				Id:             byte(id),
				Duration:       math.MaxInt32,
				DurationEasy:   math.MaxInt32,
				DurationNormal: math.MaxInt32,
				DurationHard:   math.MaxInt32,
				FactorCalculationData: map[string]any{
					"change_timestamp": int32(0),
					"factor_current":   float32(0),
					"factor_previous":  float32(0),
					"factor_start":     float32(0),
					"factor_target":    float32(1),
					"had_applied":      uint8(0x1),
					"had_last_tick":    uint8(0x0),
					"padding_duration": int32(0),
				},
				ShowParticles:                   showParticles,
				Ambient:                         false,
				Amplifier:                       1,
				DisplayOnScreenTextureAnimation: false,
			})
		}

		scale, ok := metadata[protocol.EntityDataKeyScale]
		if !ok {
			scale = 1
		}

		invisible := metadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagInvisible)
		if invisible || scale == float32(0) || scale == 0 {
			addEffect(packet.EffectInvisibility, false)
		}

		if len(activeEffects) > 0 {
			nbt["ActiveEffects"] = activeEffects
		}
	}

	nbt["Attributes"] = attributes
}

func vec3float32(x mgl32.Vec3) []float32 {
	return []float32{float32(x[0]), float32(x[1]), float32(x[2])}
}

func (s *Entity) ToServerEntity(links []int64) serverEntity {
	e := serverEntity{
		EntityType: serverEntityType{
			Encoded: s.EntityType,
			NBT: map[string]any{
				"Pos":      vec3float32(s.Position),
				"Rotation": []float32{s.HeadYaw, s.Pitch},
				"Motion":   vec3float32(s.Velocity),
				"UniqueID": int64(s.UniqueID),
			},
		},
	}
	s.toNBT(e.EntityType.NBT)
	if len(s.Properties) > 0 {
		nbtProperties := map[string]any{}
		for name, prop := range s.Properties {
			nbtProperties[name] = prop.Value
		}
		e.EntityType.NBT["properties"] = nbtProperties
	}

	var linksTag []map[string]any
	for i, el := range links {
		linksTag = append(linksTag, map[string]any{
			"entityID": el,
			"linkID":   int32(i),
		})
	}
	if len(linksTag) > 0 {
		e.EntityType.NBT["LinksTag"] = linksTag
	}

	/*
		if false {
			armor := make([]map[string]any, 4)
			if s.Helmet != nil {
				armor[0] = nbtconv.WriteItem(utils.StackToItem(w.serverState.blocks, s.Helmet.Stack), true)
			}
			if s.Chestplate != nil {
				armor[1] = nbtconv.WriteItem(utils.StackToItem(w.serverState.blocks, s.Chestplate.Stack), true)
			}
			if s.Leggings != nil {
				armor[2] = nbtconv.WriteItem(utils.StackToItem(w.serverState.blocks, s.Leggings.Stack), true)
			}
			if s.Boots != nil {
				armor[3] = nbtconv.WriteItem(utils.StackToItem(w.serverState.blocks, s.Boots.Stack), true)
			}
			e.EntityType.NBT["Armor"] = armor
		}
	*/

	return e
}
