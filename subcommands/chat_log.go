package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/handlers"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
)

type ChatLogCMD struct {
	ServerAddress     string
	Verbose           bool
	EnableClientCache bool
}

func (*ChatLogCMD) Name() string     { return "chat-log" }
func (*ChatLogCMD) Synopsis() string { return locale.Loc("chat_log_synopsis", nil) }
func (c *ChatLogCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
	f.BoolVar(&c.Verbose, "v", false, "verbose")
	f.BoolVar(&c.EnableClientCache, "client-cache", true, "Enable Client Cache")
}

func (c *ChatLogCMD) Execute(ctx context.Context) error {
	proxyContext, err := proxy.New(ctx, true, c.EnableClientCache)
	if err != nil {
		return err
	}
	proxyContext.AddHandler(handlers.NewChatLogger())

	server := ctx.Value(utils.ConnectInfoKey).(*utils.ConnectInfo)
	return proxyContext.Run(server)
}

func init() {
	commands.RegisterCommand(&ChatLogCMD{})
}
