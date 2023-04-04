package seconduser

import (
	"sync"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/google/uuid"
)

type provider struct {
	s *secondaryUser
}

func (p *provider) Settings() *world.Settings {
	return &world.Settings{
		Mutex:           sync.Mutex{},
		Name:            "world",
		Spawn:           cube.Pos{0, 0, 0},
		DefaultGameMode: world.GameModeCreative,
		Difficulty:      world.DifficultyNormal,
	}
}

func (p *provider) SaveSettings(*world.Settings) {
}

func (p *provider) Close() error {
	return nil
}

func (p *provider) LoadPlayerSpawnPosition(uuid uuid.UUID) (pos cube.Pos, exists bool, err error) {
	return cube.Pos{0, 0, 0}, false, nil
}

func (p *provider) SavePlayerSpawnPosition(uuid uuid.UUID, pos cube.Pos) error {
	return nil
}

func (p *provider) LoadChunk(position world.ChunkPos, dim world.Dimension) (c *chunk.Chunk, exists bool, err error) {
	c, ok := p.s.chunks[position]
	return c, ok, nil
}

func (p *provider) SaveChunk(position world.ChunkPos, c *chunk.Chunk, dim world.Dimension) error {
	return nil
}

func (p *provider) LoadEntities(position world.ChunkPos, dim world.Dimension, reg world.EntityRegistry) ([]world.Entity, error) {
	return nil, nil
}

func (p *provider) SaveEntities(position world.ChunkPos, entities []world.Entity, dim world.Dimension) error {
	return nil
}

func (p *provider) LoadBlockNBT(position world.ChunkPos, dim world.Dimension) ([]map[string]any, error) {
	return nil, nil
}

func (p *provider) SaveBlockNBT(position world.ChunkPos, data []map[string]any, dim world.Dimension) error {
	return nil
}
