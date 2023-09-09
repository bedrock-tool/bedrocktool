/**
 * @param {Entity} entity
 * @param {EntityMetadata} data
 * @returns {boolean} ignore entity
 */
function OnEntityAdd(entity, data) {
    console.log("adding entity " + entity.EntityType);
    console.log(`NoAI: ${data.Flag(EntityDataKey.Flags, EntityDataFlag.NoAI)}`)
    console.log(`entity name: ${data[EntityDataKey.Name]}`)
    return false;
}

/**
 * @param {ChunkPos} pos
 * @returns {boolean} ignore chunk
 */
function OnChunkAdd(pos) {
    return false;
}

/**
 * @param {Entity} entity
 * @param {EntityMetadata} data
*/
function OnEntityDataUpdate(entity, data) {
    console.log("OnEntityDataUpdate");
    console.log("entity name: "+data[EntityDataKey.Name]);
}