package utils

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func DumpStruct(f io.StringWriter, inputStruct any) {
	dumpValue(f, 0, reflect.ValueOf(inputStruct), true, false)
}

func dumpValue(f io.StringWriter, level int, value reflect.Value, withType, isEntityMetadata bool) {
	tabs := strings.Repeat("\t", level)

	typeName := value.Type().String()
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			f.WriteString("nil")
			return
		}
		value = value.Elem()
	}
	if stringer := value.MethodByName("String"); stringer.IsValid() {
		v := stringer.Call(nil)
		value = v[0]
	}

	valueType := value.Type()

	if strings.HasPrefix(typeName, "protocol.Optional") {
		v := value.MethodByName("Value").Call(nil)
		val, set := v[0], v[1]
		if !set.Bool() {
			f.WriteString(typeName + " Not Set")
		} else {
			f.WriteString(typeName + "{\n" + tabs + "\t")
			dumpValue(f, level+1, val, false, false)
			f.WriteString("\n" + tabs + "}")
		}
		return
	}

	switch valueType.Kind() {
	case reflect.Struct:
		f.WriteString(typeName + "{")
		if valueType.NumField() == 0 {
			f.WriteString("}")
			return
		}
		f.WriteString("\n")
		for i := 0; i < valueType.NumField(); i++ {
			fieldType := valueType.Field(i)
			if fieldType.IsExported() {
				f.WriteString(tabs + "\t" + fieldType.Name + ": ")
				dumpValue(f, level+1, value.Field(i), true, fieldType.Name == "EntityMetadata")
				f.WriteString(",\n")
			} else {
				f.WriteString(tabs + "\t" + fieldType.Name + " (unexported)\n")
			}
		}
		f.WriteString(tabs + "}")

	case reflect.Map:
		mapValueType := valueType.Elem().String()
		isAny := false
		if mapValueType == "interface {}" {
			mapValueType = "any"
			isAny = true
		}
		mapKeyType := valueType.Key().String()
		f.WriteString("map[" + mapKeyType + "]" + mapValueType + "{")
		if value.Len() == 0 {
			f.WriteString("}")
			return
		}
		f.WriteString("\n")

		if isEntityMetadata {
			meta := protocol.EntityMetadata(value.Interface().(map[uint32]any))
			if _, ok := meta[protocol.EntityDataKeyFlags]; ok {
				f.WriteString(tabs + "\tFlags: ")
				var first = true
				for i, name := range entityDataFlags {
					if meta.Flag(protocol.EntityDataKeyFlags, uint8(i)) {
						if !first {
							f.WriteString("|")
						}
						f.WriteString(name[len("EntityDataFlag"):])
						first = false
					}
				}
				f.WriteString(",\n")
			}
		}

		iter := value.MapRange()
		for iter.Next() {
			f.WriteString(tabs + "\t")
			var kevV bool
			if isEntityMetadata {
				idx := int(iter.Key().Uint())
				if idx == 0 {
					continue
				}
				if idx < len(entityDataKeys) {
					vv := entityDataKeys[idx][len("EntityDataKey"):]
					f.WriteString(vv)
					kevV = true
				}
			}
			if !kevV {
				dumpValue(f, level+1, iter.Key(), false, false)
			}
			f.WriteString(": ")
			elem := iter.Value()
			if isAny {
				elem = elem.Elem()
			}
			dumpValue(f, level+1, elem, isAny, false)
			f.WriteString(",\n")
		}
		f.WriteString(tabs + "}")

	case reflect.Slice, reflect.Array:
		elemType := valueType.Elem()
		elemTypeString := elemType.String()
		if elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}

		isAny := false
		if elemType.Kind() == reflect.Interface {
			elemTypeString = "any"
			isAny = true
		}

		f.WriteString("[]" + elemTypeString + "{")
		if value.Len() == 0 {
			f.WriteString("}")
			return
		}
		if value.Len() > 1000 {
			f.WriteString("<slice to long>}")
			return
		}
		isStructish := false
		switch elemType.Kind() {
		case reflect.Struct, reflect.Map, reflect.Slice:
			f.WriteString("\n")
			isStructish = true
		}
		for i := 0; i < value.Len(); i++ {
			if isStructish {
				f.WriteString(tabs + "\t")
			}
			elem := value.Index(i)
			if isAny {
				elem = elem.Elem()
			}
			dumpValue(f, level+1, elem, isAny, false)
			if isStructish {
				f.WriteString(",\n")
			} else if i == value.Len()-1 {
				f.WriteString("}")
			} else {
				f.WriteString(", ")
			}
		}
		if isStructish {
			f.WriteString(tabs + "}")
		}

	case reflect.String:
		f.WriteString("\"" + value.String() + "\"")

	case reflect.Bool:
		if value.Bool() {
			f.WriteString("true")
		} else {
			f.WriteString("false")
		}

	default:
		if withType {
			f.WriteString(typeName + "(")
		}
		switch valueType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.WriteString(strconv.FormatInt(value.Int(), 10))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.WriteString(strconv.FormatInt(int64(value.Uint()), 10))
		case reflect.Uintptr:
			f.WriteString("0x" + strconv.FormatInt(int64(value.Uint()), 16))
		case reflect.Float32:
			f.WriteString(strconv.FormatFloat(float64(value.Float()), 'g', 9, 32))
		case reflect.Float64:
			f.WriteString(strconv.FormatFloat(float64(value.Float()), 'g', 9, 64))
		default:
			f.WriteString(fmt.Sprintf("%#+v", value.Interface()))
		}
		if withType {
			f.WriteString(")")
		}
	}
}

var entityDataKeys = []string{
	"EntityDataKeyFlags",
	"EntityDataKeyStructuralIntegrity",
	"EntityDataKeyVariant",
	"EntityDataKeyColorIndex",
	"EntityDataKeyName",
	"EntityDataKeyOwner",
	"EntityDataKeyTarget",
	"EntityDataKeyAirSupply",
	"EntityDataKeyEffectColor",
	"EntityDataKeyEffectAmbience",
	"EntityDataKeyJumpDuration",
	"EntityDataKeyHurt",
	"EntityDataKeyHurtDirection",
	"EntityDataKeyRowTimeLeft",
	"EntityDataKeyRowTimeRight",
	"EntityDataKeyValue",
	"EntityDataKeyDisplayTileRuntimeID",
	"EntityDataKeyDisplayOffset",
	"EntityDataKeyCustomDisplay",
	"EntityDataKeySwell",
	"EntityDataKeyOldSwell",
	"EntityDataKeySwellDirection",
	"EntityDataKeyChargeAmount",
	"EntityDataKeyCarryBlockRuntimeID",
	"EntityDataKeyClientEvent",
	"EntityDataKeyUsingItem",
	"EntityDataKeyPlayerFlags",
	"EntityDataKeyPlayerIndex",
	"EntityDataKeyBedPosition",
	"EntityDataKeyPowerX",
	"EntityDataKeyPowerY",
	"EntityDataKeyPowerZ",
	"EntityDataKeyAuxPower",
	"EntityDataKeyFishX",
	"EntityDataKeyFishZ",
	"EntityDataKeyFishAngle",
	"EntityDataKeyAuxValueData",
	"EntityDataKeyLeashHolder",
	"EntityDataKeyScale",
	"EntityDataKeyHasNPC",
	"EntityDataKeyNPCData",
	"EntityDataKeyActions",
	"EntityDataKeyAirSupplyMax",
	"EntityDataKeyMarkVariant",
	"EntityDataKeyContainerType",
	"EntityDataKeyContainerSize",
	"EntityDataKeyContainerStrengthModifier",
	"EntityDataKeyBlockTarget",
	"EntityDataKeyInventory",
	"EntityDataKeyTargetA",
	"EntityDataKeyTargetB",
	"EntityDataKeyTargetC",
	"EntityDataKeyAerialAttack",
	"EntityDataKeyWidth",
	"EntityDataKeyHeight",
	"EntityDataKeyFuseTime",
	"EntityDataKeySeatOffset",
	"EntityDataKeySeatLockPassengerRotation",
	"EntityDataKeySeatLockPassengerRotationDegrees",
	"EntityDataKeySeatRotationOffset",
	"EntityDataKeySeatRotationOffstDegrees",
	"EntityDataKeyDataRadius",
	"EntityDataKeyDataWaiting",
	"EntityDataKeyDataParticle",
	"EntityDataKeyPeekID",
	"EntityDataKeyAttachFace",
	"EntityDataKeyAttached",
	"EntityDataKeyAttachedPosition",
	"EntityDataKeyTradeTarget",
	"EntityDataKeyCareer",
	"EntityDataKeyHasCommandBlock",
	"EntityDataKeyCommandName",
	"EntityDataKeyLastCommandOutput",
	"EntityDataKeyTrackCommandOutput",
	"EntityDataKeyControllingSeatIndex",
	"EntityDataKeyStrength",
	"EntityDataKeyStrengthMax",
	"EntityDataKeyDataSpellCastingColor",
	"EntityDataKeyDataLifetimeTicks",
	"EntityDataKeyPoseIndex",
	"EntityDataKeyDataTickOffset",
	"EntityDataKeyAlwaysShowNameTag",
	"EntityDataKeyColorTwoIndex",
	"EntityDataKeyNameAuthor",
	"EntityDataKeyScore",
	"EntityDataKeyBalloonAnchor",
	"EntityDataKeyPuffedState",
	"EntityDataKeyBubbleTime",
	"EntityDataKeyAgent",
	"EntityDataKeySittingAmount",
	"EntityDataKeySittingAmountPrevious",
	"EntityDataKeyEatingCounter",
	"EntityDataKeyFlagsTwo",
	"EntityDataKeyLayingAmount",
	"EntityDataKeyLayingAmountPrevious",
	"EntityDataKeyDataDuration",
	"EntityDataKeyDataSpawnTime",
	"EntityDataKeyDataChangeRate",
	"EntityDataKeyDataChangeOnPickup",
	"EntityDataKeyDataPickupCount",
	"EntityDataKeyInteractText",
	"EntityDataKeyTradeTier",
	"EntityDataKeyMaxTradeTier",
	"EntityDataKeyTradeExperience",
	"EntityDataKeySkinID",
	"EntityDataKeySpawningFrames",
	"EntityDataKeyCommandBlockTickDelay",
	"EntityDataKeyCommandBlockExecuteOnFirstTick",
	"EntityDataKeyAmbientSoundInterval",
	"EntityDataKeyAmbientSoundIntervalRange",
	"EntityDataKeyAmbientSoundEventName",
	"EntityDataKeyFallDamageMultiplier",
	"EntityDataKeyNameRawText",
	"EntityDataKeyCanRideTarget",
	"EntityDataKeyLowTierCuredTradeDiscount",
	"EntityDataKeyHighTierCuredTradeDiscount",
	"EntityDataKeyNearbyCuredTradeDiscount",
	"EntityDataKeyNearbyCuredDiscountTimeStamp",
	"EntityDataKeyHitBox",
	"EntityDataKeyIsBuoyant",
	"EntityDataKeyFreezingEffectStrength",
	"EntityDataKeyBuoyancyData",
	"EntityDataKeyGoatHornCount",
	"EntityDataKeyBaseRuntimeID",
	"EntityDataKeyMovementSoundDistanceOffset",
	"EntityDataKeyHeartbeatIntervalTicks",
	"EntityDataKeyHeartbeatSoundEvent",
	"EntityDataKeyPlayerLastDeathPosition",
	"EntityDataKeyPlayerLastDeathDimension",
	"EntityDataKeyPlayerHasDied",
	"EntityDataKeyCollisionBox",
}

var entityDataFlags = []string{
	"EntityDataFlagOnFire",
	"EntityDataFlagSneaking",
	"EntityDataFlagRiding",
	"EntityDataFlagSprinting",
	"EntityDataFlagUsingItem",
	"EntityDataFlagInvisible",
	"EntityDataFlagTempted",
	"EntityDataFlagInLove",
	"EntityDataFlagSaddled",
	"EntityDataFlagPowered",
	"EntityDataFlagIgnited",
	"EntityDataFlagBaby",
	"EntityDataFlagConverting",
	"EntityDataFlagCritical",
	"EntityDataFlagShowName",
	"EntityDataFlagAlwaysShowName",
	"EntityDataFlagNoAI",
	"EntityDataFlagSilent",
	"EntityDataFlagWallClimbing",
	"EntityDataFlagClimb",
	"EntityDataFlagSwim",
	"EntityDataFlagFly",
	"EntityDataFlagWalk",
	"EntityDataFlagResting",
	"EntityDataFlagSitting",
	"EntityDataFlagAngry",
	"EntityDataFlagInterested",
	"EntityDataFlagCharged",
	"EntityDataFlagTamed",
	"EntityDataFlagOrphaned",
	"EntityDataFlagLeashed",
	"EntityDataFlagSheared",
	"EntityDataFlagGliding",
	"EntityDataFlagElder",
	"EntityDataFlagMoving",
	"EntityDataFlagBreathing",
	"EntityDataFlagChested",
	"EntityDataFlagStackable",
	"EntityDataFlagShowBottom",
	"EntityDataFlagStanding",
	"EntityDataFlagShaking",
	"EntityDataFlagIdling",
	"EntityDataFlagCasting",
	"EntityDataFlagCharging",
	"EntityDataFlagKeyboardControlled",
	"EntityDataFlagPowerJump",
	"EntityDataFlagDash",
	"EntityDataFlagLingering",
	"EntityDataFlagHasCollision",
	"EntityDataFlagHasGravity",
	"EntityDataFlagFireImmune",
	"EntityDataFlagDancing",
	"EntityDataFlagEnchanted",
	"EntityDataFlagReturnTrident",
	"EntityDataFlagContainerPrivate",
	"EntityDataFlagTransforming",
	"EntityDataFlagDamageNearbyMobs",
	"EntityDataFlagSwimming",
	"EntityDataFlagBribed",
	"EntityDataFlagPregnant",
	"EntityDataFlagLayingEgg",
	"EntityDataFlagPassengerCanPick",
	"EntityDataFlagTransitionSitting",
	"EntityDataFlagEating",
	"EntityDataFlagLayingDown",
	"EntityDataFlagSneezing",
	"EntityDataFlagTrusting",
	"EntityDataFlagRolling",
	"EntityDataFlagScared",
	"EntityDataFlagInScaffolding",
	"EntityDataFlagOverScaffolding",
	"EntityDataFlagDescendThroughBlock",
	"EntityDataFlagBlocking",
	"EntityDataFlagTransitionBlocking",
	"EntityDataFlagBlockedUsingShield",
	"EntityDataFlagBlockedUsingDamagedShield",
	"EntityDataFlagSleeping",
	"EntityDataFlagWantsToWake",
	"EntityDataFlagTradeInterest",
	"EntityDataFlagDoorBreaker",
	"EntityDataFlagBreakingObstruction",
	"EntityDataFlagDoorOpener",
	"EntityDataFlagCaptain",
	"EntityDataFlagStunned",
	"EntityDataFlagRoaring",
	"EntityDataFlagDelayedAttack",
	"EntityDataFlagAvoidingMobs",
	"EntityDataFlagAvoidingBlock",
	"EntityDataFlagFacingTargetToRangeAttack",
	"EntityDataFlagHiddenWhenInvisible",
	"EntityDataFlagInUI",
	"EntityDataFlagStalking",
	"EntityDataFlagEmoting",
	"EntityDataFlagCelebrating",
	"EntityDataFlagAdmiring",
	"EntityDataFlagCelebratingSpecial",
	"EntityDataFlagOutOfControl",
	"EntityDataFlagRamAttack",
	"EntityDataFlagPlayingDead",
	"EntityDataFlagInAscendingBlock",
	"EntityDataFlagOverDescendingBlock",
	"EntityDataFlagCroaking",
	"EntityDataFlagDigestMob",
	"EntityDataFlagJumpGoal",
	"EntityDataFlagEmerging",
	"EntityDataFlagSniffing",
	"EntityDataFlagDigging",
	"EntityDataFlagSonicBoom",
	"EntityDataFlagHasDashTimeout",
	"EntityDataFlagPushTowardsClosestSpace",
	"EntityDataFlagScenting",
	"EntityDataFlagRising",
	"EntityDataFlagFeelingHappy",
	"EntityDataFlagSearching",
	"EntityDataFlagCrawling",
	"EntityDataFlagTimerFlag1",
	"EntityDataFlagTimerFlag2",
	"EntityDataFlagTimerFlag3",
}
