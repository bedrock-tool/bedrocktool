package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var DisconnectReason = "Connection lost"

type (
	PacketFunc    func(header packet.Header, payload []byte, src, dst net.Addr)
	ingameCommand struct {
		Exec func(cmdline []string) bool
		Cmd  protocol.Command
	}
)

type Handler struct {
	Name     string
	ProxyRef func(pc *Context)
	//
	AddressAndName func(address, hostname string) error

	// called to change game data
	ToClientGameDataModifier func(gd *minecraft.GameData)

	// Called with raw packet data
	PacketRaw func(header packet.Header, payload []byte, src, dst net.Addr)

	// called on every packet after login
	PacketCB func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error)

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

type Context struct {
	Server           minecraft.IConn
	Client           minecraft.IConn
	Listener         *minecraft.Listener
	tokenSource      oauth2.TokenSource
	clientConnecting chan bool
	clientAddr       net.Addr
	spawned          bool

	AlwaysGetPacks   bool
	WithClient       bool
	IgnoreDisconnect bool
	CustomClientData *login.ClientData

	serverAddress string
	serverName    string

	commands map[string]ingameCommand
	handlers []*Handler

	transfer  *packet.Transfer
	rpHandler *rpHandler
	ui        ui.UI
}

func New(ui ui.UI) (*Context, error) {
	p := &Context{
		commands:         make(map[string]ingameCommand),
		AlwaysGetPacks:   false,
		WithClient:       true,
		IgnoreDisconnect: false,
		ui:               ui,
	}
	return p, nil
}

func (p *Context) AddCommand(exec func([]string) bool, cmd protocol.Command) {
	p.commands[cmd.Name] = ingameCommand{exec, cmd}
}

func (p *Context) ClientWritePacket(pk packet.Packet) error {
	if p.Client == nil {
		return nil
	}
	return p.Client.WritePacket(pk)
}

func (p *Context) SendMessage(text string) {
	p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypeSystem,
		Message:  "§8[§bBedrocktool§8]§r " + text,
	})
}

func (p *Context) SendPopup(text string) {
	p.ClientWritePacket(&packet.Text{
		TextType: packet.TextTypePopup,
		Message:  text,
	})
}

func (p *Context) AddHandler(handler *Handler) {
	p.handlers = append(p.handlers, handler)
}

func (p *Context) CommandHandlerPacketCB(pk packet.Packet, toServer bool, _ time.Time, _ bool) (packet.Packet, error) {
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

func (p *Context) proxyLoop(ctx context.Context, toServer bool) error {
	var c1, c2 minecraft.IConn
	if toServer {
		c1 = p.Client
		c2 = p.Server
	} else {
		c1 = p.Server
		c2 = p.Client
	}

	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}

		pk, err := c1.ReadPacket()
		if err != nil {
			if errors.Is(err, io.EOF) {
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
					DisconnectReason = disconnect.Error()
				}
				if errors.Is(err, io.EOF) {
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
	p.Client.Close()
}

// Disconnect disconnects from the server
func (p *Context) DisconnectServer() {
	if p.Server == nil {
		return
	}
	p.Server.Close()
}

func (p *Context) IsClient(addr net.Addr) bool {
	return p.clientAddr.String() == addr.String()
}

var NewDebugLogger func(bool) *Handler
var NewPacketCapturer func() *Handler
var d *Handler

func (p *Context) packetFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	/* for logging the to client packets
	if dst.String() == "[::]:19132" || src.String() == "[::]:19132" {
		pk, ok := decodePacket(header, payload)
		if !ok {
			return
		}
		d.PacketCB(pk, dst.String() == "[::]:19132", time.Now(), true)
		return
	}
	*/

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
		pk, ok := decodePacket(header, payload)
		if !ok {
			return
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

var errCancelConnect = fmt.Errorf("cancelled connecting")

func (p *Context) onServerConnect() error {
	for _, handler := range p.handlers {
		if handler.OnServerConnect == nil {
			continue
		}
		disconnect := handler.OnServerConnect()
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
			handler.AddressAndName(p.serverAddress, p.serverName)
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

	// setup Client and Server Connections
	wg := sync.WaitGroup{}
	var cdp *login.ClientData = nil
	if isReplay {
		p.Server, err = createReplayConnector(p.serverAddress[5:], p.packetFunc)
		if err != nil {
			return err
		}
	} else {
		if p.WithClient {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = p.connectClient(ctx, p.serverAddress, &cdp)
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
			err = p.onServerConnect()
			if err != nil {
				cancel(err)
				return
			}
		}()
	}

	wg.Wait()
	defer p.Server.Close()
	if p.Listener != nil {
		defer func() {
			p.Listener.Disconnect(p.Client.(*minecraft.Conn), DisconnectReason)
			p.Listener.Close()
		}()
	}

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		if errors.Is(err, errCancelConnect) {
			err = nil
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
			return err
		}

		for _, handler := range p.handlers {
			if handler.ConnectCB == nil {
				continue
			}
			if handler.ConnectCB(nil) {
				logrus.Info("Disconnecting")
				return nil
			}
		}
	}

	{ // packet loop
		doProxy := func(client bool) {
			defer wg.Done()
			if err := p.proxyLoop(ctx, client); err != nil {
				cancel(err)
				return
			}
			if p.transfer != nil {
				cancel(errTransfer{})
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
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		}
	}

	return err
}

func (p *Context) connect(ctx context.Context) (err error) {
	p.spawned = false
	p.clientAddr = nil
	p.transfer = nil
	p.Client = nil
	p.clientConnecting = make(chan bool)
	ctx2, cancel := context.WithCancelCause(ctx)
	err = p.doSession(ctx2, cancel)

	if _, ok := err.(errTransfer); ok {
		p.serverAddress = fmt.Sprintf("%s:%d", p.transfer.Address, p.transfer.Port)
		logrus.Infof("transferring to %s", p.serverAddress)
		return p.connect(ctx)
	}

	return err
}

func (p *Context) Run(ctx context.Context, serverAddress, name string) (err error) {
	p.serverAddress = serverAddress
	p.serverName = name

	utils.Auth.Ctx = ctx

	if utils.Options.Debug || utils.Options.ExtraDebug {
		d = NewDebugLogger(utils.Options.ExtraDebug)
		p.AddHandler(d)
	}
	if utils.Options.Capture {
		p.AddHandler(NewPacketCapturer())
	}
	p.AddHandler(&Handler{
		Name:     "Commands",
		PacketCB: p.CommandHandlerPacketCB,
	})

	for _, handler := range p.handlers {
		if handler.ProxyRef != nil {
			handler.ProxyRef(p)
		}
	}

	return p.connect(ctx)
}

var serverPool = packet.NewServerPool()
var clientPool = packet.NewClientPool()

func decodePacket(header packet.Header, payload []byte) (packet.Packet, bool) {
	var pk packet.Packet

	pkFunc, ok := serverPool[header.PacketID]
	if !ok {
		pkFunc, ok = clientPool[header.PacketID]
	}
	if ok {
		pk = pkFunc()
	} else {
		pk = &packet.Unknown{PacketID: header.PacketID, Payload: payload}
	}

	var success = true
	defer func() {
		if recoveredErr := recover(); recoveredErr != nil {
			logrus.Errorf("%T: %s", pk, recoveredErr.(error))
			success = false
		}
	}()
	pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), 0, false))
	return pk, success
}
