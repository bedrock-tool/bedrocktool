package subcommands

import (
	"context"
	"flag"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/handlers"
)

type ChatLogCMD struct {
	ServerAddress string
	Verbose       bool
}

func (*ChatLogCMD) Name() string     { return "chat-log" }
func (*ChatLogCMD) Synopsis() string { return locale.Loc("chat_log_synopsis", nil) }
func (c *ChatLogCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
	f.BoolVar(&c.Verbose, "v", false, "verbose")
}

func (c *ChatLogCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}
	proxy.AddHandler(handlers.NewChatLogger())
	return proxy.Run(ctx, address, hostname)
}

func init() {
	utils.RegisterCommand(&ChatLogCMD{})
}
