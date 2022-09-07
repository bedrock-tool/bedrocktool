package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
	Server   *minecraft.Conn
	Client   *minecraft.Conn
	Listener *minecraft.Listener
	commands map[string]IngameCommand

	log *logrus.Logger

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
		for _, ic := range p.commands {
			pk.Commands = append(pk.Commands, ic.Cmd)
		}
	}
	return pk, nil
}

func proxyLoop(ctx context.Context, proxy *ProxyContext, toServer bool, packetCBs []PacketCallback) error {
	var c1, c2 *minecraft.Conn
	if toServer {
		c1 = proxy.Client
		c2 = proxy.Server
	} else {
		c1 = proxy.Server
		c2 = proxy.Client
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
			pk, err = packetCB(pk, proxy, toServer)
			if err != nil {
				return err
			}
		}

		if pk != nil {
			if err := c2.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					G_disconnect_reason = disconnect.Error()
				}
				return err
			}
		}
	}
}

func NewProxy(log *logrus.Logger) *ProxyContext {
	if log == nil {
		log = logrus.StandardLogger()
	}
	return &ProxyContext{
		log:      log,
		commands: make(map[string]IngameCommand),
	}
}

var Client_addr net.Addr

func (p *ProxyContext) Run(ctx context.Context, server_address string) (err error) {
	if strings.HasSuffix(server_address, ".pcap") {
		return create_replay_connection(ctx, p.log, server_address, p.ConnectCB, p.PacketCB)
	}

	GetTokenSource() // ask for login before listening

	var packs []*resource.Pack
	if G_preload_packs {
		p.log.Info("Preloading resourcepacks")
		serverConn, err := ConnectServer(ctx, server_address, nil, true, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %s", server_address, err)
		}
		serverConn.Close()
		packs = serverConn.ResourcePacks()
		p.log.Infof("%d packs loaded", len(packs))
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
		return err
	}
	defer p.Listener.Close()

	p.log.Infof("Listening on %s", p.Listener.Addr())

	c, err := p.Listener.Accept()
	if err != nil {
		p.log.Fatal(err)
	}
	p.Client = c.(*minecraft.Conn)

	cd := p.Client.ClientData()
	p.Server, err = ConnectServer(ctx, server_address, &cd, false, p.PacketFunc)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %s", server_address, err)
	}

	// spawn and start the game
	if err := spawn_conn(ctx, p.Client, p.Server); err != nil {
		return fmt.Errorf("failed to spawn: %s", err)
	}

	defer p.Server.Close()
	defer p.Listener.Disconnect(p.Client, G_disconnect_reason)

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
		if err := proxyLoop(ctx, p, false, cbs); err != nil {
			p.log.Error(err)
			return
		}
	}()

	// client to server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxyLoop(ctx, p, true, cbs); err != nil {
			p.log.Error(err)
			return
		}
	}()

	wg.Wait()
	return nil
}
