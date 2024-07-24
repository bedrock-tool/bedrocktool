declare const console: {
    log(data: any);
};


/**
 * Names of events that can be registered.
 */
declare type EventNames = 'EntityAdd' | 'EntityDataUpdate' | 'ChunkAdd' | 'BlockUpdate' | 'SpawnParticle';


/**
 * Callback for the `EntityAdd` event.
 * 
 * @param entity - The entity being added.
 * @param metadata - Metadata associated with the entity.
 * @param properties - Properties of the entity.
 * @param time - The time the entity was added.
 */
declare type EntityAddCallback = (entity: Entity, metadata: EntityMetadata, properties: {[k: string]: EntityProperty}, time: number) => void;


/**
 * Callback for the `EntityDataUpdate` event.
 * 
 * @param entity - The entity being updated.
 * @param metadata - Metadata associated with the entity.
 * @param properties - Properties of the entity.
 * @param time - The time the entity's data was updated.
 */
declare type EntityDataUpdateCallback = (entity: Entity, metadata: EntityMetadata, properties: {[k: string]: EntityProperty}, time: number) => void;


/**
 * Callback for the `ChunkAdd` event.
 * 
 * @param pos - The position of the chunk.
 * @param time - The time the chunk was added.
 */
declare type ChunkAddCallback = (pos: [number, number, number], time: number) => void;


/**
 * Callback for the `BlockUpdate` event.
 * 
 * @param name - The name of the block.
 * @param properties - Properties of the block.
 * @param time - The time the block was updated.
 */
declare type BlockUpdateCallback = (name: string, properties: {[k: string]: any}, pos: [number, number, number], time: number) => void;


/**
 * Callback for the `SpawnParticle` event.
 * 
 * @param name - The name of the particle.
 * @param x - The x-coordinate of the particle.
 * @param y - The y-coordinate of the particle.
 * @param z - The z-coordinate of the particle.
 * @param time - The time the particle was spawned.
 */
declare type SpawnParticleCallback = (name: string, pos: [number, number, number], time: number) => void;

/**
 * Callback for the `Packet` event.
 * 
 * @param name - The name of the particle.
 * @param packet - The packet data.
 * @param time - The time the particle was spawned.
 */
declare type PacketCallback = (name: string, packet: any, time: number) => void;




declare const events: {
    /**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'EntityAdd':
     *     - `entity` - The entity being added.
     *     - `metadata` - Metadata associated with the entity.
     *     - `properties` - Properties of the entity.
     *     - `time` - The time the entity was added.
     * 
     * @example
     * events.register('EntityAdd', (entity: Entity, metadata: EntityMetadata, properties, time) => {
     *     console.log(`EntityAdd ${entity.EntityType}`);
     * });
     * 
     */
    register(name: 'EntityAdd', callback: EntityAddCallback): void;
    /**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'EntityDataUpdate':
     *     - `entity` - The entity being updated.
     *     - `metadata` - Metadata associated with the entity.
     *     - `properties` - Properties of the entity.
     *     - `time` - The time the entity's data was updated.
     * 
     * @example
     * events.register('EntityDataUpdate', (entity, metadata, properties, time) => {
     *     console.log(`EntityDataUpdate ${entity.EntityType}`);
     * });
     * 
     */
    register(name: 'EntityDataUpdate', callback: EntityDataUpdateCallback): void;
    /**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'ChunkAdd':
     *     - `pos` - The position of the chunk.
     *     - `time` - The time the chunk was added.
     * 
     * @example
     * events.register('ChunkAdd', (pos, time) => {
     *     console.log(`ChunkAdd ${pos}`);
     * });
     * 
     */
    register(name: 'ChunkAdd', callback: ChunkAddCallback): void;
    /**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'BlockUpdate':
     *     - `name` - The name of the block.
     *     - `properties` - Properties of the block.
     *     - `time` - The time the block was updated.
     * 
     * @example
     * events.register('BlockUpdate', (name, properties, time) => {
     *     console.log(`BlockUpdate ${name}`);
     * });
     * 
     */
    register(name: 'BlockUpdate', callback: BlockUpdateCallback): void;
    /**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'SpawnParticle':
     *     - `name` - The name of the particle.
     *     - `x` - The x-coordinate of the particle.
     *     - `y` - The y-coordinate of the particle.
     *     - `z` - The z-coordinate of the particle.
     *     - `time` - The time the particle was spawned.
     * 
     * @example
     * events.register('SpawnParticle', (name, x, y, z, time) => {
     *     console.log(`SpawnParticle ${name}`);
     * });
     * 
     */
    register(name: 'SpawnParticle', callback: SpawnParticleCallback): void;
	/**
     * Registers a callback function to be executed when a specified event occurs.
     * 
     * @param name - The name of the event to register. Possible values are:
     *   - 'EntityAdd': Triggered when a new entity is added.
     *   - 'EntityDataUpdate': Triggered when an entity's data is updated.
     *   - 'ChunkAdd': Triggered when a new chunk is added.
     *   - 'BlockUpdate': Triggered when a block is updated.
     *   - 'SpawnParticle': Triggered when a particle is spawned.
	 *   - 'Packet': Triggered when a packet is received.
     * 
     * @param callback - The callback function to invoke when the event occurs. The parameters of the callback function vary based on the event:
     *   - For 'Packet':
     *     - `name` - The name of the packet.
	 *     - `packet` the packet data
     *     - `time` - The time the packet was received.
     * 
     * @example
     * events.register('Packet', (name, packet, time) => {
     *     console.log(`Packet ${name}`);
     * });
     * 
     */
    register(name: 'Packet', callback: PacketCallback): void;
};


/**
 * Represents an entity in the world.
 */
declare type Entity = {
    /**
     * The runtime identifier of the entity.
     */
    RuntimeID: number
    /**
     * A unique identifier of the entity.
     */
    UniqueID: number
    /**
     * The type identifier of the entity, e.g., 'minecraft:sheep'.
     */
    EntityType: string
    /**
     * The current location of the entity, represented as an array containing x, y, and z coordinates.
     */
    Position: [number, number, number];
    /**
     * The current pitch angle of the entity.
     */
    Pitch: number;
    /**
     * The current yaw angle of the entity.
     */
    Yaw: number;
    /**
     * The current velocity of the entity, represented as an array containing x, y, and z coordinates.
     */
    Velocity: [number, number, number];
}


/**
 * Represents an item instance in the game.
 */
declare type ItemInstance = {
    /**
     * The network identifier of the item.
     */
    StackNetworkID: number;
    /**
     * The item stack associated with this instance.
     */
    Stack: ItemStack;
}


/**
 * Represents an item stack in the game.
 */
declare type ItemStack = {
    /**
     * The network identifier of the item.
     */
    NetworkID: number;
    /**
     * The metadata value of the item.
     */
    MetadataValue: number;
    /**
     * The runtime identifier of the block, if the item stack represents a block.
     */
    BlockRuntimeID: number;
    /**
     * The quantity of items in the stack.
     */
    Count: number;
    /**
     * The NBT data associated with the item stack.
     */
    NBTData: {[k: string]: any};
    /**
     * A list of block identifiers that this item can be placed on.
     */
    CanBePlacedOn: Array<string>;
    /**
     * A list of block identifiers that this item can break.
     */
    CanBreak: Array<string>;
    /**
     * Indicates whether the item stack has a network identifier.
     */
    HasNetworkID: boolean;
}


declare type EntityMetadata = {
    [k: EntityDataKey]: any;
    SetFlag: (key: EntityDataKey, index: EntityDataFlag) => void;
    Flag: (key: EntityDataKey, index: EntityDataFlag) => boolean;
};


declare enum EntityDataKey {
	Flags,
	StructuralIntegrity,
	Variant,
	ColorIndex,
	Name,
	Owner,
	Target,
	AirSupply,
	EffectColor,
	EffectAmbience,
	JumpDuration,
	Hurt,
	HurtDirection,
	RowTimeLeft,
	RowTimeRight,
	Value,
	DisplayTileRuntimeID,
	DisplayOffset,
	CustomDisplay,
	Swell,
	OldSwell,
	SwellDirection,
	ChargeAmount,
	CarryBlockRuntimeID,
	ClientEvent,
	UsingItem,
	PlayerFlags,
	PlayerIndex,
	BedPosition,
	PowerX,
	PowerY,
	PowerZ,
	AuxPower,
	FishX,
	FishZ,
	FishAngle,
	AuxValueData,
	LeashHolder,
	Scale,
	HasNPC,
	NPCData,
	Actions,
	AirSupplyMax,
	MarkVariant,
	ContainerType,
	ContainerSize,
	ContainerStrengthModifier,
	BlockTarget,
	Inventory,
	TargetA,
	TargetB,
	TargetC,
	AerialAttack,
	Width,
	Height,
	FuseTime,
	SeatOffset,
	SeatLockPassengerRotation,
	SeatLockPassengerRotationDegrees,
	SeatRotationOffset,
	SeatRotationOffstDegrees,
	DataRadius,
	DataWaiting,
	DataParticle,
	PeekID,
	AttachFace,
	Attached,
	AttachedPosition,
	TradeTarget,
	Career,
	HasCommandBlock,
	CommandName,
	LastCommandOutput,
	TrackCommandOutput,
	ControllingSeatIndex,
	Strength,
	StrengthMax,
	DataSpellCastingColor,
	DataLifetimeTicks,
	PoseIndex,
	DataTickOffset,
	AlwaysShowNameTag,
	ColorTwoIndex,
	NameAuthor,
	Score,
	BalloonAnchor,
	PuffedState,
	BubbleTime,
	Agent,
	SittingAmount,
	SittingAmountPrevious,
	EatingCounter,
	FlagsTwo,
	LayingAmount,
	LayingAmountPrevious,
	DataDuration,
	DataSpawnTime,
	DataChangeRate,
	DataChangeOnPickup,
	DataPickupCount,
	InteractText,
	TradeTier,
	MaxTradeTier,
	TradeExperience,
	SkinID,
	SpawningFrames,
	CommandBlockTickDelay,
	CommandBlockExecuteOnFirstTick,
	AmbientSoundInterval,
	AmbientSoundIntervalRange,
	AmbientSoundEventName,
	FallDamageMultiplier,
	NameRawText,
	CanRideTarget,
	LowTierCuredTradeDiscount,
	HighTierCuredTradeDiscount,
	NearbyCuredTradeDiscount,
	NearbyCuredDiscountTimeStamp,
	HitBox,
	IsBuoyant,
	FreezingEffectStrength,
	BuoyancyData,
	GoatHornCount,
	BaseRuntimeID,
	MovementSoundDistanceOffset,
	HeartbeatIntervalTicks,
	HeartbeatSoundEvent,
	PlayerLastDeathPosition,
	PlayerLastDeathDimension,
	PlayerHasDied,
	CollisionBox,
};


declare enum EntityDataFlag {
    OnFire,
	Sneaking,
	Riding,
	Sprinting,
	UsingItem,
	Invisible,
	Tempted,
	InLove,
	Saddled,
	Powered,
	Ignited,
	Baby,
	Converting,
	Critical,
	ShowName,
	AlwaysShowName,
	NoAI,
	Silent,
	WallClimbing,
	Climb,
	Swim,
	Fly,
	Walk,
	Resting,
	Sitting,
	Angry,
	Interested,
	Charged,
	Tamed,
	Orphaned,
	Leashed,
	Sheared,
	Gliding,
	Elder,
	Moving,
	Breathing,
	Chested,
	Stackable,
	ShowBottom,
	Standing,
	Shaking,
	Idling,
	Casting,
	Charging,
	KeyboardControlled,
	PowerJump,
	Dash,
	Lingering,
	HasCollision,
	HasGravity,
	FireImmune,
	Dancing,
	Enchanted,
	ReturnTrident,
	ContainerPrivate,
	Transforming,
	DamageNearbyMobs,
	Swimming,
	Bribed,
	Pregnant,
	LayingEgg,
	PassengerCanPick,
	TransitionSitting,
	Eating,
	LayingDown,
	Sneezing,
	Trusting,
	Rolling,
	Scared,
	InScaffolding,
	OverScaffolding,
	DescendThroughBlock,
	Blocking,
	TransitionBlocking,
	BlockedUsingShield,
	BlockedUsingDamagedShield,
	Sleeping,
	WantsToWake,
	TradeInterest,
	DoorBreaker,
	BreakingObstruction,
	DoorOpener,
	Captain,
	Stunned,
	Roaring,
	DelayedAttack,
	AvoidingMobs,
	AvoidingBlock,
	FacingTargetToRangeAttack,
	HiddenWhenInvisible,
	InUI,
	Stalking,
	Emoting,
	Celebrating,
	Admiring,
	CelebratingSpecial,
	OutOfControl,
	RamAttack,
	PlayingDead,
	InAscendingBlock,
	OverDescendingBlock,
	Croaking,
	DigestMob,
	JumpGoal,
	Emerging,
	Sniffing,
	Digging,
	SonicBoom,
	HasDashTimeout,
	PushTowardsClosestSpace,
	Scenting,
	Rising,
	FeelingHappy,
	Searching,
	Crawling,
}