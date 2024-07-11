package messages

import (
	"encoding/json"
	"errors"
	"image"
	"reflect"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Message struct {
	Source string
	Target string
	Data   any
}

type HandlerFunc = func(msg *Message) *Message

type Handler interface {
	HandleMessage(msg *Message) *Message
}

type UIState int

const (
	UIStateInvalid UIState = iota
	UIStateMain
	UIStateFinished
)

type ConnectState int

const (
	ConnectStateBegin ConnectState = iota + 1
	ConnectStateListening
	ConnectStateServerConnecting
	ConnectStateReceivingResources
	ConnectStateEstablished
	ConnectStateDone
)

type ConnectStateUpdate struct {
	ListenIP   string
	ListenPort int
	State      ConnectState
}

type SetValue struct {
	Name  string
	Value string
}

type Features struct {
	Request  bool
	Features []string
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

type InitialPacksInfo struct {
	Packs    []protocol.TexturePackInfo
	OnlyKeys bool
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
	Pack resource.Pack
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
type Close struct {
	Type string
	ID   string
}

type ShowPopup struct {
	Popup any
}

type StartSubcommand struct {
	Command any
}

type ExitSubcommand struct{}

type HaveFinishScreen struct{}

type Error error

type RequestLogin struct {
	Wait bool
}

type ProcessingWorldUpdate struct {
	Name  string
	State string
}

type FinishedSavingWorld struct {
	World *SavedWorld
}

type SavedWorld struct {
	Name     string
	Path     string
	Chunks   int
	Entities int
	Image    image.Image
}

type ServerInput struct {
	Request  bool // if this is a request for input
	IsReplay bool
	Address  string
	Port     string
	Name     string
}

func Decode(bytes []byte) (*Message, error) {
	var dec struct {
		Source string
		Target string
		Type   string
		Data   json.RawMessage
	}
	err := json.Unmarshal(bytes, &dec)
	if err != nil {
		return nil, err
	}

	var data any
	switch dec.Type {
	case "SetValue":
		data = &SetValue{}
	case "ServerInput":
		data = &ServerInput{}
	case "Error":
		data = errors.New("")
	case "Features":
		data = &Features{}
	default:
		panic("unknown message type")
	}

	return &Message{
		Source: dec.Source,
		Target: dec.Target,
		Data:   data,
	}, nil
}

func Encode(msg *Message) []byte {
	msgType := reflect.TypeOf(msg.Data).String()

	var enc = struct {
		Source string
		Target string
		Type   string
		Data   any
	}{
		Source: msg.Source,
		Target: msg.Target,
		Type:   msgType,
		Data:   msg.Data,
	}

	data, err := json.Marshal(&enc)
	if err != nil {
		panic(err)
	}
	return data
}
