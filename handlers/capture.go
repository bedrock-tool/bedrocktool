package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (p *packetCapturer) dumpPacket(toServer bool, payload []byte) {
	p.dumpLock.Lock()
	p.wPacket.Write([]byte{0xAA, 0xAA, 0xAA, 0xAA})
	packetSize := uint32(len(payload))
	binary.Write(p.wPacket, binary.LittleEndian, packetSize)
	binary.Write(p.wPacket, binary.LittleEndian, toServer)
	binary.Write(p.wPacket, binary.LittleEndian, time.Now().UnixMilli())
	p.wPacket.Write(payload)
	p.wPacket.Write([]byte{0xBB, 0xBB, 0xBB, 0xBB})
	p.dumpLock.Unlock()
}

type packetCapturer struct {
	proxy    *proxy.Context
	file     *os.File
	zip      *zip.Writer
	wPacket  io.Writer
	tempBuf  *bytes.Buffer
	dumpLock sync.Mutex
}

func (p *packetCapturer) AddressAndName(address, hostname string) (err error) {
	os.Mkdir("captures", 0o775)
	p.file, err = os.Create(fmt.Sprintf("captures/%s-%s.pcap2", hostname, time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		return err
	}
	p.zip = zip.NewWriter(p.file)
	if err != nil {
		return err
	}

	{
		f, err := p.zip.Create("version")
		if err != nil {
			return err
		}
		binary.Write(f, binary.LittleEndian, uint32(3))
	}

	// temporary buffer
	p.tempBuf = bytes.NewBuffer(nil)
	p.wPacket = p.tempBuf

	return nil
}

func (p *packetCapturer) OnServerConnect() (bool, error) {
	packs := p.proxy.Server.ResourcePacks()
	select {
	case <-p.proxy.Server.OnDisconnect():
		_, err := p.proxy.Server.ReadPacket()
		return true, err
	default:
	}

	written := make(map[string]bool)
	for _, pack := range packs {
		filename := filepath.Join("packcache", pack.UUID()+"_"+pack.Version()+".zip")
		if _, ok := written[filename]; ok {
			continue
		}
		logrus.Debugf("Writing %s to capture", pack.Name())
		f, err := p.zip.CreateHeader(&zip.FileHeader{
			Name:   filename,
			Method: zip.Store,
		})
		if err != nil {
			panic(err)
		}
		pack.WriteTo(f)
		pack.Seek(0, 0)
		written[filename] = true
	}

	// create the packets.bin file and dump already received packets into it
	p.dumpLock.Lock()
	// DO NOT OPEN ANY FILES IN THE ZIP AFTER THIS
	f, err := p.zip.Create("packets.bin")
	if err != nil {
		panic(err)
	}
	_, err = p.tempBuf.WriteTo(f)
	if err != nil {
		panic(err)
	}
	p.tempBuf = nil
	p.wPacket = f
	p.dumpLock.Unlock()
	return false, nil
}

func (p *packetCapturer) PacketFunc(header packet.Header, payload []byte, src, dst net.Addr) {
	buf := bytes.NewBuffer(nil)
	header.Write(buf)
	buf.Write(payload)
	p.dumpPacket(p.proxy.IsClient(src), buf.Bytes())
}

func NewPacketCapturer() *proxy.Handler {
	p := &packetCapturer{}
	return &proxy.Handler{
		Name: "Packet Capturer",
		ProxyRef: func(pc *proxy.Context) {
			p.proxy = pc
		},
		AddressAndName:  p.AddressAndName,
		OnServerConnect: p.OnServerConnect,
		PacketRaw:       p.PacketFunc,
		OnEnd: func() {
			p.dumpLock.Lock()
			defer p.dumpLock.Unlock()
			if p.zip != nil {
				err := p.zip.Close()
				if err != nil {
					logrus.Error(err)
				}
				p.file.Close()
			}
		},
	}
}

func init() {
	proxy.NewPacketCapturer = NewPacketCapturer
}
