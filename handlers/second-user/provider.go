package seconduser

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
)

type provider struct {
	s *secondaryUser
}

func (p *provider) Settings() *world.Settings {
	return &world.Settings{
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

func (p *provider) LoadColumn(pos world.ChunkPos, dim world.Dimension) (*world.Column, error) {
	return &world.Column{
		Chunk: p.s.chunks[pos],
	}, nil
}

func (p *provider) StoreColumn(pos world.ChunkPos, dim world.Dimension, col *world.Column) error {
	return nil
}
