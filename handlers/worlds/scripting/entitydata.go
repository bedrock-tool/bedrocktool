package scripting

import (
	"slices"

	"github.com/dop251/goja"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

var EntityKeys = []string{
	"Flags",
	"StructuralIntegrity",
	"Variant",
	"ColorIndex",
	"Name",
	"Owner",
	"Target",
	"AirSupply",
	"EffectColor",
	"EffectAmbience",
	"JumpDuration",
	"Hurt",
	"HurtDirection",
	"RowTimeLeft",
	"RowTimeRight",
	"Value",
	"DisplayTileRuntimeID",
	"DisplayOffset",
	"CustomDisplay",
	"Swell",
	"OldSwell",
	"SwellDirection",
	"ChargeAmount",
	"CarryBlockRuntimeID",
	"ClientEvent",
	"UsingItem",
	"PlayerFlags",
	"PlayerIndex",
	"BedPosition",
	"PowerX",
	"PowerY",
	"PowerZ",
	"AuxPower",
	"FishX",
	"FishZ",
	"FishAngle",
	"AuxValueData",
	"LeashHolder",
	"Scale",
	"HasNPC",
	"NPCData",
	"Actions",
	"AirSupplyMax",
	"MarkVariant",
	"ContainerType",
	"ContainerSize",
	"ContainerStrengthModifier",
	"BlockTarget",
	"Inventory",
	"TargetA",
	"TargetB",
	"TargetC",
	"AerialAttack",
	"Width",
	"Height",
	"FuseTime",
	"SeatOffset",
	"SeatLockPassengerRotation",
	"SeatLockPassengerRotationDegrees",
	"SeatRotationOffset",
	"SeatRotationOffsetDegrees",
	"DataRadius",
	"DataWaiting",
	"DataParticle",
	"PeekID",
	"AttachFace",
	"Attached",
	"AttachedPosition",
	"TradeTarget",
	"Career",
	"HasCommandBlock",
	"CommandName",
	"LastCommandOutput",
	"TrackCommandOutput",
	"ControllingSeatIndex",
	"Strength",
	"StrengthMax",
	"DataSpellCastingColor",
	"DataLifetimeTicks",
	"PoseIndex",
	"DataTickOffset",
	"AlwaysShowNameTag",
	"ColorTwoIndex",
	"NameAuthor",
	"Score",
	"BalloonAnchor",
	"PuffedState",
	"BubbleTime",
	"Agent",
	"SittingAmount",
	"SittingAmountPrevious",
	"EatingCounter",
	"FlagsTwo",
	"LayingAmount",
	"LayingAmountPrevious",
	"DataDuration",
	"DataSpawnTime",
	"DataChangeRate",
	"DataChangeOnPickup",
	"DataPickupCount",
	"InteractText",
	"TradeTier",
	"MaxTradeTier",
	"TradeExperience",
	"SkinID",
	"SpawningFrames",
	"CommandBlockTickDelay",
	"CommandBlockExecuteOnFirstTick",
	"AmbientSoundInterval",
	"AmbientSoundIntervalRange",
	"AmbientSoundEventName",
	"FallDamageMultiplier",
	"NameRawText",
	"CanRideTarget",
	"LowTierCuredTradeDiscount",
	"HighTierCuredTradeDiscount",
	"NearbyCuredTradeDiscount",
	"NearbyCuredDiscountTimeStamp",
	"HitBox",
	"IsBuoyant",
	"FreezingEffectStrength",
	"BuoyancyData",
	"GoatHornCount",
	"BaseRuntimeID",
	"MovementSoundDistanceOffset",
	"HeartbeatIntervalTicks",
	"HeartbeatSoundEvent",
	"PlayerLastDeathPosition",
	"PlayerLastDeathDimension",
	"PlayerHasDied",
	"CollisionBox",
	"VisibleMobEffects",
}

var EntityFlags = []string{
	"OnFire",
	"Sneaking",
	"Riding",
	"Sprinting",
	"UsingItem",
	"Invisible",
	"Tempted",
	"InLove",
	"Saddled",
	"Powered",
	"Ignited",
	"Baby",
	"Converting",
	"Critical",
	"ShowName",
	"AlwaysShowName",
	"NoAI",
	"Silent",
	"WallClimbing",
	"Climb",
	"Swim",
	"Fly",
	"Walk",
	"Resting",
	"Sitting",
	"Angry",
	"Interested",
	"Charged",
	"Tamed",
	"Orphaned",
	"Leashed",
	"Sheared",
	"Gliding",
	"Elder",
	"Moving",
	"Breathing",
	"Chested",
	"Stackable",
	"ShowBottom",
	"Standing",
	"Shaking",
	"Idling",
	"Casting",
	"Charging",
	"KeyboardControlled",
	"PowerJump",
	"Dash",
	"Lingering",
	"HasCollision",
	"HasGravity",
	"FireImmune",
	"Dancing",
	"Enchanted",
	"ReturnTrident",
	"ContainerPrivate",
	"Transforming",
	"DamageNearbyMobs",
	"Swimming",
	"Bribed",
	"Pregnant",
	"LayingEgg",
	"PassengerCanPick",
	"TransitionSitting",
	"Eating",
	"LayingDown",
	"Sneezing",
	"Trusting",
	"Rolling",
	"Scared",
	"InScaffolding",
	"OverScaffolding",
	"DescendThroughBlock",
	"Blocking",
	"TransitionBlocking",
	"BlockedUsingShield",
	"BlockedUsingDamagedShield",
	"Sleeping",
	"WantsToWake",
	"TradeInterest",
	"DoorBreaker",
	"BreakingObstruction",
	"DoorOpener",
	"Captain",
	"Stunned",
	"Roaring",
	"DelayedAttack",
	"AvoidingMobs",
	"AvoidingBlock",
	"FacingTargetToRangeAttack",
	"HiddenWhenInvisible",
	"InUI",
	"Stalking",
	"Emoting",
	"Celebrating",
	"Admiring",
	"CelebratingSpecial",
	"OutOfControl",
	"RamAttack",
	"PlayingDead",
	"InAscendingBlock",
	"OverDescendingBlock",
	"Croaking",
	"DigestMob",
	"JumpGoal",
	"Emerging",
	"Sniffing",
	"Digging",
	"SonicBoom",
	"HasDashTimeout",
	"PushTowardsClosestSpace",
	"Scenting",
	"Rising",
	"FeelingHappy",
	"Searching",
	"Crawling",
	"TimerFlag1",
	"TimerFlag2",
	"TimerFlag3",
}

type entityDataFlags struct {
	r       *goja.Runtime
	setFlag func(index uint8, b bool)
	getFlag func(index uint8) bool
}

func (m entityDataFlags) Get(key string) goja.Value {
	idx := slices.Index(EntityFlags, key)
	if idx == -1 {
		return goja.Null()
	}
	return m.r.ToValue(m.getFlag(uint8(idx)))
}

func (m entityDataFlags) Set(key string, val goja.Value) bool {
	idx := slices.Index(EntityFlags, key)
	if idx == -1 {
		return false
	}
	m.setFlag(uint8(idx), val.ToBoolean())
	return true
}

func (m entityDataFlags) Has(key string) bool {
	idx := slices.Index(EntityFlags, key)
	return idx != -1
}

func (m entityDataFlags) Delete(key string) bool {
	return false
}

func (m entityDataFlags) Keys() (keys []string) {
	for i, k := range EntityFlags {
		if m.getFlag(uint8(i)) {
			keys = append(keys, k)
		}
	}
	return
}

type entityDataObject struct {
	r *goja.Runtime
	d protocol.EntityMetadata

	flags *goja.Object
}

func newEntityDataObject(r *goja.Runtime, meta protocol.EntityMetadata) *goja.Object {
	return r.NewDynamicObject(entityDataObject{
		r: r,
		d: meta,
		flags: r.NewDynamicObject(entityDataFlags{
			r: r,
			getFlag: func(index uint8) bool {
				return meta.Flag(protocol.EntityDataKeyFlags, index)
			},
			setFlag: func(index uint8, b bool) {
				s := meta.Flag(protocol.EntityDataKeyFlags, index)
				if s != b {
					meta.SetFlag(protocol.EntityDataKeyFlags, index)
				}
			},
		}),
	})
}

func (m entityDataObject) Get(key string) goja.Value {
	idx := slices.Index(EntityKeys, key)
	if idx == -1 {
		return goja.Null()
	}

	if key == "Flags" {
		return m.flags
	}

	obj := m.d[uint32(idx)]
	return m.r.ToValue(obj)
}

func (m entityDataObject) Set(key string, val goja.Value) bool {
	idx := slices.Index(EntityKeys, key)
	if idx == -1 {
		return false
	}

	m.d[uint32(idx)] = val.Export()
	return true
}

func (m entityDataObject) Has(key string) bool {
	idx := slices.Index(EntityKeys, key)
	return idx != -1
}

func (m entityDataObject) Delete(key string) bool {
	idx := slices.Index(EntityKeys, key)
	if idx == -1 {
		return false
	}
	delete(m.d, uint32(idx))
	return true
}

func (m entityDataObject) Keys() (keys []string) {
	for i := range m.d {
		keys = append(keys, EntityKeys[i])
	}
	return
}
