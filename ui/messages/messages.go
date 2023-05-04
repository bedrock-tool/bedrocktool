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
	World *SavedWorld
}

type SavedWorld struct {
	Name   string
	Path   string
	Chunks int
	Image  image.Image
}

type CanShowImages struct{}

type InitialPacksInfo struct {
	Packs []protocol.TexturePackInfo
}

type PackDownloadProgress struct {
	UUID      string
	LoadedAdd uint64
}

type DownloadedPack struct {
	UUID string
	Name string
	Path string
	Size int
	Icon image.Image
	Err  error
}

type FinishedDownloadingPacks struct {
	Packs []*DownloadedPack
}

type UpdateAvailable struct {
	Version string
}
