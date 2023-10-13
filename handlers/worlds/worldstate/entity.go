package worldstate

import (
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/bedrock-tool/bedrocktool/utils/nbtconv"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

type EntityState struct {
	RuntimeID  EntityRuntimeID
	UniqueID   EntityUniqueID
	EntityType string

	Position            mgl32.Vec3
	Pitch, Yaw, HeadYaw float32
	Velocity            mgl32.Vec3

	Metadata  protocol.EntityMetadata
	Inventory map[byte]map[byte]protocol.ItemInstance

	Helmet     *protocol.ItemInstance
	Chestplate *protocol.ItemInstance
	Leggings   *protocol.ItemInstance
	Boots      *protocol.ItemInstance
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

func (w *World) ProcessAddActor(pk *packet.AddActor, ignoreCB func(*EntityState) bool, bpCB func(behaviourpack.EntityIn)) {
	e, ok := w.GetEntity(pk.EntityRuntimeID)
	if !ok {
		e = &EntityState{
			RuntimeID:  pk.EntityRuntimeID,
			UniqueID:   pk.EntityUniqueID,
			EntityType: pk.EntityType,
			Inventory:  make(map[byte]map[byte]protocol.ItemInstance),
			Metadata:   make(map[uint32]any),
		}
	}
	e.Position = pk.Position
	e.Pitch = pk.Pitch
	e.Yaw = pk.Yaw
	e.HeadYaw = pk.HeadYaw
	e.Velocity = pk.Velocity

	metadata := make(protocol.EntityMetadata)
	maps.Copy(metadata, pk.EntityMetadata)
	e.Metadata = metadata

	ignore := ignoreCB(e)
	if ignore {
		logrus.Infof("Ignoring Entity: %s %d", e.EntityType, e.UniqueID)
		return
	}

	w.StoreEntity(uint64(pk.EntityUniqueID), e)
	for _, el := range pk.EntityLinks {
		w.AddEntityLink(el)
	}

	bpCB(behaviourpack.EntityIn{
		Identifier: pk.EntityType,
		Attr:       pk.Attributes,
		Meta:       pk.EntityMetadata,
	})
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

func entityMetadataToNBT(metadata protocol.EntityMetadata, nbt map[string]any) {
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
		nbt["SkinID"] = int32(skinID.(int32))
	}

	if name, ok := metadata[protocol.EntityDataKeyName]; ok {
		nbt["CustomName"] = name
	}
	if ShowNameTag, ok := metadata[protocol.EntityDataKeyAlwaysShowNameTag]; ok {
		if ShowNameTag != 0 {
			nbt["CustomNameVisible"] = true
		} else {
			nbt["CustomNameVisible"] = false
		}
	}

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
				Duration:       1000,
				DurationEasy:   1000,
				DurationNormal: 1000,
				DurationHard:   1000,
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
				ShowParticles:                   false,
				Ambient:                         false,
				Amplifier:                       1,
				DisplayOnScreenTextureAnimation: false,
			})
		}

		invisible := metadata.Flag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagInvisible)
		if invisible {
			addEffect(packet.EffectInvisibility, false)
		}

		if len(activeEffects) > 0 {
			nbt["ActiveEffects"] = activeEffects
		}
	}
}

func vec3float32(x mgl32.Vec3) []float32 {
	return []float32{float32(x[0]), float32(x[1]), float32(x[2])}
}

func (s *EntityState) ToServerEntity(links []int64) serverEntity {
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
	entityMetadataToNBT(s.Metadata, e.EntityType.NBT)

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

	if false {
		armor := make([]map[string]any, 4)
		if s.Helmet != nil {
			armor[0] = nbtconv.WriteItem(utils.StackToItem(s.Helmet.Stack), true)
		}
		if s.Chestplate != nil {
			armor[1] = nbtconv.WriteItem(utils.StackToItem(s.Chestplate.Stack), true)
		}
		if s.Leggings != nil {
			armor[2] = nbtconv.WriteItem(utils.StackToItem(s.Leggings.Stack), true)
		}
		if s.Boots != nil {
			armor[3] = nbtconv.WriteItem(utils.StackToItem(s.Boots.Stack), true)
		}
		e.EntityType.NBT["Armor"] = armor
	}

	e.EntityType.NBT["Attributes"] = []any{
		map[string]any{
			"Name":       "minecraft:movement",
			"Base":       float32(0.25),
			"Current":    float32(0.25),
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

	return e
}
