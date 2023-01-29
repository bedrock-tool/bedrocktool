package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

var G_disconnect_reason = "Connection lost"

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

type ProxyContext struct {
	Server         *minecraft.Conn
	Client         *minecraft.Conn
	Listener       *minecraft.Listener
	commands       map[string]IngameCommand
	AlwaysGetPacks bool
	WithClient     bool

	// called for every packet
	PacketFunc PacketFunc
	// called after game started
	ConnectCB ConnectCallback
	// called on every packet after login
	PacketCB PacketCallback
}

type (
	PacketFunc      func(header packet.Header, payload []byte, src, dst net.Addr)
	PacketCallback  func(pk packet.Packet, proxy *ProxyContext, toServer bool) (packet.Packet, error)
	ConnectCallback func(proxy *ProxyContext)
)

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

type IngameCommand struct {
	Exec func(cmdline []string) bool
	Cmd  protocol.Command
}

func (p *ProxyContext) AddCommand(cmd IngameCommand) {
	p.commands[cmd.Cmd.Name] = cmd
}

func (p *ProxyContext) CommandHandlerPacketCB(pk packet.Packet, proxy *ProxyContext, toServer bool) (packet.Packet, error) {
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
			pk, err = packetCB(pk, p, toServer)
			if err != nil {
				return err
			}
		}

		if pk != nil && c2 != nil {
			if err := c2.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					G_disconnect_reason = disconnect.Error()
				}
				return err
			}
		}
	}
}

func NewProxy() *ProxyContext {
	return &ProxyContext{
		commands:   make(map[string]IngameCommand),
		WithClient: true,
	}
}

var Client_addr net.Addr

func (p *ProxyContext) Run(ctx context.Context, server_address string) (err error) {
	if strings.HasSuffix(server_address, ".pcap") {
		return fmt.Errorf(locale.Loc("not_supported_anymore", nil))
	}
	if strings.HasSuffix(server_address, ".pcap2") {
		return create_replay_connection(ctx, server_address, p.ConnectCB, p.PacketCB)
	}

	GetTokenSource() // ask for login before listening

	var cdp *login.ClientData = nil
	if p.WithClient {
		var packs []*resource.Pack
		if G_preload_packs {
			logrus.Info(locale.Loc("preloading_packs", nil))
			var serverConn *minecraft.Conn
			serverConn, err = connectServer(ctx, server_address, nil, true, nil)
			if err != nil {
				err = fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": server_address, "Err": err}))
				return
			}
			serverConn.Close()
			packs = serverConn.ResourcePacks()
			logrus.Infof(locale.Locm("pack_count_loaded", locale.Strmap{"Count": len(packs)}, len(packs)))
		}

		_status := minecraft.NewStatusProvider("Server")
		p.Listener, err = minecraft.ListenConfig{
			StatusProvider: _status,
			ResourcePacks:  packs,
			AcceptedProtocols: []minecraft.Protocol{
				dummyProto{id: 544, ver: "1.19.20"},
			},
		}.Listen("raknet", ":19132")
		if err != nil {
			return
		}
		defer p.Listener.Close()

		logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": p.Listener.Addr()}))
		logrus.Infof(locale.Loc("help_connect", nil))

		go func() {
			<-ctx.Done()
			p.Listener.Close()
		}()

		var c net.Conn
		c, err = p.Listener.Accept()
		if err != nil {
			logrus.Fatal(err)
		}
		p.Client = c.(*minecraft.Conn)
		cd := p.Client.ClientData()
		cdp = &cd
	}
	p.Server, err = connectServer(ctx, server_address, cdp, p.AlwaysGetPacks, p.PacketFunc)
	if err != nil {
		err = fmt.Errorf(locale.Loc("failed_to_connect", locale.Strmap{"Address": server_address, "Err": err}))
		return
	}
	// spawn and start the game
	if err = spawn_conn(ctx, p.Client, p.Server); err != nil {
		err = fmt.Errorf(locale.Loc("failed_to_spawn", locale.Strmap{"Err": err}))
		return
	}

	defer p.Server.Close()
	if p.Listener != nil {
		defer p.Listener.Disconnect(p.Client, G_disconnect_reason)
	}

	if p.ConnectCB != nil {
		p.ConnectCB(p)
	}

	wg := sync.WaitGroup{}
	cbs := []PacketCallback{
		p.CommandHandlerPacketCB,
	}
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
