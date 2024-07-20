package proxy

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type errTransfer struct {
	transfer *packet.Transfer
}

func (e errTransfer) Error() string {
	return fmt.Sprintf("transfer to %s:%d", e.transfer.Address, e.transfer.Port)
}

type Context struct {
	ExtraDebug    bool
	PlayerMoveCB  []func()
	ListenAddress string
	withClient    bool

	addedPacks []resource.Pack
	commands   map[string]ingameCommand
	handlers   []*Handler

	session *Session
}

// New creates a new proxy context
func New(withClient bool) (*Context, error) {
	p := &Context{
		commands:      make(map[string]ingameCommand),
		withClient:    withClient,
		ListenAddress: "0.0.0.0:19132",
	}
	return p, nil
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

func (p *Context) connect(ctx context.Context, connect *utils.ConnectInfo) (err error) {
	p.session = NewSession()
	p.session.withClient = p.withClient
	p.session.extraDebug = p.ExtraDebug
	p.session.addedPacks = p.addedPacks
	p.session.listenAddress = p.ListenAddress
	p.session.packetCallback = func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
		for _, handler := range p.handlers {
			pk, err = handler.PacketCallback(pk, toServer, timeReceived, preLogin)
			if err != nil {
				return nil, err
			}
			if pk == nil {
				break
			}
		}
		return pk, err
	}
	p.session.packetRaw = func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
		for _, handler := range p.handlers {
			if handler.PacketRaw != nil {
				handler.PacketRaw(header, payload, src, dst, timeReceived)
			}
		}
	}
	p.session.filterResourcePack = func(id string) bool {
		ignore := false
		for _, handler := range p.handlers {
			if handler.FilterResourcePack != nil {
				ignore = handler.FilterResourcePack(id)
			}
		}
		return ignore
	}
	p.session.onFinishedPack = func(pack resource.Pack) error {
		messages.Router.Handle(&messages.Message{
			Source: "proxy",
			Target: "ui",
			Data:   messages.FinishedPack{Pack: pack},
		})
		for _, handler := range p.handlers {
			if handler.OnFinishedPack != nil {
				err := handler.OnFinishedPack(pack)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	p.session.resourcePacksFinished = func() bool {
		for _, handler := range p.handlers {
			if handler.ResourcePacksFinished != nil {
				if handler.ResourcePacksFinished() {
					return true
				}
			}
		}
		return false
	}
	p.session.onServerConnect = func() error {
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
	p.session.gameDataModifier = func(gd *minecraft.GameData) {
		for _, handler := range p.handlers {
			if handler.GameDataModifier == nil {
				continue
			}
			handler.GameDataModifier(gd)
		}
	}
	p.session.onConnect = func() bool {
		for _, handler := range p.handlers {
			if handler.OnConnect == nil {
				continue
			}
			if handler.OnConnect() {
				return true
			}
		}
		return false
	}

	for _, handler := range p.handlers {
		if handler.SessionStart != nil {
			handler.SessionStart(p.session, connect.Name())
		}
	}

	err = p.session.Run(ctx, connect)

	for _, handler := range p.handlers {
		if handler.OnSessionEnd != nil {
			handler.OnSessionEnd()
		}
	}

	if err, ok := err.(*errTransfer); ok {
		address := fmt.Sprintf("%s:%d", err.transfer.Address, err.transfer.Port)
		logrus.Infof("transferring to %s", address)
		return p.connect(ctx, &utils.ConnectInfo{
			ServerAddress: address,
		})
	}
	return err
}

func (p *Context) Run(ctx context.Context, connect *utils.ConnectInfo) (err error) {
	defer func() {
		for _, handler := range p.handlers {
			if handler.OnProxyEnd != nil {
				handler.OnProxyEnd()
			}
		}
		messages.Router.Handle(&messages.Message{
			Source: "proxy",
			Target: "ui",
			Data:   messages.UIStateFinished,
		})
	}()

	if connect.Replay == "" && !utils.Auth.LoggedIn() {
		messages.Router.Handle(&messages.Message{
			Source: "proxy",
			Target: "ui",
			Data: messages.RequestLogin{
				Wait: true,
			},
		})
	}

	if utils.Options.Debug || utils.Options.ExtraDebug {
		p.ExtraDebug = utils.Options.ExtraDebug
		p.AddHandler(NewDebugLogger(utils.Options.ExtraDebug))
	}
	if utils.Options.Capture {
		p.AddHandler(NewPacketCapturer())
	}
	p.AddHandler(&Handler{
		Name:           "Commands",
		PacketCallback: p.commandHandlerPacketCB,
	})
	p.AddHandler(&Handler{
		Name: "Player",
		PacketCallback: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			haveMoved := p.session.Player.handlePackets(pk)
			if haveMoved {
				for _, cb := range p.PlayerMoveCB {
					cb()
				}
			}
			return pk, nil
		},
	})

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

	return p.connect(ctx, connect)
}
