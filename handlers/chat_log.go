package handlers

import (
	"fmt"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type chatLogger struct {
	Verbose bool
	fio     *os.File
}

func (c *chatLogger) PacketCB(pk packet.Packet, toServer bool, t time.Time, _ bool) (packet.Packet, error) {
	if text, ok := pk.(*packet.Text); ok {
		logLine := text.Message
		if c.Verbose {
			logLine += fmt.Sprintf("   (TextType: %d | XUID: %s | PlatformChatID: %s)", text.TextType, text.XUID, text.PlatformChatID)
		}
		c.fio.WriteString(fmt.Sprintf("[%s] ", t.Format(time.RFC3339)))
		logrus.Info(logLine)
		if toServer {
			c.fio.WriteString("SENT: ")
		}
		c.fio.WriteString(logLine + "\n")
	}
	return pk, nil
}

func NewChatLogger() *proxy.Handler {
	c := &chatLogger{}
	return &proxy.Handler{
		Name:     "Packet Capturer",
		PacketCB: c.PacketCB,
		AddressAndName: func(address, hostname string) error {
			filename := fmt.Sprintf("%s_%s_chat.log", hostname, time.Now().Format("2006-01-02_15-04-05_Z07"))
			f, err := os.Create(filename)
			if err != nil {
				return err
			}
			c.fio = f
			return nil
		},
		OnEnd: func() {
			c.fio.Close()
		},
	}
}
