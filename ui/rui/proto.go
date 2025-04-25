package rui

import (
	"errors"
	"fmt"
	"image"
	reflect "reflect"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func readPacket(r *protocol.Reader) (packet any, err error) {
	defer func() {
		switch p := recover().(type) {
		case string:
			err = errors.New(p)
		case error:
			err = p
		case nil:
			return
		default:
			panic("invalid panic")
		}
	}()
	var packetType uint8
	r.Uint8(&packetType)
	registered, ok := idPacket[packetType]
	if !ok {
		return nil, fmt.Errorf("unregistered packet type")
	}
	packetValue := reflect.New(registered.t).Addr().Interface()
	registered.marshal(packetValue, r)
	return packetValue, nil
}

func writePacket(w *protocol.Writer, packet any) (err error) {
	defer func() {
		switch p := recover().(type) {
		case string:
			err = errors.New(p)
		case error:
			err = p
		case nil:
			return
		default:
			panic("invalid panic")
		}
	}()
	rv := reflect.ValueOf(packet)
	registered, ok := packetIds[rv.Type().Elem()]
	if !ok {
		return fmt.Errorf("unregistered packet type")
	}
	w.Uint8(&registered.id)
	registered.marshal(packet, w)
	return nil
}

type registeredPacket struct {
	marshal func(v any, r protocol.IO)
	t       reflect.Type
	id      uint8
}

var packetIds = map[reflect.Type]registeredPacket{}
var idPacket = map[uint8]registeredPacket{}

func registerPacket[T any](marshaler func(v *T, i protocol.IO)) {
	var marshalFunc = func(v any, i protocol.IO) { marshaler(v.(*T), i) }
	if marshaler == nil {
		marshalFunc = func(v any, i protocol.IO) {}
	}
	t := reflect.TypeFor[T]()
	id := uint8(len(packetIds))
	rp := registeredPacket{
		marshal: marshalFunc,
		t:       t,
		id:      id,
	}
	packetIds[t] = rp
	idPacket[id] = rp
}

func init() {
	registerPacket[getSubcommandsRequest](nil)
	registerPacket(marshalGetSubcommandsResponse)
	registerPacket(marshalStartSubcommandRequest)
	registerPacket[stopSubcommandRequest](nil)
	registerPacket[requestLoginRequest](nil)

	registerPacket(marshalEventAuthFinished)
	registerPacket(marshalEventConnectStateUpdate)
	registerPacket(marshalEventDisplayAuthCode)
	registerPacket(marshalEventError)
	registerPacket(marshalEventFinishedPack)
	registerPacket(marshalEventFinishedSavingWorld)
	registerPacket(marshalEventInitialPacksInfo)
	registerPacket(marshalEventMapTiles)
	registerPacket(marshalEventPackDownloadProgress)
	registerPacket(marshalEventPlayerPosition)
	registerPacket(marshalEventPlayerSkin)
	registerPacket(marshalEventProcessingPack)
	registerPacket(marshalEventProcessingWorldUpdate)
	registerPacket(marshalEventResetMap)
	registerPacket(marshalEventSetUIState)
	registerPacket(marshalEventSetValue)
	registerPacket(marshalEventUpdateAvailable)
}

type getSubcommandsRequest struct{}

type subcommand struct {
	Name string
	Args []commands.Arg
}

func marshalArg(i protocol.IO, a *commands.Arg) {
	i.String(&a.Name)
	i.String(&a.Desc)
	i.String(&a.Flag)
	protocol.FuncSlice(i, &a.Path, i.String)
	i.String(&a.Default)
}

func (s *subcommand) Marshal(i protocol.IO) {
	i.String(&s.Name)
	protocol.FuncIOSlice(i, &s.Args, marshalArg)
}

type getSubcommandsResponse struct {
	Commands []subcommand
}

func marshalGetSubcommandsResponse(v *getSubcommandsResponse, i protocol.IO) {
	protocol.Slice(i, &v.Commands)
}

type startSubcommandRequest struct {
	Name     string
	Settings []byte
}

func marshalStartSubcommandRequest(v *startSubcommandRequest, i protocol.IO) {
	i.String(&v.Name)
	i.ByteSlice(&v.Settings)
}

type stopSubcommandRequest struct{}

type requestLoginRequest struct{}

func marshalError(i protocol.IO, err *error) {
	var errStr protocol.Optional[string]
	if *err != nil {
		errStr = protocol.Option((*err).Error())
	}
	protocol.OptionalFunc(i, &errStr, i.String)
	if errStr, ok := errStr.Value(); ok {
		*err = errors.New(errStr)
	} else {
		*err = nil
	}
}

func marshalInt(i protocol.IO, v *int) {
	vi := int32(*v)
	i.Int32(&vi)
	*v = int(vi)
}

func marshalImage(i protocol.IO, img *image.RGBA) {
	var dx int32 = int32(img.Rect.Dx())
	var dy int32 = int32(img.Rect.Dy())
	var stride int32 = int32(img.Stride)
	i.Int32(&dx)
	i.Int32(&dy)
	i.Int32(&stride)
	i.ByteSlice(&img.Pix)
	img.Rect = image.Rect(0, 0, int(dx), int(dy))
	img.Stride = int(stride)
}

func marshalEventAuthFinished(v *messages.EventAuthFinished, i protocol.IO) {
	marshalError(i, &v.Error)
}

func marshalEventConnectStateUpdate(v *messages.EventConnectStateUpdate, i protocol.IO) {
	i.Uint8((*uint8)(&v.State))
	i.String(&v.ListenAddr)
}

func marshalEventDisplayAuthCode(v *messages.EventDisplayAuthCode, i protocol.IO) {
	i.String(&v.AuthCode)
	i.String(&v.URI)
}

func marshalEventError(v *messages.EventError, i protocol.IO) {
	marshalError(i, &v.Error)
}

func marshalEventFinishedPack(v *messages.EventFinishedPack, i protocol.IO) {
	i.String(&v.ID)
	i.String(&v.Name)
	i.String(&v.Version)
	i.String(&v.Filepath)
	var iconOpt protocol.Optional[image.RGBA]
	if v.Icon != nil {
		iconOpt = protocol.Option(*v.Icon)
	}
	protocol.OptionalFuncIO(i, &iconOpt, marshalImage)
	if icon, ok := iconOpt.Value(); ok {
		v.Icon = &icon
	}
	marshalError(i, &v.Error)
}

func marshalEventFinishedSavingWorld(v *messages.EventFinishedSavingWorld, i protocol.IO) {
	i.String(&v.WorldName)
	i.String(&v.Filepath)
	marshalInt(i, &v.Chunks)
	marshalInt(i, &v.Entities)
}

func marshalEventInitialPacksInfo(v *messages.EventInitialPacksInfo, i protocol.IO) {
	protocol.Slice(i, &v.Packs)
	i.Bool(&v.KeysOnly)
}

func marshalEventMapTiles(v *messages.EventMapTiles, i protocol.IO) {
	protocol.FuncSlice(i, &v.Tiles, func(t *messages.MapTile) {
		i.ChunkPos(&t.Pos)
		marshalImage(i, &t.Img)
	})
}

func marshalEventPackDownloadProgress(v *messages.EventPackDownloadProgress, i protocol.IO) {
	i.String(&v.ID)
	marshalInt(i, &v.BytesAdded)
}

func marshalEventPlayerPosition(v *messages.EventPlayerPosition, i protocol.IO) {
	i.Vec3(&v.Position)
}

func marshalEventPlayerSkin(v *messages.EventPlayerSkin, i protocol.IO) {
	i.String(&v.PlayerName)
	protocol.Single(i, &v.Skin)
}

func marshalEventProcessingPack(v *messages.EventProcessingPack, i protocol.IO) {
	i.String(&v.ID)
}

func marshalEventProcessingWorldUpdate(v *messages.EventProcessingWorldUpdate, i protocol.IO) {
	i.String(&v.WorldName)
	i.String(&v.State)
}

func marshalEventResetMap(v *messages.EventResetMap, i protocol.IO) {
	// empty
}

func marshalEventSetUIState(v *messages.EventSetUIState, i protocol.IO) {
	i.Uint8((*uint8)(&v.State))
}

func marshalEventSetValue(v *messages.EventSetValue, i protocol.IO) {
	i.String(&v.Name)
	i.String(&v.Value)
}

func marshalEventUpdateAvailable(v *messages.EventUpdateAvailable, i protocol.IO) {
	i.String(&v.Version)
}
