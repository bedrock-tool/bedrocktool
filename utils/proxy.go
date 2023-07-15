package utils

import (
	"bytes"
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
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
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
	ProxyRef func(pc *ProxyContext)
	//
	AddressAndName func(address, hostname string) error

	// called to change game data
	GameDataModifier func(gd *minecraft.GameData)

	// called for every packet
	PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr)

	// called on every packet after login
	PacketCB func(pk packet.Packet, toServer bool, timeReceived time.Time) (packet.Packet, error)

	// called after client connected
	OnClientConnect   func(conn minecraft.IConn)
	SecondaryClientCB func(conn minecraft.IConn)

	// called after server connected & downloaded resource packs
	OnServerConnect func() (cancel bool)
	// called after game started
	ConnectCB func(err error) bool

	// called when the proxy stops
	OnEnd func()
}

type ProxyContext struct {
	Server     minecraft.IConn
	Client     minecraft.IConn
	clientAddr net.Addr
	Listener   *minecraft.Listener

	AlwaysGetPacks   bool
	WithClient       bool
	IgnoreDisconnect bool
	CustomClientData *login.ClientData

	commands map[string]IngameCommand
	handlers []*ProxyHandler

	reconnectHandler *ProxyHandler
}

func NewProxy() (*ProxyContext, error) {
	p := &ProxyContext{
		commands:         make(map[string]IngameCommand),
		AlwaysGetPacks:   false,
		WithClient:       true,
		IgnoreDisconnect: false,
		reconnectHandler: NewTransferHandler(),
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
	switch pk := pk.(type) {
	case *packet.CommandRequest:
		cmd := strings.Split(pk.CommandLine, " ")
		name := cmd[0][1:]
		if h, ok := p.commands[name]; ok {
			if h.Exec(cmd[1:]) {
				pk = nil
			}
		}
	case *packet.AvailableCommands:
		cmds := make([]protocol.Command, 0, len(p.commands))
		for _, ic := range p.commands {
			cmds = append(cmds, ic.Cmd)
		}
		pk = &packet.AvailableCommands{
			Constraints: pk.Constraints,
			Commands:    append(pk.Commands, cmds...),
		}
	}
	return pk, nil
}

func (p *ProxyContext) proxyLoop(ctx context.Context, toServer bool) error {
	var c1, c2 minecraft.IConn
	if toServer {
		c1 = p.Client
		c2 = p.Server
	} else {
		c1 = p.Server
		c2 = p.Client
	}

	var transferingErr error = nil

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
					if errors.Is(err, transferingErr) {
						transferingErr = err
						err = nil
					} else {
						return err
					}
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

		if transferingErr != nil {
			return transferingErr
		}
	}
}

// Disconnect disconnects both the client and server
func (p *ProxyContext) Disconnect() {
	p.DisconnectClient()
	p.DisconnectServer()
}

// Disconnect disconnects the client
func (p *ProxyContext) DisconnectClient() {
	if p.Client == nil {
		return
	}
	p.Client.Close()
}

// Disconnect disconnects from the server
func (p *ProxyContext) DisconnectServer() {
	if p.Server == nil {
		return
	}
	p.Server.Close()
}

func (p *ProxyContext) IsClient(addr net.Addr) bool {
	return p.clientAddr.String() == addr.String()
}

var NewDebugLogger func(bool) *ProxyHandler
var NewPacketCapturer func() *ProxyHandler

func (p *ProxyContext) connectClient(ctx context.Context, serverAddress string, cdpp **login.ClientData, tokenSource oauth2.TokenSource) (err error) {
	var packs []*resource.Pack
	if Options.Preload {
		logrus.Info(locale.Loc("preloading_packs", nil))
		serverConn, err := connectServer(ctx, serverAddress, nil, true, nil, tokenSource)
		if err != nil {
			return fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": serverAddress, "Err": err}))
		}
		serverConn.Close()
		packs = serverConn.ResourcePacks()
		logrus.Infof(locale.Locm("pack_count_loaded", locale.Strmap{"Count": len(packs)}, len(packs)))
	}

	status := minecraft.NewStatusProvider(fmt.Sprintf("%s Proxy", serverAddress))
	p.Listener, err = minecraft.ListenConfig{
		StatusProvider:    status,
		ResourcePacks:     packs,
		AcceptedProtocols: []minecraft.Protocol{
			//dummyProto{id: 567, ver: "1.19.60"},
		},
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}

	logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": p.Listener.Addr()}))
	logrus.Infof(locale.Loc("help_connect", nil))

	go func() {
		<-ctx.Done()
		p.Listener.Close()
	}()

	c, err := p.Listener.Accept()
	if err != nil {
		return err
	}
	p.Client = c.(*minecraft.Conn)
	cd := p.Client.ClientData()
	*cdpp = &cd
	return nil
}

func (p *ProxyContext) packetFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	if header.PacketID == packet.IDRequestNetworkSettings {
		p.clientAddr = src
	}
	for _, handler := range p.handlers {
		if handler.PacketFunc == nil {
			continue
		}
		handler.PacketFunc(header, payload, src, dst)
	}
}

func (p *ProxyContext) connect(ctx context.Context, serverAddress string) (err error) {
	defer func() {
		for _, handler := range p.handlers {
			if handler.OnEnd != nil {
				handler.OnEnd()
			}
		}
	}()

	isReplay := false
	if strings.HasPrefix(serverAddress, "PCAP!") {
		isReplay = true
	}

	var tokenSource oauth2.TokenSource
	if !isReplay {
		// ask for login before listening
		tokenSource, err = Auth.GetTokenSource()
		if err != nil {
			return err
		}
	}

	var cdp *login.ClientData = nil
	if p.WithClient && !isReplay {
		CurrentUI.Message(messages.SetUIState(messages.UIStateConnect))
		err = p.connectClient(ctx, serverAddress, &cdp, tokenSource)
		if err != nil {
			return err
		}

		defer func() {
			if p.Listener != nil {
				if p.Client != nil {
					p.Listener.Disconnect(p.Client.(*minecraft.Conn), DisconnectReason)
				}
			}
		}()
	}

	if p.CustomClientData != nil {
		cdp = p.CustomClientData
	}

	for _, handler := range p.handlers {
		if handler.OnClientConnect == nil {
			continue
		}
		handler.OnClientConnect(p.Client)
	}

	if isReplay {
		p.Server, err = createReplayConnector(serverAddress[5:], p.packetFunc)
		if err != nil {
			return err
		}
	} else {
		p.Server, err = connectServer(ctx, serverAddress, cdp, p.AlwaysGetPacks, p.packetFunc, tokenSource)
	}
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

	for _, handler := range p.handlers {
		if handler.OnServerConnect == nil {
			continue
		}
		cancel := handler.OnServerConnect()
		if cancel {
			return nil
		}
	}

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
			logrus.Info("Disconnecting")
			return nil
		}
	}

	ctx2, cancel := context.WithCancelCause(ctx)

	wg := sync.WaitGroup{}
	doProxy := func(client bool) {
		defer wg.Done()
		if err := p.proxyLoop(ctx2, client); err != nil {
			cancel(err)
			return
		}
	}

	// server to client
	wg.Add(1)
	go doProxy(false)

	// client to server
	if p.Client != nil {
		wg.Add(1)
		go doProxy(true)
	}
	go func() {
		wg.Wait()
		if ctx.Err() == nil {
			cancel(nil)
		}
	}()

	/*
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
	*/

	<-ctx2.Done()
	err = ctx2.Err()
	if err, ok := err.(*transferingErr); ok {
		logrus.Infof("Redirect to %s", err.To)
		if p.Client != nil {
			p.Listener.Disconnect(p.Client.(*minecraft.Conn), "please reconnect")
		}
		return p.connect(ctx, err.To)
	}

	return nil
}

func (p *ProxyContext) Run(ctx context.Context, serverAddress, name string) (err error) {
	if Options.Debug || Options.ExtraDebug {
		p.AddHandler(NewDebugLogger(Options.ExtraDebug))
	}
	if Options.Capture {
		p.AddHandler(NewPacketCapturer())
	}
	p.AddHandler(&ProxyHandler{
		Name:     "Commands",
		PacketCB: p.CommandHandlerPacketCB,
	})

	p.AddHandler(p.reconnectHandler)

	for _, handler := range p.handlers {
		if handler.AddressAndName != nil {
			handler.AddressAndName(serverAddress, name)
		}
		if handler.ProxyRef != nil {
			handler.ProxyRef(p)
		}
	}

	return p.connect(ctx, serverAddress)
}

func DecodePacket(pool packet.Pool, header packet.Header, payload []byte) packet.Packet {
	var pk packet.Packet
	if pkFunc, ok := pool[header.PacketID]; ok {
		pk = pkFunc()
	} else {
		pk = &packet.Unknown{PacketID: header.PacketID, Payload: payload}
	}

	defer func() {
		if recoveredErr := recover(); recoveredErr != nil {
			logrus.Errorf("%T: %s", pk, recoveredErr.(error))
		}
	}()
	pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), 0, false))
	return pk
}
