package messages

import (
	"image"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type MessageResponse struct {
	Ok   bool
	Data interface{}
}

type UIState = int

const (
	UIStateConnect = iota
	UIStateConnecting
	UIStateMain
	UIStateFinished
)

type HandlerFunc = func(name string, data interface{}) MessageResponse

//

const SetUIState = "set_ui_state"

type SetUIStatePayload = UIState

//

const SetVoidGen = "set_void_gen"

type SetVoidGenPayload struct {
	Value bool
}

//

const SetWorldName = "set_world_name"

type SetWorldNamePayload struct {
	WorldName string
}

//

var Init = "init"

type InitPayload struct {
	Handler HandlerFunc
}

//

var UpdateMap = "update_map"

type UpdateMapPayload struct {
	ChunkCount   int
	Rotation     float32
	UpdatedTiles []protocol.ChunkPos
	Tiles        map[protocol.ChunkPos]*image.RGBA
	BoundsMin    protocol.ChunkPos
	BoundsMax    protocol.ChunkPos
}

//

var NewSkin = "new_skin"

type NewSkinPayload struct {
	PlayerName string
	Skin       *protocol.Skin
}

var SavingWorld = "saving_world"

type SavingWorldPayload struct {
	Name   string
	Chunks int
}
