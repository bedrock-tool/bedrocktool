events.register('EntityAdd', (entity, metadata, properties, time) => {
    console.log(`EntityAdd ${entity.EntityType}`);
});


events.register('EntityDataUpdate', (entity, metadata, properties, time) => {
    console.log(`EntityDataUpdate ${entity.EntityType}`);
});


events.register('ChunkAdd', (pos, time) => {
    console.log(`ChunkAdd ${pos}`);
});


events.register('BlockUpdate', (name, properties, pos, time) => {
    console.log(`BlockUpdate ${name}`);
});


events.register('SpawnParticle', (name, pos, time) => {
    console.log(`SpawnParticle ${name}`);
});

events.register('Packet', (name, packet, toServer, time) => {
    if(name === 'LevelSoundEvent') {
        console.log(`Packet ${name} ${JSON.stringify(packet)}`);
    }
});