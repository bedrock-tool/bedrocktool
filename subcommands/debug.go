package subcommands

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type DebugProxySettings struct {
	ProxySettings proxy.ProxySettings
}

type DebugProxyCMD struct{}

func (DebugProxyCMD) Name() string {
	return "debug-proxy"
}

func (DebugProxyCMD) Description() string {
	return locale.Loc("debug_proxy_synopsis", nil)
}

func (DebugProxyCMD) Settings() any {
	return new(DebugProxySettings)
}

func (DebugProxyCMD) Run(ctx context.Context, settings any) error {
	debugProxySettings := settings.(*DebugProxySettings)
	debugProxySettings.ProxySettings.Debug = true
	p, err := proxy.New(ctx, debugProxySettings.ProxySettings)
	if err != nil {
		return err
	}
	return p.Run(ctx, true)
}

func init() {
	commands.RegisterCommand(&DebugProxyCMD{})
}
