package messages

import (
	"image"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Message struct {
	Source     string
	SourceType string
	Data       any
	Ok         bool
}

type UIState int

const (
	UIStateInvalid UIState = iota
	UIStateMain
	UIStateFinished
)

type HandlerFunc = func(msg *Message) *Message

type Handler interface {
	HandleMessage(msg *Message) *Message
}

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

type MapLookup struct {
	Lookup *image.RGBA
}

type UpdateMap struct {
	Chunks        map[protocol.ChunkPos]*image.RGBA
	UpdatedChunks []protocol.ChunkPos
	ChunkCount    int
	Rotation      float32
}

type PlayerPosition struct {
	Position mgl32.Vec3
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

// close self
type Close struct{}

type ShowPopup struct {
	Popup any
}

type StartSubcommand struct {
	Command any
}

type ExitSubcommand struct{}

type HaveFinishScreen struct{}

type Error error
