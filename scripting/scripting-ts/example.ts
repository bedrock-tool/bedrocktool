
function OnEntityAdd(entity: Entity, data: EntityMetadata): boolean {
    console.log("adding entity " + entity.EntityType);
    console.log(`NoAI: ${data.Flag(EntityDataKey.Flags, EntityDataFlag.NoAI)}`)
    console.log(`entity name: ${data[EntityDataKey.Name]}`)
    return false;
}

function OnChunkAdd(pos: ChunkPos): boolean {
    return false;
}

function OnEntityDataUpdate(entity: Entity, data: EntityMetadata) {
    console.log("OnEntityDataUpdate");
    console.log("entity name: "+data[EntityDataKey.Name]);
}
