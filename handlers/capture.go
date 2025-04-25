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

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/klauspost/compress/s2"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func (p *packetCapturer) dumpPacket(toServer bool, payload []byte, timeReceived time.Time) {
	p.dumpLock.Lock()
	payloadCompressed := s2.EncodeBetter(nil, payload)

	var buf []byte = []byte{0xAA, 0xAA, 0xAA, 0xAA}
	packetSize := uint32(len(payloadCompressed))
	buf = binary.LittleEndian.AppendUint32(buf, packetSize)
	if toServer {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	buf = binary.LittleEndian.AppendUint64(buf, uint64(timeReceived.UnixMilli()))
	buf = append(buf, payloadCompressed...)
	buf = append(buf, []byte{0xBB, 0xBB, 0xBB, 0xBB}...)
	p.wPacket.Write(buf)
	p.dumpLock.Unlock()
}

type packetCapturer struct {
	file     *os.File
	wPacket  io.Writer
	tempBuf  *bytes.Buffer
	dumpLock sync.Mutex
	hostname string
	log      *logrus.Entry
}

func (p *packetCapturer) onServerName(hostname string) (err error) {
	p.hostname = hostname
	// temporary buffer
	p.tempBuf = bytes.NewBuffer(nil)
	p.wPacket = p.tempBuf
	return nil
}

func (p *packetCapturer) OnServerConnect(s *proxy.Session) (disconnect bool, err error) {
	os.Mkdir(utils.PathData("captures"), 0o775)
	captureName := fmt.Sprintf("%s-%s.pcap2", p.hostname, time.Now().Format("2006-01-02_15-04-05"))
	capturePath := utils.PathData("captures", captureName)
	p.file, err = os.Create(capturePath)
	if err != nil {
		return false, err
	}

	p.file.WriteString("BTCP")
	binary.Write(p.file, binary.LittleEndian, uint32(5))
	binary.Write(p.file, binary.LittleEndian, uint64(0))

	z := zip.NewWriter(p.file)
	z.SetOffset(16)

	written := make(map[string]bool)
	packs := s.Server.ResourcePacks()
	for _, pack := range packs {
		filename := filepath.Join("packcache", pack.UUID().String()+"_"+pack.Version()+".zip")
		if _, ok := written[filename]; ok {
			continue
		}
		p.log.Debugf("Writing %s to capture", pack.Name())
		f, err := z.CreateHeader(&zip.FileHeader{
			Name:   filename,
			Method: zip.Store,
		})
		if err != nil {
			panic(err)
		}
		_, err = pack.WriteTo(f)
		if err != nil {
			panic(err)
		}
		written[filename] = true
	}
	err = z.Close()
	if err != nil {
		return false, err
	}

	// write size of zip
	endZip, _ := p.file.Seek(0, 1)
	p.file.Seek(8, 0)
	binary.Write(p.file, binary.LittleEndian, uint64(endZip-16))
	p.file.Seek(endZip, 0)

	p.dumpLock.Lock()
	_, err = p.tempBuf.WriteTo(p.file)
	if err != nil {
		return false, err
	}
	p.tempBuf = nil
	p.wPacket = p.file
	p.dumpLock.Unlock()
	return false, nil
}

func (p *packetCapturer) PacketFunc(s *proxy.Session, header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
	if header.PacketID == packet.IDResourcePackChunkData || header.PacketID == packet.IDResourcePackChunkRequest || header.PacketID == packet.IDResourcePackDataInfo {
		return
	}

	buf := bytes.NewBuffer(nil)
	header.Write(buf)
	buf.Write(payload)
	p.dumpPacket(s.IsClient(src), buf.Bytes(), timeReceived)
}

func NewPacketCapturer() *proxy.Handler {
	p := &packetCapturer{
		log: logrus.WithField("part", "PacketCapture"),
	}
	return &proxy.Handler{
		Name: "Packet Capturer",
		SessionStart: func(s *proxy.Session, serverName string) error {
			return p.onServerName(serverName)
		},
		OnServerConnect: p.OnServerConnect,
		PacketRaw:       p.PacketFunc,
		OnSessionEnd: func(s *proxy.Session, wg *sync.WaitGroup) {
			p.dumpLock.Lock()
			defer p.dumpLock.Unlock()
			if p.file != nil {
				p.file.Close()
			}
		},
		OnBlobs: func(s *proxy.Session, blobs []protocol.CacheBlob) {
			var pk packet.ClientCacheMissResponse
			for _, blob := range blobs {
				pk.Blobs = append(pk.Blobs, protocol.CacheBlob{
					Hash:    blob.Hash,
					Payload: blob.Payload,
				})
			}
			buf := bytes.NewBuffer(nil)
			head := packet.Header{
				PacketID: packet.IDClientCacheMissResponse,
			}
			head.Write(buf)
			io := protocol.NewWriter(buf, 0)
			pk.Marshal(io)
			p.dumpPacket(false, buf.Bytes(), time.Now())
		},
	}
}

func init() {
	proxy.NewPacketCapturer = NewPacketCapturer
}
