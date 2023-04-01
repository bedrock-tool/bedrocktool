package subcommands

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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
	f.Write(payload)
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
	fio, err := os.Create(fmt.Sprintf("captures/%s-%s.pcap2", hostname, time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		return err
	}
	defer fio.Close()
	utils.WriteReplayHeader(fio)

	proxy, err := utils.NewProxy()
	if err != nil {
		return err
	}
	proxy.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
		IsfromClient := src.String() == proxy.Client.LocalAddr().String()

		buf := bytes.NewBuffer(nil)
		header.Write(buf)
		buf.Write(payload)
		dumpPacket(fio, IsfromClient, buf.Bytes())
	}

	return proxy.Run(ctx, address)
}
