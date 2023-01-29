package subcommands

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func init() {
	utils.RegisterCommand(&CaptureCMD{})
}

var dump_lock sync.Mutex

func dump_packet(f io.WriteCloser, toServer bool, payload []byte) {
	dump_lock.Lock()
	defer dump_lock.Unlock()
	f.Write([]byte{0xAA, 0xAA, 0xAA, 0xAA})
	packet_size := uint32(len(payload))
	binary.Write(f, binary.LittleEndian, packet_size)
	binary.Write(f, binary.LittleEndian, toServer)
	_, err := f.Write(payload)
	if err != nil {
		logrus.Error(err)
	}
	f.Write([]byte{0xBB, 0xBB, 0xBB, 0xBB})
}

type CaptureCMD struct {
	server_address string
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return locale.Loc("capture_synopsis", nil) }

func (p *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.server_address, "address", "", "remote server address")
}

func (c *CaptureCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n" + locale.Loc("server_address_help", nil)
}

func (c *CaptureCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	address, hostname, err := utils.ServerInput(ctx, c.server_address)
	if err != nil {
		logrus.Fatal(err)
		return 1
	}

	os.Mkdir("captures", 0o775)

	fio, err := os.Create("captures/" + hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap2")
	if err != nil {
		logrus.Fatal(err)
		return 1
	}
	defer fio.Close()

	proxy := utils.NewProxy()
	proxy.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
		from_client := dst.String() == proxy.Server.RemoteAddr().String()

		buf := bytes.NewBuffer(nil)
		header.Write(buf)
		buf.Write(payload)
		dump_packet(fio, from_client, buf.Bytes())
	}

	err = proxy.Run(ctx, address)
	time.Sleep(2 * time.Second)
	if err != nil {
		logrus.Fatal(err)
		return 1
	}
	return 0
}
