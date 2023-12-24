package handlers

import (
	"archive/zip"
	"bytes"
	"compress/flate"
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
	if p.fw != nil {
		p.fw.Flush()
	}
	p.dumpLock.Unlock()
}

type packetCapturer struct {
	proxy    *proxy.Context
	file     *os.File
	wPacket  io.Writer
	fw       *flate.Writer
	tempBuf  *bytes.Buffer
	dumpLock sync.Mutex
	hostname string
}

func (p *packetCapturer) AddressAndName(address, hostname string) (err error) {
	p.hostname = hostname
	// temporary buffer
	p.tempBuf = bytes.NewBuffer(nil)
	p.wPacket = p.tempBuf
	return nil
}

func (p *packetCapturer) OnServerConnect() (disconnect bool, err error) {
	os.Mkdir("captures", 0o775)
	p.file, err = os.Create(fmt.Sprintf("captures/%s-%s.pcap2", p.hostname, time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		return false, err
	}

	packs := p.proxy.Server.ResourcePacks()
	select {
	case <-p.proxy.Server.OnDisconnect():
		_, err := p.proxy.Server.ReadPacket()
		return true, err
	default:
	}

	p.file.WriteString("BTCP")
	binary.Write(p.file, binary.LittleEndian, uint32(4))
	binary.Write(p.file, binary.LittleEndian, uint64(0))

	{
		z := zip.NewWriter(p.file)
		if err != nil {
			return false, err
		}
		z.SetOffset(16)

		written := make(map[string]bool)
		for _, pack := range packs {
			filename := filepath.Join("packcache", pack.UUID()+"_"+pack.Version()+".zip")
			if _, ok := written[filename]; ok {
				continue
			}
			logrus.Debugf("Writing %s to capture", pack.Name())
			f, err := z.CreateHeader(&zip.FileHeader{
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
		err = z.Close()
		if err != nil {
			return false, err
		}
	}
	// write size of zip
	endZip, _ := p.file.Seek(0, 1)
	p.file.Seek(8, 0)
	binary.Write(p.file, binary.LittleEndian, uint64(endZip-16))
	p.file.Seek(endZip, 0)

	p.dumpLock.Lock()
	fw, err := flate.NewWriter(p.file, 4)
	if err != nil {
		return false, err
	}

	_, err = p.tempBuf.WriteTo(fw)
	if err != nil {
		panic(err)
	}
	p.tempBuf = nil
	p.wPacket = fw
	p.fw = fw
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
			if p.file != nil {
				p.fw.Close()
				p.file.Close()
			}
		},
	}
}

func init() {
	proxy.NewPacketCapturer = NewPacketCapturer
}
