declare const console: {
    log(data: any);
};

declare type WindowID = number;
declare type Slot = number;

declare type Entity = {
    RuntimeID: number
    UniqueID: number
    EntityType: string

    Position: [number, number, number];
    Pitch: number;
    Yaw: number;
    Velocity: [number, number, number];
    Metadata: {[k: number]: any}
    Inventory: {[k: WindowID]: {[k: Slot]: ItemInstance}}
}

declare type ItemInstance = {
    StackNetworkID: number;
    Stack: ItemStack;
}

declare type ItemStack = {
    NetworkID: number;
    MetadataValue: number;
    BlockRuntimeID: number;
    Count: number;
    NBTData: {[k: string]: any};
    CanBePlacedOn: Array<string>;
    CanBreak: Array<string>;
    HasNetworkID: boolean;
}

declare type ChunkPos = [number, number];
