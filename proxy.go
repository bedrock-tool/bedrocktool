package main

import (
	"context"
	"errors"
	"fmt"
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
	server   *minecraft.Conn
	client   *minecraft.Conn
	listener *minecraft.Listener
	commands map[string]IngameCommand
}

type (
	PacketCallback  func(pk packet.Packet, proxy *ProxyContext, toServer bool) (packet.Packet, error)
	ConnectCallback func(proxy *ProxyContext)
)

func (p *ProxyContext) sendMessage(text string) {
	if p.client != nil {
		p.client.WritePacket(&packet.Text{
			TextType: packet.TextTypeSystem,
			Message:  "§8[§bBedrocktool§8]§r " + text,
		})
	}
}

func (p *ProxyContext) sendPopup(text string) {
	if p.client != nil {
		p.client.WritePacket(&packet.Text{
			TextType: packet.TextTypePopup,
			Message:  text,
		})
	}
}

type IngameCommand struct {
	exec func(cmdline []string) bool
	cmd  protocol.Command
}

func (p *ProxyContext) addCommand(cmd IngameCommand) {
	p.commands[cmd.cmd.Name] = cmd
}

func (p *ProxyContext) CommandHandlerPacketCB(pk packet.Packet, proxy *ProxyContext, toServer bool) (packet.Packet, error) {
	switch pk := pk.(type) {
	case *packet.CommandRequest:
		cmd := strings.Split(pk.CommandLine, " ")
		name := cmd[0][1:]
		if h, ok := p.commands[name]; ok {
			if h.exec(cmd[1:]) {
				pk = nil
			}
		}
	case *packet.AvailableCommands:
		for _, ic := range p.commands {
			pk.Commands = append(pk.Commands, ic.cmd)
		}
	}
	return pk, nil
}

func proxyLoop(ctx context.Context, proxy *ProxyContext, toServer bool, packetCBs []PacketCallback) error {
	var c1, c2 *minecraft.Conn
	if toServer {
		c1 = proxy.client
		c2 = proxy.server
	} else {
		c1 = proxy.server
		c2 = proxy.client
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

func create_proxy(ctx context.Context, log *logrus.Logger, server_address string, onConnect ConnectCallback, packetCB PacketCallback) (err error) {
	if strings.HasSuffix(server_address, ".pcap") {
		return create_replay_connection(ctx, log, server_address, onConnect, packetCB)
	}

	GetTokenSource() // ask for login before listening

	proxy := ProxyContext{}
	proxy.commands = make(map[string]IngameCommand)

	var packs []*resource.Pack
	if G_preload_packs {
		log.Info("Preloading resourcepacks")
		serverConn, err := connect_server(ctx, server_address, nil, true)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %s", server_address, err)
		}
		serverConn.Close()
		packs = serverConn.ResourcePacks()
		log.Infof("%d packs loaded\n", len(packs))
	}

	_status := minecraft.NewStatusProvider("Server")
	proxy.listener, err = minecraft.ListenConfig{
		StatusProvider: _status,
		ResourcePacks:  packs,
		AcceptedProtocols: []minecraft.Protocol{
			dummyProto{id: 544, ver: "1.19.20"},
		},
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}
	defer proxy.listener.Close()

	log.Infof("Listening on %s\n", proxy.listener.Addr())

	c, err := proxy.listener.Accept()
	if err != nil {
		log.Fatal(err)
	}
	proxy.client = c.(*minecraft.Conn)

	cd := proxy.client.ClientData()
	proxy.server, err = connect_server(ctx, server_address, &cd, false)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %s", server_address, err)
	}

	// spawn and start the game
	if err := spawn_conn(ctx, proxy.client, proxy.server); err != nil {
		return fmt.Errorf("failed to spawn: %s", err)
	}

	defer proxy.server.Close()
	defer proxy.listener.Disconnect(proxy.client, G_disconnect_reason)

	if onConnect != nil {
		onConnect(&proxy)
	}

	wg := sync.WaitGroup{}
	cbs := []PacketCallback{
		proxy.CommandHandlerPacketCB,
		packetCB,
	}

	// server to client
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxyLoop(ctx, &proxy, false, cbs); err != nil {
			log.Error(err)
			return
		}
	}()

	// client to server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxyLoop(ctx, &proxy, true, cbs); err != nil {
			log.Error(err)
			return
		}
	}()

	wg.Wait()
	return nil
}
