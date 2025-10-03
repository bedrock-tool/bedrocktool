package subcommands

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type ChatLogSettings struct {
	ProxySettings proxy.ProxySettings

	Verbose bool `opt:"Verbose" flag:"verbose"`
}

type ChatLogCMD struct {
	ServerAddress     string
	Verbose           bool
	EnableClientCache bool
}

func (ChatLogCMD) Name() string {
	return "chat-log"
}

func (ChatLogCMD) Description() string {
	return locale.Loc("chat_log_synopsis", nil)
}

func (ChatLogCMD) Settings() any {
	return new(ChatLogSettings)
}

func (ChatLogCMD) Run(ctx context.Context, settings any) error {
	chatLogSettings := settings.(*ChatLogSettings)
	proxyContext, err := proxy.New(ctx, chatLogSettings.ProxySettings)
	if err != nil {
		return err
	}
	proxyContext.AddHandler(handlers.NewChatLogger())
	return proxyContext.Run(ctx, true)
}

func init() {
	commands.RegisterCommand(&ChatLogCMD{})
}
