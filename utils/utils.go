package utils

import (
	"context"
	"encoding/json"
	"net"
	"regexp"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"

	//"github.com/sandertv/gophertunnel/minecraft/gatherings"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const SERVER_ADDRESS_HELP = `accepted server address formats:
  123.234.123.234
  123.234.123.234:19132
  realm:<Realmname>
  realm:<Realmname>:<Id>

`

var (
	G_debug         bool
	G_cleanup_funcs []func() = []func(){}
)

var name_regexp = regexp.MustCompile(`\||(?:§.?)`)

// cleans name so it can be used as a filename
func CleanupName(name string) string {
	name = strings.Split(name, "\n")[0]
	var _tmp struct {
		K string `json:"k"`
	}
	err := json.Unmarshal([]byte(name), &_tmp)
	if err == nil {
		name = _tmp.K
	}
	name = string(name_regexp.ReplaceAll([]byte(name), []byte("")))
	name = strings.TrimSpace(name)
	return name
}

// connections

type (
	PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr)
)

func ConnectServer(ctx context.Context, address, clientName string, packetFunc PacketFunc) (serverConn *minecraft.Conn, err error) {
	var local_addr net.Addr
	packet_func := func(header packet.Header, payload []byte, src, dst net.Addr) {
		if G_debug {
			PacketLogger(header, payload, src, dst, local_addr)
			if packetFunc != nil {
				packetFunc(header, payload, src, dst)
			}
		}
	}

	logrus.Infof("Connecting to %s", address)
	serverConn, err = minecraft.Dialer{
		TokenSource:   GetTokenSource(clientName),
		PacketFunc:    packet_func,
		DownloadPacks: false,
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return nil, err
	}

	local_addr = serverConn.LocalAddr()

	logrus.Debug("Connected.")
	return serverConn, nil
}
