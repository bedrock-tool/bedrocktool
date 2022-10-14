package subcommands

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"io"
	"net"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func init() {
	utils.RegisterCommand(&CaptureCMD{})
}

func dump_packet(f io.WriteCloser, toServer bool, payload []byte) {
	binary.Write(f, binary.LittleEndian, uint32(len(payload)))
	binary.Write(f, binary.LittleEndian, toServer)
	f.Write(payload)
}

type CaptureCMD struct {
	server_address string
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return "capture packets in a pcap file" }

func (p *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.server_address, "address", "", "remote server address")
}

func (c *CaptureCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + utils.SERVER_ADDRESS_HELP
}

func (c *CaptureCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := utils.ServerInput(c.server_address)
	if err != nil {
		logrus.Fatal(err)
		return 1
	}

	fio, err := os.Create(hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap2")
	if err != nil {
		logrus.Fatal(err)
		return 1
	}
	defer fio.Close()

	proxy := utils.NewProxy(logrus.StandardLogger())
	proxy.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
		from_client := src.String() == proxy.Client.LocalAddr().String()

		buf := bytes.NewBuffer(nil)
		header.Write(buf)
		buf.Write(payload)
		dump_packet(fio, from_client, buf.Bytes())
	}

	err = proxy.Run(ctx, address)
	if err != nil {
		logrus.Fatal(err)
		return 1
	}
	return 0
}
