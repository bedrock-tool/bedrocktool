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

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func init() {
	utils.RegisterCommand(&CaptureCMD{})
}

var dumpLock sync.Mutex

func dumpPacket(f io.WriteCloser, toServer bool, payload []byte) {
	dumpLock.Lock()
	defer dumpLock.Unlock()
	f.Write([]byte{0xAA, 0xAA, 0xAA, 0xAA})
	packetSize := uint32(len(payload))
	binary.Write(f, binary.LittleEndian, packetSize)
	binary.Write(f, binary.LittleEndian, toServer)
	binary.Write(f, binary.LittleEndian, time.Now().UnixMilli())
	n, err := f.Write(payload)
	if err != nil {
		logrus.Error(err)
	}
	if n < int(packetSize) {
		f.Write(make([]byte, int(packetSize)-n))
	}
	f.Write([]byte{0xBB, 0xBB, 0xBB, 0xBB})
}

type CaptureCMD struct {
	ServerAddress string
}

func (*CaptureCMD) Name() string     { return "capture" }
func (*CaptureCMD) Synopsis() string { return locale.Loc("capture_synopsis", nil) }
func (c *CaptureCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, "address", "", "remote server address")
}

func (c *CaptureCMD) Execute(ctx context.Context, ui utils.UI) error {
	address, hostname, err := utils.ServerInput(ctx, c.ServerAddress)
	if err != nil {
		return err
	}

	os.Mkdir("captures", 0o775)
	fio, err := os.Create("captures/" + hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap2")
	if err != nil {
		return err
	}
	defer fio.Close()
	utils.WriteReplayHeader(fio)

	proxy, err := utils.NewProxy()
	if err != nil {
		logrus.Fatal(err)
	}
	proxy.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
		IsfromClient := src.String() == proxy.Client.LocalAddr().String()

		buf := bytes.NewBuffer(nil)
		header.Write(buf)
		buf.Write(payload)
		dumpPacket(fio, IsfromClient, buf.Bytes())
	}

	err = proxy.Run(ctx, address)
	time.Sleep(2 * time.Second)
	if err != nil {
		return err
	}
	return nil
}
