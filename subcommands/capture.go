package subcommands

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/google/subcommands"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

var (
	SrcIp_client = net.IPv4(127, 0, 0, 1)
	SrcIp_server = net.IPv4(243, 0, 0, 2)
)

func init() {
	utils.RegisterCommand(&CaptureCMD{})
}

func dump_packet(w *pcapgo.Writer, from_client bool, pk packet.Header, payload []byte) {
	var err error
	var prefix []byte
	if from_client {
		prefix = []byte("client")
	} else {
		prefix = []byte("server")
	}

	packet_data := bytes.NewBuffer(nil)
	pk.Write(packet_data)
	packet_data.Write(payload)

	serialize_buf := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(
		serialize_buf,
		gopacket.SerializeOptions{},
		gopacket.Payload(prefix),
		gopacket.Payload(packet_data.Bytes()),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = w.WritePacket(gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		Length:         len(serialize_buf.Bytes()),
		CaptureLength:  len(serialize_buf.Bytes()),
		InterfaceIndex: 1,
	}, serialize_buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fio, err := os.Create(hostname + "-" + time.Now().Format("2006-01-02_15-04-05") + ".pcap")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer fio.Close()
	w := pcapgo.NewWriter(fio)
	w.WriteFileHeader(65536, layers.LinkTypeEthernet)

	proxy := utils.NewProxy(logrus.StandardLogger())
	proxy.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
		from_client := src.String() == proxy.Client.LocalAddr().String()
		dump_packet(w, from_client, header, payload)
	}

	err = proxy.Run(ctx, address)
	if err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}
