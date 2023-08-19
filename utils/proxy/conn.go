package proxy

import (
	"context"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sirupsen/logrus"
)

func (p *Context) connectServer(ctx context.Context) (err error) {
	if p.WithClient {
		select {
		case <-p.clientConnecting:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": p.serverAddress}))
	p.Server, err = minecraft.Dialer{
		TokenSource: p.tokenSource,
		PacketFunc:  p.packetFunc,
		GetClientData: func() login.ClientData {
			if p.WithClient {
				<-p.haveClientData
			}
			return p.clientData
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			p.Server = c
			if p.WithClient {
				p.rpHandler.Server = c
			} else {
				p.rpHandler = newRpHandler(c, nil)
			}
			c.ResourcePackHandler = p.rpHandler
		},
	}.DialContext(ctx, "raknet", p.serverAddress)
	if err != nil {
		return err
	}

	logrus.Debug(locale.Loc("connected", nil))
	return nil
}

func (p *Context) connectClient(ctx context.Context, serverAddress string, cdpp **login.ClientData) (err error) {
	p.Listener, err = minecraft.ListenConfig{
		StatusProvider: minecraft.NewStatusProvider(fmt.Sprintf("%s Proxy", serverAddress)),
		//PacketFunc:     p.packetFunc,
		OnClientData: func(c *minecraft.Conn) {
			p.clientData = c.ClientData()
			close(p.haveClientData)
		},
		EarlyConnHandler: func(c *minecraft.Conn) {
			p.Client = c
			p.rpHandler = newRpHandler(nil, c)
			c.ResourcePackHandler = p.rpHandler
			close(p.clientConnecting)
		},
	}.Listen("raknet", ":19132")
	if err != nil {
		return err
	}

	p.ui.Message(messages.SetUIState(messages.UIStateConnect))
	logrus.Infof(locale.Loc("listening_on", locale.Strmap{"Address": p.Listener.Addr()}))
	logrus.Infof(locale.Loc("help_connect", nil))

	go func() {
		<-ctx.Done()
		if p.Client == nil {
			p.Listener.Close()
		}
	}()

	c, err := p.Listener.Accept()
	if err != nil {
		return err
	}
	p.Client = c.(*minecraft.Conn)
	cd := p.Client.ClientData()
	*cdpp = &cd
	return nil
}
