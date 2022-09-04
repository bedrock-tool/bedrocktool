package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type SkinProxyCMD struct {
	server_address string
	filter         string
}

func (*SkinProxyCMD) Name() string     { return "skins-proxy" }
func (*SkinProxyCMD) Synopsis() string { return "download skins from players on a server with proxy" }

func (c *SkinProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.server_address, "address", "", "remote server address")
	f.StringVar(&c.filter, "filter", "", "player name filter prefix")
}

func (c *SkinProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + SERVER_ADDRESS_HELP
}

func (c *SkinProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := server_input(c.server_address)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	out_path := fmt.Sprintf("skins/%s", hostname)
	os.MkdirAll(out_path, 0o755)

	err = create_proxy(ctx, logrus.StandardLogger(), address, nil, func(pk packet.Packet, proxy *ProxyContext, toServer bool) (packet.Packet, error) {
		if !toServer {
			process_packet_skins(proxy.client, out_path, pk, c.filter)
		}
		return pk, nil
	})
	if err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func init() {
	register_command(&SkinProxyCMD{})
}
