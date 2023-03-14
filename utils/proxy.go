package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

var DisconnectReason = "Connection lost"

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

type (
	PacketFunc            func(header packet.Header, payload []byte, src, dst net.Addr)
	PacketCallback        func(pk packet.Packet, proxy *ProxyContext, toServer bool, timeReceived time.Time) (packet.Packet, error)
	ClientConnectCallback func(proxy *ProxyContext, hasClient bool)
	ConnectCallback       func(proxy *ProxyContext, err error) bool
	IngameCommand         struct {
		Exec func(cmdline []string) bool
		Cmd  protocol.Command
	}
)

type ProxyContext struct {
	Server           *minecraft.Conn
	Client           *minecraft.Conn
	Listener         *minecraft.Listener
	commands         map[string]IngameCommand
	AlwaysGetPacks   bool
	WithClient       bool
	IgnoreDisconnect bool
	CustomClientData *login.ClientData

	// called for every packet
	PacketFunc PacketFunc
	// called after client connected
	OnClientConnect ClientConnectCallback
	// called after game started
	ConnectCB ConnectCallback
	// called on every packet after login
	PacketCB PacketCallback
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

func (p *ProxyContext) SendMessage(text string) {
	if p.Client != nil {
		p.Client.WritePacket(&packet.Text{
			TextType: packet.TextTypeSystem,
			Message:  "§8[§bBedrocktool§8]§r " + text,
		})
	}
}

func (p *ProxyContext) SendPopup(text string) {
	if p.Client != nil {
		p.Client.WritePacket(&packet.Text{
			TextType: packet.TextTypePopup,
			Message:  text,
		})
	}
}

func (p *ProxyContext) CommandHandlerPacketCB(pk packet.Packet, proxy *ProxyContext, toServer bool, _ time.Time) (packet.Packet, error) {
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

func (p *ProxyContext) proxyLoop(ctx context.Context, toServer bool, packetCBs []PacketCallback) error {
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

		for _, packetCB := range packetCBs {
			pk, err = packetCB(pk, p, toServer, time.Now())
			if err != nil {
				return err
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

var ClientAddr net.Addr

func (p *ProxyContext) Run(ctx context.Context, serverAddress string) (err error) {
	if strings.HasPrefix(serverAddress, "PCAP!") {
		return createReplayConnection(ctx, serverAddress[5:], p.ConnectCB, p.PacketCB)
	}

	GetTokenSource() // ask for login before listening

	var cdp *login.ClientData = nil
	if p.WithClient {
		var packs []*resource.Pack
		if Options.Preload {
			logrus.Info(locale.Loc("preloading_packs", nil))
			var serverConn *minecraft.Conn
			serverConn, err = connectServer(ctx, serverAddress, nil, true, nil)
			if err != nil {
				err = fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": serverAddress, "Err": err}))
				return
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

		go func() {
			<-ctx.Done()
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

	if p.OnClientConnect != nil {
		p.OnClientConnect(p, p.WithClient)
	}

	if p.CustomClientData != nil {
		cdp = p.CustomClientData
	}

	p.Server, err = connectServer(ctx, serverAddress, cdp, p.AlwaysGetPacks, p.PacketFunc)
	if err != nil {
		if p.ConnectCB != nil {
			if p.ConnectCB(p, err) {
				err = nil
			}
		}
		err = fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": serverAddress, "Err": err}))
		return err
	}
	defer p.Server.Close()

	// spawn and start the game
	if err = spawnConn(ctx, p.Client, p.Server); err != nil {
		err = fmt.Errorf(locale.Loc("failed_to_spawn", locale.Strmap{"Err": err}))
		return err
	}

	if p.ConnectCB != nil {
		if !p.ConnectCB(p, nil) {
			return errors.New("Cancelled")
		}
	}

	wg := sync.WaitGroup{}

	var cbs []PacketCallback
	cbs = append(cbs, p.CommandHandlerPacketCB)
	if p.PacketCB != nil {
		cbs = append(cbs, p.PacketCB)
	}

	// server to client
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := p.proxyLoop(ctx, false, cbs); err != nil {
			logrus.Error(err)
			return
		}
	}()

	// client to server
	if p.Client != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.proxyLoop(ctx, true, cbs); err != nil {
				logrus.Error(err)
				return
			}
		}()
	}

	wg.Wait()
	return err
}
