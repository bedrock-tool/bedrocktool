package skins

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type SkinProxyCMD struct {
	server_address     string
	filter             string
	only_with_geometry bool
	pathCustomUserData string
}

func (*SkinProxyCMD) Name() string     { return "skins-proxy" }
func (*SkinProxyCMD) Synopsis() string { return locale.Loc("skins_proxy_synopsis", nil) }

func (c *SkinProxyCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.server_address, "address", "", locale.Loc("remote_address", nil))
	f.StringVar(&c.filter, "filter", "", locale.Loc("name_prefix", nil))
	f.BoolVar(&c.only_with_geometry, "only-geom", false, locale.Loc("only_with_geometry", nil))
	f.StringVar(&c.pathCustomUserData, "userdata", "", locale.Loc("custom_user_data", nil))
}

func (c *SkinProxyCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *SkinProxyCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := utils.ServerInput(ctx, c.server_address)
	if err != nil {
		logrus.Error(err)
		return 1
	}

	proxy, err := utils.NewProxy(c.pathCustomUserData)
	if err != nil {
		logrus.Error(err)
		return 1
	}

	outPathBase := fmt.Sprintf("skins/%s", hostname)
	os.MkdirAll(outPathBase, 0o755)

	s := NewSkinsSession(proxy, hostname, outPathBase)
	s.OnlyIfHasGeometry = c.only_with_geometry
	s.PlayerNameFilter = c.filter

	proxy.PacketCB = func(pk packet.Packet, proxy *utils.ProxyContext, toServer bool, _ time.Time) (packet.Packet, error) {
		if !toServer {
			s.ProcessPacket(pk)
		}
		return pk, nil
	}

	if err := proxy.Run(ctx, address); err != nil {
		logrus.Error(err)
	}

	return 0
}

func init() {
	utils.RegisterCommand(&SkinProxyCMD{})
}
