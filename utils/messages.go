package utils

import (
	"image"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type UIState = int

const (
	UIStateConnect = iota
	UIStateConnecting
	UIStateMain
)

type HandlerFunc = func(name string, data interface{}) MessageResponse

var SetUIStateName = "set_ui_state"

var SetVoidGenName = "set_void_gen"

type SetVoidGenPayload struct {
	Value bool
}

var SetWorldNameName = "set_world_name"

type SetWorldNamePayload struct {
	WorldName string
}

var InitName = "init"

type InitPayload struct {
	Handler HandlerFunc
}

var UpdateMapName = "update_map"

type UpdateMapPayload struct {
	ChunkCount   int
	UpdatedTiles []protocol.ChunkPos
	Tiles        map[protocol.ChunkPos]*image.RGBA
	BoundsMin    protocol.ChunkPos
	BoundsMax    protocol.ChunkPos
}
