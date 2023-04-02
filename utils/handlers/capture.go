package handlers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

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

type PacketCapturer struct {
	proxy *utils.ProxyContext
	fio   *os.File
}

func (p *PacketCapturer) AddressAndName(address, hostname string) error {
	os.Mkdir("captures", 0o775)
	fio, err := os.Create(fmt.Sprintf("captures/%s-%s.pcap2", hostname, time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		return err
	}
	utils.WriteReplayHeader(fio)
	p.fio = fio
	return nil
}

func (p *PacketCapturer) PacketFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	IsfromClient := utils.ClientAddr.String() == src.String()

	buf := bytes.NewBuffer(nil)
	header.Write(buf)
	buf.Write(payload)
	dumpPacket(p.fio, IsfromClient, buf.Bytes())
}

func NewPacketCapturer() *utils.ProxyHandler {
	p := &PacketCapturer{}
	return &utils.ProxyHandler{
		Name: "Packet Capturer",
		ProxyRef: func(pc *utils.ProxyContext) {
			p.proxy = pc
		},
		PacketFunc:     p.PacketFunc,
		AddressAndName: p.AddressAndName,
		OnEnd: func() {
			p.fio.Close()
		},
	}
}
