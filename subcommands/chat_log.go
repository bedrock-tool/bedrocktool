package subcommands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
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

	filename := fmt.Sprintf("%s_%s_chat.log", hostname, time.Now().Format("2006-01-02_15-04-05_Z07"))
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}
	proxy.PacketCB = func(pk packet.Packet, toServer bool, t time.Time) (packet.Packet, error) {
		if text, ok := pk.(*packet.Text); ok {
			logLine := text.Message
			if c.Verbose {
				logLine += fmt.Sprintf("   (TextType: %d | XUID: %s | PlatformChatID: %s)", text.TextType, text.XUID, text.PlatformChatID)
			}
			f.WriteString(fmt.Sprintf("[%s] ", t.Format(time.RFC3339)))
			logrus.Info(logLine)
			if toServer {
				f.WriteString("SENT: ")
			}
			f.WriteString(logLine + "\n")
		}
		return pk, nil
	}

	err = proxy.Run(ctx, address)
	return err
}

func init() {
	utils.RegisterCommand(&ChatLogCMD{})
}
