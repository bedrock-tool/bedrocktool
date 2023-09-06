package proxy

import (
	"context"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

func (p *Context) onResourcePacksInfo() {
	p.ui.Message(messages.ConnectStateReceivingResources)
}

func (p *Context) onFinishedPack(pack *resource.Pack) {
	p.ui.Message(messages.FinishedPack{Pack: pack})
}

func (p *Context) connectServer(ctx context.Context) (err error) {
	if p.withClient {
		select {
		case <-p.clientConnecting:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	p.ui.Message(messages.ConnectStateServerConnecting)
	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": p.serverAddress}))
	server, err := minecraft.Dialer{
		TokenSource: p.tokenSource,
		PacketFunc:  p.packetFunc,
		GetClientData: func() login.ClientData {
			if p.withClient {
				select {
				case <-p.haveClientData:
				case <-ctx.Done():
				}
			}
			return p.clientData
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			p.Server = c
			if p.withClient {
				p.rpHandler.Server = c
			} else {
				p.rpHandler = newRpHandler(ctx, c, nil, p.addedPacks)
				p.rpHandler.OnResourcePacksInfoCB = p.onResourcePacksInfo
				p.rpHandler.OnFinishedPack = p.onFinishedPack
			}
			c.ResourcePackHandler = p.rpHandler
		},
	}.DialContext(ctx, "raknet", p.serverAddress)
	if err != nil {
		return err
	}
	p.Server = server

	p.ui.Message(messages.ConnectState(messages.ConnectStateEstablished))
	logrus.Debug(locale.Loc("connected", nil))
	return nil
}

func (p *Context) connectClient(ctx context.Context, serverAddress string) (err error) {
	p.listener, err = minecraft.ListenConfig{
		StatusProvider: minecraft.NewStatusProvider(fmt.Sprintf("%s Proxy", serverAddress)),
		//PacketFunc:     p.packetFunc,
		OnClientData: func(c *minecraft.Conn) {
			p.clientData = c.ClientData()
			close(p.haveClientData)
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			p.Client = c
			p.rpHandler = newRpHandler(ctx, nil, c, p.addedPacks)
			p.rpHandler.OnResourcePacksInfoCB = p.onResourcePacksInfo
			p.rpHandler.OnFinishedPack = p.onFinishedPack
			c.ResourcePackHandler = p.rpHandler
			close(p.clientConnecting)
		},
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}

	p.ui.Message(messages.ConnectStateListening)
	logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": p.listener.Addr()}))
	logrus.Infof(locale.Loc("help_connect", nil))

	var accepted = false

	go func() {
		<-ctx.Done()
		if !accepted {
			_ = p.listener.Close()
		}
	}()

	c, err := p.listener.Accept()
	if err != nil {
		return err
	}
	accepted = true
	p.Client = c.(*minecraft.Conn)
	return nil
}
