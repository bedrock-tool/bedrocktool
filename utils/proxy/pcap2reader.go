package proxy

import (
	"archive/zip"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"time"

	"sync/atomic"

	"github.com/klauspost/compress/s2"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type Pcap2Reader struct {
	f                 *os.File
	packetsOffset     int64
	Version           uint32
	packetsReader     io.Reader
	ResourcePacks     *replayCache
	packetOffsetIndex []int64
	CurrentPacket     int

	pool       packet.Pool
	protocol   minecraft.Protocol
	packetFunc PacketFunc
	shieldID   *atomic.Int32
}

func NewPcap2Reader(f *os.File, packetFunc PacketFunc, ShieldID *atomic.Int32) (*Pcap2Reader, error) {
	var head = make([]byte, 16)
	f.Read(head)
	magic := string(head[0:4])
	if magic != "BTCP" {
		return nil, errors.New("unsupported old format")
	}

	ver := binary.LittleEndian.Uint32(head[4:8])
	zipSize := binary.LittleEndian.Uint64(head[8:16])
	z, err := zip.NewReader(io.NewSectionReader(f, 16, int64(zipSize)), int64(zipSize))
	if err != nil {
		return nil, err
	}

	// read all packs
	cache := &replayCache{}
	err = cache.ReadFrom(z)
	if err != nil {
		return nil, err
	}

	var packetsOffset int64
	var packetReader io.ReadCloser
	if ver < 4 {
		return nil, errors.New("version < 4 no longer supported")
	} else if ver < 5 {
		f.Seek(int64(zipSize+16), 0)
		packetReader = flate.NewReader(f)
	} else {
		packetsOffset, _ = f.Seek(int64(zipSize+16), 0)
		packetReader = f
	}

	pool := minecraft.DefaultProtocol.Packets(true)
	maps.Copy(pool, minecraft.DefaultProtocol.Packets(false))

	return &Pcap2Reader{
		f:             f,
		packetsOffset: packetsOffset,
		Version:       ver,
		packetsReader: packetReader,
		ResourcePacks: cache,
		pool:          pool,
		protocol:      minecraft.DefaultProtocol,
		packetFunc:    packetFunc,
		shieldID:      ShieldID,
	}, nil
}

func (r *Pcap2Reader) ReadPacket(skip bool) (packet packet.Packet, toServer bool, receivedTime time.Time, err error) {
	var magic uint32
	var magic2 uint32
	var packetLength uint32
	receivedTime = time.Now()

	// add where this is to index
	if len(r.packetOffsetIndex) < r.CurrentPacket && r.Version >= 5 {
		off, _ := r.f.Seek(0, 1)
		r.packetOffsetIndex = append(r.packetOffsetIndex, off)
	}
	r.CurrentPacket++

	err = binary.Read(r.packetsReader, binary.LittleEndian, &magic)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		if errors.Is(err, io.EOF) {
			logrus.Info("Reached End")
			return nil, false, receivedTime, net.ErrClosed
		}
		return nil, false, receivedTime, err
	}
	if magic != 0xAAAAAAAA {
		return nil, toServer, receivedTime, fmt.Errorf("wrong Magic")
	}
	binary.Read(r.packetsReader, binary.LittleEndian, &packetLength)
	binary.Read(r.packetsReader, binary.LittleEndian, &toServer)
	var timeMs int64
	binary.Read(r.packetsReader, binary.LittleEndian, &timeMs)
	receivedTime = time.UnixMilli(timeMs)

	if skip {
		_, err := io.CopyN(io.Discard, r.packetsReader, int64(packetLength))
		if err != nil {
			return nil, toServer, receivedTime, err
		}
		return nil, false, receivedTime, nil
	}

	payload := make([]byte, packetLength)
	n, err := io.ReadFull(r.packetsReader, payload)
	if err != nil {
		return nil, toServer, receivedTime, err
	}
	if n != int(packetLength) {
		return nil, toServer, receivedTime, errors.New("truncated")
	}

	binary.Read(r.packetsReader, binary.LittleEndian, &magic2)
	if magic2 != 0xBBBBBBBB {
		return nil, toServer, receivedTime, errors.New("wrong Magic2")
	}

	// version 5 compresses payloads seperately
	if r.Version >= 5 {
		payload, err = s2.Decode(nil, payload)
		if err != nil {
			return nil, toServer, receivedTime, err
		}
	}

	var src, dst = replayRemoteAddr, replayLocalAddr
	if toServer {
		src, dst = replayLocalAddr, replayRemoteAddr
	}
	pkData, err := minecraft.ParseData(payload, r.packetFunc, src, dst)
	if err != nil {
		return nil, toServer, receivedTime, err
	}
	pks, err := pkData.Decode(r.pool, r.protocol, nil, false, false, r.shieldID.Load())
	if err != nil {
		return nil, toServer, receivedTime, err
	}
	pk := pks[0]

	return pk, toServer, receivedTime, nil
}

func (r *Pcap2Reader) Seek(packet int) error {
	if r.Version < 5 {
		return errors.New("capture version < 5 cannot seek")
	}
	diff := packet - r.CurrentPacket
	if diff == 0 {
		return nil
	} else if diff > 0 {
		for i := 0; i < diff; i++ {
			_, _, _, err := r.ReadPacket(true)
			if err != nil {
				return err
			}
		}
	} else if diff < 0 {
		off := r.packetOffsetIndex[packet]
		_, err := r.f.Seek(off+r.packetsOffset, 0)
		if err != nil {
			return err
		}
		r.CurrentPacket = packet
		return nil
	}
	return nil
}
