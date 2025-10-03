package proxy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
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
	ctx          context.Context
	wg           sync.WaitGroup
	settings     ProxySettings
	OnPlayerMove []func()

	addedPacks []resource.Pack
	handlers   []func() *Handler
}

// New creates a new proxy context
func New(ctx context.Context, settings ProxySettings) (*Context, error) {
	p := &Context{
		ctx:      ctx,
		settings: settings,
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

func (p *Context) connect(connectInfo *connectinfo.ConnectInfo, withClient bool) (err error) {
	session := NewSession(p.ctx, p.settings, p.addedPacks, connectInfo, withClient)
	for _, handlerFunc := range p.handlers {
		session.handlers = append(session.handlers, handlerFunc())
	}

	serverName, err := connectInfo.Name(p.ctx)
	if err != nil {
		return err
	}

	session.handlers.SessionStart(session, serverName)
	err = session.Run()
	session.handlers.OnSessionEnd(session, &p.wg)

	if err, ok := err.(*errTransfer); ok {
		if connectInfo.IsReplay() {
			return nil
		}
		address := fmt.Sprintf("%s:%d", err.transfer.Address, err.transfer.Port)
		logrus.Infof("transferring to %s", address)
		return p.connect(&connectinfo.ConnectInfo{Value: address}, withClient)
	}
	return err
}

func (p *Context) newPlayerHandler() *Handler {
	return &Handler{
		Name: "Player",
		PacketCallback: func(s *Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			if pk, ok := pk.(*packet.PacketViolationWarning); ok {
				logrus.Infof("%+#v\n", pk)
			}

			haveMoved := s.Player.handlePackets(pk)
			if haveMoved {
				for _, cb := range p.OnPlayerMove {
					cb()
				}
			}
			return pk, nil
		},
	}
}

func (p *Context) Run(ctx context.Context, withClient bool) (err error) {
	err = utils.Netisolation()
	if err != nil {
		logrus.Warnf("Failed to Enable Loopback for Minecraft: %s", err)
	}

	defer func() {
		messages.SendEvent(&messages.EventSetUIState{
			State: messages.UIStateFinished,
		})
	}()

	if p.settings.ConnectInfo == nil || p.settings.ConnectInfo.Value == "" {
		return fmt.Errorf("no address")
	}

	if !p.settings.ConnectInfo.IsReplay() && p.settings.ConnectInfo.Account == nil {
		if !auth.Auth.LoggedIn() {
			err := auth.Auth.Login(ctx, &xbox.DeviceTypeAndroid, "")
			if err != nil {
				return err
			}
		}
		p.settings.ConnectInfo.Account = auth.Auth.Account()
	}

	if p.settings.Capture {
		p.AddHandler(NewPacketCapturer)
	}
	p.AddHandler(p.newPlayerHandler)
	p.addedPacks, err = loadForcedPacks()
	if err != nil {
		return err
	}

	err = p.connect(p.settings.ConnectInfo, withClient)
	p.wg.Wait()
	return err
}

func loadForcedPacks() ([]resource.Pack, error) {
	var packs []resource.Pack
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
				packs = append(packs, pack)
				logrus.Infof("Added %s to the forced packs", pack.Name())
			default:
				logrus.Warnf("Unrecognized file %s in forcedpacks", path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return packs, nil
}
