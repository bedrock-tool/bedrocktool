package proxy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Context struct {
	Server   minecraft.IConn
	Client   minecraft.IConn
	listener *minecraft.Listener
	Player   Player

	withClient bool
	addedPacks []*resource.Pack

	dimensionData    *packet.DimensionData
	tokenSource      oauth2.TokenSource
	clientConnecting chan struct{}
	haveClientData   chan struct{}
	clientData       login.ClientData
	clientAddr       net.Addr
	spawned          bool
	disconnectReason string
	serverAddress    string
	serverName       string

	commands  map[string]ingameCommand
	handlers  []*Handler
	transfer  *packet.Transfer
	rpHandler *rpHandler
	ui        ui.UI
}

// New creates a new proxy context
func New(ui ui.UI, withClient bool) (*Context, error) {
	p := &Context{
		commands:         make(map[string]ingameCommand),
		withClient:       withClient,
		disconnectReason: "Connection Lost",
		ui:               ui,
	}
	return p, nil
}

// AddCommand adds a command to the command handler
func (p *Context) AddCommand(exec func([]string) bool, cmd protocol.Command) {
	cmd.AliasesOffset = 0xffffffff
	p.commands[cmd.Name] = ingameCommand{exec, cmd}
}

// ClientWritePacket sends a packet to the client, nop if no client connected
func (p *Context) ClientWritePacket(pk packet.Packet) error {
	if p.Client == nil {
		return nil
	}
	return p.Client.WritePacket(pk)
}

// SendMessage sends a chat message to the client
func (p *Context) SendMessage(text string) {
	_ = p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypeSystem,
		Message:  "§8[§bBedrocktool§8]§r " + text,
	})
}

// SendPopup sends a toolbar popup to the client
func (p *Context) SendPopup(text string) {
	_ = p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

// AddHandler adds a handler to the proxy
func (p *Context) AddHandler(handler *Handler) {
	p.handlers = append(p.handlers, handler)
}

func (p *Context) commandHandlerPacketCB(pk packet.Packet, toServer bool, _ time.Time, _ bool) (packet.Packet, error) {
	switch _pk := pk.(type) {
	case *packet.CommandRequest:
		cmd := strings.Split(_pk.CommandLine, " ")
		name := cmd[0][1:]
		if h, ok := p.commands[name]; ok {
			pk = nil
			h.Exec(cmd[1:])
		}
	case *packet.AvailableCommands:
		cmds := make([]protocol.Command, 0, len(p.commands))
		for _, ic := range p.commands {
			cmds = append(cmds, ic.Cmd)
		}
		_pk.Commands = append(_pk.Commands, cmds...)
	}
	return pk, nil
}

func (p *Context) proxyLoop(ctx context.Context, toServer bool) (err error) {
	var c1, c2 minecraft.IConn
	if toServer {
		c1 = p.Client
		c2 = p.Server
	} else {
		c1 = p.Server
		c2 = p.Client
	}

	defer func() {
		rec := recover()
		if rec != nil {
			if s, ok := rec.(string); ok {
				rec = errors.New(s)
			}
			err = rec.(error)
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		pk, err := c1.ReadPacket()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return err
		}

		pkName := reflect.TypeOf(pk).String()
		for _, handler := range p.handlers {
			if handler.PacketCB != nil {
				pk, err = handler.PacketCB(pk, toServer, time.Now(), false)
				if err != nil {
					return err
				}
				if pk == nil {
					logrus.Tracef("Dropped Packet: %s", pkName)
					break
				}
			}
		}

		switch _pk := pk.(type) {
		case *packet.Transfer:
			p.transfer = _pk
			if p.Client != nil {
				host, port, err := net.SplitHostPort(p.Client.ClientData().ServerAddress)
				if err != nil {
					return err
				}
				// transfer to self
				_port, _ := strconv.Atoi(port)
				pk = &packet.Transfer{Address: host, Port: uint16(_port)}
			}
		}

		if pk != nil && c2 != nil {
			if err := c2.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					p.disconnectReason = disconnect.Error()
				}
				if errors.Is(err, net.ErrClosed) {
					err = nil
				}
				return err
			}
		}

		if p.transfer != nil {
			return nil
		}
	}
}

// Disconnect disconnects both the client and server
func (p *Context) Disconnect() {
	p.DisconnectClient()
	p.DisconnectServer()
}

// Disconnect disconnects the client
func (p *Context) DisconnectClient() {
	if p.Client == nil {
		return
	}
	_ = p.Client.Close()
}

// Disconnect disconnects from the server
func (p *Context) DisconnectServer() {
	if p.Server == nil {
		return
	}
	_ = p.Server.Close()
}

func (p *Context) IsClient(addr net.Addr) bool {
	return p.clientAddr.String() == addr.String()
}

func (p *Context) packetFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	if header.PacketID == packet.IDRequestNetworkSettings {
		p.clientAddr = src
	}
	if header.PacketID == packet.IDSetLocalPlayerAsInitialised {
		p.spawned = true
	}

	for _, h := range p.handlers {
		if h.PacketRaw != nil {
			h.PacketRaw(header, payload, src, dst)
		}
	}

	if !p.spawned {
		pk, ok := DecodePacket(header, payload)
		if !ok {
			return
		}

		switch pk := pk.(type) {
		case *packet.DimensionData:
			p.dimensionData = pk
		}

		var err error
		toServer := p.IsClient(src)
		for _, handler := range p.handlers {
			if handler.PacketCB != nil {
				pk, err = handler.PacketCB(pk, toServer, time.Now(), !p.spawned)
				if err != nil {
					logrus.Error(err)
				}
			}
		}
	}
}

func (p *Context) onServerConnect() error {
	for _, handler := range p.handlers {
		if handler.OnServerConnect == nil {
			continue
		}
		disconnect, err := handler.OnServerConnect()
		if err != nil {
			return err
		}
		if disconnect {
			return errCancelConnect
		}
	}
	return nil
}

func (p *Context) doSession(ctx context.Context, cancel context.CancelCauseFunc) (err error) {
	defer func() {
		for _, handler := range p.handlers {
			if handler.OnEnd != nil {
				handler.OnEnd()
			}
		}
	}()

	for _, handler := range p.handlers {
		if handler.AddressAndName != nil {
			err = handler.AddressAndName(p.serverAddress, p.serverName)
			if err != nil {
				return err
			}
		}
	}

	isReplay := false
	if strings.HasPrefix(p.serverAddress, "PCAP!") {
		isReplay = true
	}

	if !isReplay {
		// ask for login before listening
		p.tokenSource, err = utils.Auth.GetTokenSource()
		if err != nil {
			return err
		}
	}

	p.ui.Message(messages.ConnectStateBegin)

	// setup Client and Server Connections
	wg := sync.WaitGroup{}
	if isReplay {
		server, err := CreateReplayConnector(ctx, p.serverAddress[5:], p.packetFunc, p.onResourcePacksInfo, p.onFinishedPack)
		if err != nil {
			return err
		}
		p.Server = server
	} else {
		p.rpHandler = newRpHandler(ctx, p.addedPacks)
		p.rpHandler.OnResourcePacksInfoCB = p.onResourcePacksInfo
		p.rpHandler.OnFinishedPack = p.onFinishedPack

		if p.withClient {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = p.connectClient(ctx, p.serverAddress)
				if err != nil {
					cancel(err)
					return
				}
				for _, handler := range p.handlers {
					if handler.OnClientConnect == nil {
						continue
					}
					handler.OnClientConnect(p.Client)
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = p.connectServer(ctx)
			if err != nil {
				cancel(err)
				return
			}
		}()
	}

	wg.Wait()
	if p.Server != nil {
		defer p.Server.Close()
	}
	if p.listener != nil {
		defer func() {
			if p.Client != nil {
				_ = p.listener.Disconnect(p.Client.(*minecraft.Conn), p.disconnectReason)
			}
			_ = p.listener.Close()
		}()
	}

	if ctx.Err() == nil {
		err = p.onServerConnect()
		if err != nil {
			cancel(err)
		}
	}

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		if errors.Is(err, errCancelConnect) {
			err = nil
		}
		if err != nil {
			p.disconnectReason = err.Error()
		} else {
			p.disconnectReason = "Disconnect"
		}
		return err
	}

	{ // spawn
		gd := p.Server.GameData()
		for _, handler := range p.handlers {
			if handler.ToClientGameDataModifier != nil {
				handler.ToClientGameDataModifier(&gd)
			}
		}

		if p.Client != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p.dimensionData != nil {
					p.Client.WritePacket(p.dimensionData)
				}
				err := p.Client.StartGameContext(ctx, gd)
				if err != nil {
					cancel(err)
					return
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := p.Server.DoSpawnContext(ctx)
			if err != nil {
				cancel(err)
				return
			}
		}()

		wg.Wait()
		err = context.Cause(ctx)
		if err != nil {
			p.disconnectReason = err.Error()
			return err
		}

		for _, handler := range p.handlers {
			if handler.ConnectCB != nil {
				if handler.ConnectCB() {
					logrus.Info("Disconnecting")
					return nil
				}
			}
		}
	}

	p.ui.Message(messages.ConnectState(messages.ConnectStateDone))

	{ // packet loop
		doProxy := func(client bool) {
			defer wg.Done()
			if err := p.proxyLoop(ctx, client); err != nil {
				if !errors.Is(err, context.Canceled) {
					cancel(err)
				}
				return
			}
			if p.transfer != nil {
				cancel(errTransfer)
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

		wg.Wait()
		err = context.Cause(ctx)
		if err != nil {
			p.disconnectReason = err.Error()
		}
	}

	return err
}

var errTransfer = errors.New("err transfer")

func (p *Context) connect(ctx context.Context) (err error) {
	p.spawned = false
	p.clientAddr = nil
	p.transfer = nil
	p.Client = nil
	p.clientConnecting = make(chan struct{})
	p.haveClientData = make(chan struct{})
	ctx2, cancel := context.WithCancelCause(ctx)
	err = p.doSession(ctx2, cancel)
	cancel(nil)

	if errors.Is(err, errTransfer) && p.transfer != nil {
		p.serverAddress = fmt.Sprintf("%s:%d", p.transfer.Address, p.transfer.Port)
		logrus.Infof("transferring to %s", p.serverAddress)
		return p.connect(ctx)
	}

	return err
}

func (p *Context) Run(ctx context.Context, connectString string) (err error) {
	p.serverAddress, p.serverName, err = p.ui.ServerInput(ctx, connectString)
	if err != nil {
		return err
	}

	if utils.Options.Debug || utils.Options.ExtraDebug {
		p.AddHandler(NewDebugLogger(utils.Options.ExtraDebug, false))
	}
	if utils.Options.Capture {
		p.AddHandler(NewPacketCapturer())
	}
	p.AddHandler(&Handler{
		Name:     "Commands",
		PacketCB: p.commandHandlerPacketCB,
	})
	p.AddHandler(&Handler{
		Name: "Player",
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			p.Player.handlePackets(pk)
			return pk, nil
		},
	})

	for _, handler := range p.handlers {
		if handler.ProxyRef != nil {
			handler.ProxyRef(p)
		}
	}

	defer func() {
		for _, handler := range p.handlers {
			if handler.Deferred != nil {
				handler.Deferred()
			}
		}
		p.ui.Message(messages.SetUIState(messages.UIStateFinished))
	}()

	// load forced packs
	if _, err := os.Stat("forcedpacks"); err == nil {
		if err = filepath.WalkDir("forcedpacks/", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			switch ext {
			case ".mcpack", ".zip":
				pack, err := resource.ReadPath(path)
				if err != nil {
					return err
				}
				p.addedPacks = append(p.addedPacks, pack)
				logrus.Infof("Added %s to the forced packs", pack.Name())
			default:
				logrus.Warnf("Unrecognized file %s in forcedpacks", path)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	return p.connect(ctx)
}
