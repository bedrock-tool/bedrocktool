events.register('EntityAdd', (entity, metadata, time) => {
    console.log(`EntityAdd ${entity.EntityType}`);
});


events.register('EntityUpdate', (update, time) => {
    console.log(`EntityUpdate ${update.Entity.EntityType}`);
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

// return true to save a player, false to leave it out,
// or nothing to use the usual rules
events.register('PlayerAdd', (player, metadata, time) => {
    console.log(`PlayerAdd ${player.Username} npc=${player.IsNPC}`);
    if (player.Username.startsWith('§')) {
        return true;
    }
});
