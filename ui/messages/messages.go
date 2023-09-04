package messages

import (
	"image"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
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

type ConnectState int

const (
	ConnectStateBegin ConnectState = iota + 1
	ConnectStateListening
	ConnectStateServerConnecting
	ConnectStateReceivingResources
	ConnectStateEstablished
	ConnectStateDone
)

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
	ChunkCount    int
	Rotation      float32
	UpdatedChunks []protocol.ChunkPos
	Chunks        map[protocol.ChunkPos]*image.RGBA
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
	Name     string
	Path     string
	Chunks   int
	Entities int
	Image    image.Image
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
	Path string
	Err  error
}

type FinishedPack struct {
	Pack *resource.Pack
}

type ProcessingPack struct {
	ID         string
	Processing bool
	Path       string
	Err        error
}

type UpdateAvailable struct {
	Version string
}
