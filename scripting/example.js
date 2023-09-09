/**
 * @param {Entity} entity
 * @returns {boolean} ignore entity
 */
function OnEntityAdd(entity) {
    console.log("adding entity " + entity.EntityType);
    return false;
}

/**
 * @param {ChunkPos} pos
 * @returns {boolean} ignore chunk
 */
function OnChunkAdd(pos) {
    return false;
}