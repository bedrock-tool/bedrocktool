package proxy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type errTransfer struct {
	transfer *packet.Transfer
}

func (e *errTransfer) Error() string {
	return fmt.Sprintf("transfer to %s:%d", e.transfer.Address, e.transfer.Port)
}

type Context struct {
	ctx               context.Context
	ExtraDebug        bool
	PlayerMoveCB      []func()
	ListenAddress     string
	withClient        bool
	EnableClientCache bool

	addedPacks []resource.Pack
	handlers   []func() *Handler
	onHitBlobs func([]protocol.CacheBlob)
}

// New creates a new proxy context
func New(ctx context.Context, withClient, EnableClientCache bool) (*Context, error) {
	p := &Context{
		ctx:           ctx,
		withClient:    withClient,
		ListenAddress: "0.0.0.0:19132",
	}
	return p, nil
}

// AddHandler adds a handler to the proxy
func (p *Context) AddHandler(handler func() *Handler) {
	p.handlers = append(p.handlers, handler)
}

func (p *Context) Context() context.Context {
	return p.ctx
}

func (p *Context) connect(connectInfo *utils.ConnectInfo) (err error) {
	session := NewSession(p.ctx)
	session.withClient = p.withClient
	session.extraDebug = p.ExtraDebug
	session.addedPacks = p.addedPacks
	session.listenAddress = p.ListenAddress
	for _, hf := range p.handlers {
		session.handlers = append(session.handlers, hf())
	}
	session.onHitBlobs = p.onHitBlobs
	session.enableClientCache = p.EnableClientCache

	session.handlers.SessionStart(session, connectInfo.Name())
	err = session.Run(connectInfo)
	session.handlers.OnSessionEnd(session)

	if err, ok := err.(*errTransfer); ok {
		if connectInfo.Replay != "" {
			return nil
		}
		address := fmt.Sprintf("%s:%d", err.transfer.Address, err.transfer.Port)
		logrus.Infof("transferring to %s", address)
		return p.connect(&utils.ConnectInfo{
			ServerAddress: address,
		})
	}
	return err
}

func (p *Context) Run(connect *utils.ConnectInfo) (err error) {
	defer func() {
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
		if !utils.Auth.LoggedIn() {
			return errors.New("not Logged In")
		}
	}

	p.onHitBlobs = func([]protocol.CacheBlob) {}
	if utils.Options.Capture {
		hf, onHitBlobs := NewPacketCapturer()
		p.onHitBlobs = onHitBlobs
		p.AddHandler(hf)
	}

	p.AddHandler(func() *Handler {
		return &Handler{
			Name: "Player",
			OnFinishedPack: func(_ *Session, pack resource.Pack) error {
				messages.Router.Handle(&messages.Message{
					Source: "proxy",
					Target: "ui",
					Data:   messages.FinishedPack{Pack: pack},
				})
				return nil
			},
			PacketCallback: func(s *Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
				if pk, ok := pk.(*packet.PacketViolationWarning); ok {
					logrus.Infof("%+#v\n", pk)
				}

				haveMoved := s.Player.handlePackets(pk)
				if haveMoved {
					for _, cb := range p.PlayerMoveCB {
						cb()
					}
				}
				return pk, nil
			},
		}
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

	return p.connect(connect)
}
