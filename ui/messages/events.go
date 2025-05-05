package messages

import (
	"image"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

type UIState uint8

const (
	UIStateMain UIState = iota + 1
	UIStateFinished
)

type ConnectState uint8

const (
	ConnectStateBegin ConnectState = iota + 1
	ConnectStateListening
	ConnectStateServerConnecting
	ConnectStateReceivingResources
	ConnectStateEstablished
	ConnectStateDone
)

type EventSetValue struct {
	Name  string
	Value string
}

type EventSetUIState struct {
	State UIState
}

type EventConnectStateUpdate struct {
	State      ConnectState
	ListenAddr string
}

type EventUpdateAvailable struct {
	Version string
}

type EventUpdateDownloadProgress struct {
	Progress int
}

type EventUpdateDoInstall struct {
	Filepath string
}

type EventError struct {
	Error error
}

//

type EventProcessingWorldUpdate struct {
	WorldName string
	State     string
}

type EventFinishedSavingWorld struct {
	WorldName string
	Filepath  string
	Chunks    int
	Entities  int
}

//

type EventInitialPacksInfo struct {
	Packs    []protocol.TexturePackInfo
	KeysOnly bool
}

type EventProcessingPack struct {
	ID string
}

type EventPackDownloadProgress struct {
	ID         string
	BytesAdded int
}

type EventFinishedPack struct {
	ID       string
	Name     string
	Version  string
	Filepath string
	Icon     *image.RGBA
	Error    error
}

type EventDisplayAuthCode struct {
	AuthCode string
	URI      string
}

type EventAuthFinished struct {
	Error error
}

type MapTile struct {
	Pos protocol.ChunkPos
	Img image.RGBA
}

type EventMapTiles struct {
	Tiles []MapTile
}

type EventResetMap struct{}

type EventPlayerPosition struct {
	Position mgl32.Vec3
}

type EventPlayerSkin struct {
	PlayerName string
	Skin       protocol.Skin
}

//

type EventHandler interface {
	HandleEvent(event any) error
}

var eventHandler func(event any) error

func SetEventHandler(f func(event any) error) {
	eventHandler = f
}

func SendEvent(event any) {
	//fmt.Printf("event %s\n", reflect.TypeOf(event).String())
	err := eventHandler(event)
	if err != nil {
		logrus.Errorf("event handler errored %s", err)
	}
}

type AuthHandler struct{}

func (a *AuthHandler) AuthCode(uri, code string) {
	SendEvent(&EventDisplayAuthCode{
		URI:      uri,
		AuthCode: code,
	})
}

func (a *AuthHandler) Finished(err error) {
	SendEvent(&EventAuthFinished{
		Error: err,
	})
}
