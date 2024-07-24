events.register('EntityAdd', (entity, metadata, properties, time) => {
    console.log(`EntityAdd ${entity.EntityType}`);
});


events.register('EntityDataUpdate', (entity, metadata, properties, time) => {
    console.log(`EntityDataUpdate ${entity.EntityType}`);
});


events.register('ChunkAdd', (pos, time) => {
    console.log(`ChunkAdd ${pos}`);
});


events.register('BlockUpdate', (name, properties, time) => {
    console.log(`BlockUpdate ${name}`);
});


events.register('SpawnParticle', (name, x, y, z, time) => {
    console.log(`SpawnParticle ${name}`);
});