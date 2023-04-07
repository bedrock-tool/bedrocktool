package messages

import (
	"image"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type MessageResponse struct {
	Ok   bool
	Data interface{}
}

type UIState int

const (
	UIStateConnect = iota
	UIStateConnecting
	UIStateMain
	UIStateFinished
)

type HandlerFunc = func(data interface{}) MessageResponse

//

type SetUIState = UIState

//

type SetVoidGen struct {
	Value bool
}

//

type SetWorldName struct {
	WorldName string
}

//

type Init struct {
	Handler HandlerFunc
}

//

type UpdateMap struct {
	ChunkCount   int
	Rotation     float32
	UpdatedTiles []protocol.ChunkPos
	Tiles        map[protocol.ChunkPos]*image.RGBA
	BoundsMin    protocol.ChunkPos
	BoundsMax    protocol.ChunkPos
}

//

type NewSkin struct {
	PlayerName string
	Skin       *protocol.Skin
}

type SavingWorld struct {
	Name   string
	Chunks int
}

type CanShowImages struct{}
