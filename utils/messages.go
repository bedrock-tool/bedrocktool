package utils

import (
	"image"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type HandlerFunc func(name string, data interface{}) MessageResponse

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

var InitMapName = "init_map"

type InitMapPayload struct {
	RLock     func()
	RUnlock   func()
	GetTiles  func() map[protocol.ChunkPos]*image.RGBA
	GetBounds func() (min, max protocol.ChunkPos)
}

var UpdateMapName = "update_map"

type UpdateMapPayload struct {
	ChunkCount int
}
