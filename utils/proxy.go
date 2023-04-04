package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/repeale/fp-go"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

var DisconnectReason = "Connection lost"

/*
type dummyProto struct {
	id  int32
	ver string
}

func (p dummyProto) ID() int32            { return p.id }
func (p dummyProto) Ver() string          { return p.ver }
func (p dummyProto) Packets() packet.Pool { return packet.NewPool() }
func (p dummyProto) ConvertToLatest(pk packet.Packet, _ *minecraft.Conn) []packet.Packet {
	return []packet.Packet{pk}
}

func (p dummyProto) ConvertFromLatest(pk packet.Packet, _ *minecraft.Conn) []packet.Packet {
	return []packet.Packet{pk}
}
*/

type (
	PacketFunc    func(header packet.Header, payload []byte, src, dst net.Addr)
	IngameCommand struct {
		Exec func(cmdline []string) bool
		Cmd  protocol.Command
	}
)

type ProxyHandler struct {
	Name     string
	ProxyRef func(*ProxyContext)
	//
	AddressAndName func(address, hostname string) error

	// called to change game data
	GameDataModifier func(*minecraft.GameData)

	// called for every packet
	PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr)

	// called on every packet after login
	PacketCB func(pk packet.Packet, toServer bool, timeReceived time.Time) (packet.Packet, error)

	// called after client connected
	OnClientConnect   func(conn *minecraft.Conn)
	SecondaryClientCB func(conn *minecraft.Conn)

	// called after game started
	ConnectCB func(err error) bool

	// called when the proxy stops
	OnEnd func()
}

type ProxyContext struct {
	Server     *minecraft.Conn
	Client     *minecraft.Conn
	clientAddr net.Addr
	Listener   *minecraft.Listener

	AlwaysGetPacks   bool
	WithClient       bool
	IgnoreDisconnect bool
	CustomClientData *login.ClientData

	commands map[string]IngameCommand
	handlers []*ProxyHandler
}

func NewProxy() (*ProxyContext, error) {
	p := &ProxyContext{
		commands:         make(map[string]IngameCommand),
		WithClient:       true,
		IgnoreDisconnect: false,
	}
	if Options.PathCustomUserData != "" {
		if err := p.LoadCustomUserData(Options.PathCustomUserData); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *ProxyContext) AddCommand(cmd IngameCommand) {
	p.commands[cmd.Cmd.Name] = cmd
}

type CustomClientData struct {
	// skin things
	CapeFilename         string
	SkinFilename         string
	SkinGeometryFilename string
	SkinID               string
	PlayFabID            string
	PersonaSkin          bool
	PremiumSkin          bool
	TrustedSkin          bool
	ArmSize              string
	SkinColour           string

	// misc
	IsEditorMode bool
	LanguageCode string
	DeviceID     string
}

func (p *ProxyContext) LoadCustomUserData(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var customData CustomClientData
	err = json.NewDecoder(f).Decode(&customData)
	if err != nil {
		return err
	}

	p.CustomClientData = &login.ClientData{
		SkinID:      customData.SkinID,
		PlayFabID:   customData.PlayFabID,
		PersonaSkin: customData.PersonaSkin,
		PremiumSkin: customData.PremiumSkin,
		TrustedSkin: customData.TrustedSkin,
		ArmSize:     customData.ArmSize,
		SkinColour:  customData.SkinColour,
	}

	if customData.SkinFilename != "" {
		img, err := loadPng(customData.SkinFilename)
		if err != nil {
			return err
		}
		p.CustomClientData.SkinData = base64.RawStdEncoding.EncodeToString(img.Pix)
		p.CustomClientData.SkinImageWidth = img.Rect.Dx()
		p.CustomClientData.SkinImageHeight = img.Rect.Dy()
	}

	if customData.CapeFilename != "" {
		img, err := loadPng(customData.CapeFilename)
		if err != nil {
			return err
		}
		p.CustomClientData.CapeData = base64.RawStdEncoding.EncodeToString(img.Pix)
		p.CustomClientData.CapeImageWidth = img.Rect.Dx()
		p.CustomClientData.CapeImageHeight = img.Rect.Dy()
	}

	if customData.SkinGeometryFilename != "" {
		data, err := os.ReadFile(customData.SkinGeometryFilename)
		if err != nil {
			return err
		}
		p.CustomClientData.SkinGeometry = base64.RawStdEncoding.EncodeToString(data)
	}

	p.CustomClientData.DeviceID = customData.DeviceID

	return nil
}

func (p *ProxyContext) ClientWritePacket(pk packet.Packet) error {
	if p.Client == nil {
		return nil
	}
	return p.Client.WritePacket(pk)
}

func (p *ProxyContext) SendMessage(text string) {
	p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypeSystem,
		Message:  "§8[§bBedrocktool§8]§r " + text,
	})
}

func (p *ProxyContext) SendPopup(text string) {
	p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

func (p *ProxyContext) AddHandler(handler *ProxyHandler) {
	p.handlers = append(p.handlers, handler)
}

func (p *ProxyContext) CommandHandlerPacketCB(pk packet.Packet, toServer bool, _ time.Time) (packet.Packet, error) {
	switch _pk := pk.(type) {
	case *packet.CommandRequest:
		cmd := strings.Split(_pk.CommandLine, " ")
		name := cmd[0][1:]
		if h, ok := p.commands[name]; ok {
			if h.Exec(cmd[1:]) {
				pk = nil
			}
		}
	case *packet.AvailableCommands:
		cmds := make([]protocol.Command, len(p.commands))
		for _, ic := range p.commands {
			cmds = append(cmds, ic.Cmd)
		}
		pk = &packet.AvailableCommands{
			Constraints: _pk.Constraints,
			Commands:    append(_pk.Commands, cmds...),
		}
	}
	return pk, nil
}

func (p *ProxyContext) proxyLoop(ctx context.Context, toServer bool) error {
	var c1, c2 *minecraft.Conn
	if toServer {
		c1 = p.Client
		c2 = p.Server
	} else {
		c1 = p.Server
		c2 = p.Client
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		pk, err := c1.ReadPacket()
		if err != nil {
			return err
		}

		pkName := reflect.TypeOf(pk).String()
		for _, handler := range p.handlers {
			if handler.PacketCB != nil {
				pk, err = handler.PacketCB(pk, toServer, time.Now())
				if err != nil {
					return err
				}
				if pk == nil {
					logrus.Tracef("Dropped Packet: %s", pkName)
					break
				}
			}
		}

		if pk != nil && c2 != nil {
			if err := c2.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					DisconnectReason = disconnect.Error()
				}
				return err
			}
		}
	}
}

func (p *ProxyContext) IsClient(addr net.Addr) bool {
	return p.clientAddr.String() == addr.String()
}

var NewDebugLogger func(bool) *ProxyHandler

func (p *ProxyContext) Run(ctx context.Context, serverAddress, name string) (err error) {
	if Options.Debug || Options.ExtraDebug {
		p.AddHandler(NewDebugLogger(Options.ExtraDebug))
	}

	for _, handler := range p.handlers {
		if handler.AddressAndName != nil {
			handler.AddressAndName(serverAddress, name)
		}
		if handler.ProxyRef != nil {
			handler.ProxyRef(p)
		}
	}

	if strings.HasPrefix(serverAddress, "PCAP!") {
		return createReplayConnection(ctx, serverAddress[5:], p)
	}

	GetTokenSource() // ask for login before listening

	var cdp *login.ClientData = nil
	if p.WithClient {
		var packs []*resource.Pack
		if Options.Preload {
			logrus.Info(locale.Loc("preloading_packs", nil))
			serverConn, err := connectServer(ctx, serverAddress, nil, true, func(header packet.Header, payload []byte, src, dst net.Addr) {})
			if err != nil {
				return fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": serverAddress, "Err": err}))
			}
			serverConn.Close()
			packs = serverConn.ResourcePacks()
			logrus.Infof(locale.Locm("pack_count_loaded", locale.Strmap{"Count": len(packs)}, len(packs)))
		}

		_status := minecraft.NewStatusProvider("Server")
		p.Listener, err = minecraft.ListenConfig{
			StatusProvider:    _status,
			ResourcePacks:     packs,
			AcceptedProtocols: []minecraft.Protocol{
				//dummyProto{id: 567, ver: "1.19.60"},
			},
		}.Listen("raknet", ":19132")
		if err != nil {
			return err
		}
		defer func() {
			if p.Client != nil {
				p.Listener.Disconnect(p.Client, DisconnectReason)
			}
			p.Listener.Close()
		}()

		logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": p.Listener.Addr()}))
		logrus.Infof(locale.Loc("help_connect", nil))

		c, err := p.Listener.Accept()
		if err != nil {
			return err
		}
		p.Client = c.(*minecraft.Conn)
		cd := p.Client.ClientData()
		cdp = &cd
	}

	for _, handler := range p.handlers {
		if handler.OnClientConnect == nil {
			continue
		}
		handler.OnClientConnect(p.Client)
	}

	if p.CustomClientData != nil {
		cdp = p.CustomClientData
	}

	p.Server, err = connectServer(ctx, serverAddress, cdp, p.AlwaysGetPacks, func(header packet.Header, payload []byte, src, dst net.Addr) {
		if header.PacketID == packet.IDRequestNetworkSettings {
			p.clientAddr = src
		}
		for _, handler := range p.handlers {
			if handler.PacketFunc == nil {
				continue
			}
			handler.PacketFunc(header, payload, src, dst)
		}
	})
	if err != nil {
		for _, handler := range p.handlers {
			if handler.ConnectCB == nil {
				continue
			}
			ignore := handler.ConnectCB(err)
			if ignore {
				err = nil
				break
			}
		}

		if err != nil {
			err = fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": serverAddress, "Err": err}))
		}
		return err
	}
	defer p.Server.Close()

	gd := p.Server.GameData()
	for _, handler := range p.handlers {
		if handler.GameDataModifier != nil {
			handler.GameDataModifier(&gd)
		}
	}

	// spawn and start the game
	if err = spawnConn(ctx, p.Client, p.Server, gd); err != nil {
		err = fmt.Errorf(locale.Loc("failed_to_spawn", locale.Strmap{"Err": err}))
		return err
	}

	for _, handler := range p.handlers {
		if handler.ConnectCB == nil {
			continue
		}
		if !handler.ConnectCB(nil) {
			return errors.New("Cancelled")
		}
	}

	// append self to handlers for commands
	p.handlers = append(p.handlers, &ProxyHandler{
		Name:     "Commands",
		PacketCB: p.CommandHandlerPacketCB,
	})

	wg := sync.WaitGroup{}
	// server to client
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.proxyLoop(ctx, false); err != nil {
			logrus.Error(err)
			return
		}
	}()

	// client to server
	if p.Client != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.proxyLoop(ctx, true); err != nil {
				logrus.Error(err)
				return
			}
		}()
	}

	wantSecondary := fp.Filter(func(handler *ProxyHandler) bool {
		return handler.SecondaryClientCB != nil
	})(p.handlers)

	if len(wantSecondary) > 0 {
		go func() {
			for {
				c, err := p.Listener.Accept()
				if err != nil {
					logrus.Error(err)
					return
				}

				for _, handler := range wantSecondary {
					go handler.SecondaryClientCB(c.(*minecraft.Conn))
				}
			}
		}()
	}

	wg.Wait()

	for _, handler := range p.handlers {
		if handler.OnEnd != nil {
			handler.OnEnd()
		}
	}
	return err
}
