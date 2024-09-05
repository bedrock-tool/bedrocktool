package proxy

import (
	"bufio"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/fatih/color"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var mutedPackets = []uint32{
	packet.IDUpdateBlock,
	packet.IDMoveActorAbsolute,
	packet.IDSetActorMotion,
	packet.IDSetTime,
	packet.IDRemoveActor,
	packet.IDAddActor,
	packet.IDUpdateAttributes,
	packet.IDInteract,
	packet.IDLevelEvent,
	packet.IDSetActorData,
	packet.IDMoveActorDelta,
	packet.IDMovePlayer,
	packet.IDBlockActorData,
	packet.IDPlayerAuthInput,
	packet.IDLevelChunk,
	packet.IDLevelSoundEvent,
	packet.IDActorEvent,
	packet.IDNetworkChunkPublisherUpdate,
	packet.IDUpdateSubChunkBlocks,
	packet.IDSubChunk,
	packet.IDSubChunkRequest,
	packet.IDAnimate,
	packet.IDNetworkStackLatency,
	packet.IDInventoryTransaction,
	packet.IDPlaySound,
	packet.IDPlayerAction,
	packet.IDSetTitle,
	packet.IDClientCacheMissResponse,
	packet.IDClientCacheBlobStatus,
}

var dirS2C = color.GreenString("S") + "->" + color.CyanString("C")
var dirC2S = color.CyanString("C") + "->" + color.GreenString("S")

type packetLogger struct {
	dumpLock        sync.Mutex
	timeStart       time.Time
	packetLogWriter *bufio.Writer
	closePacketLog  func() error
	clientSide      bool
}

func (p *packetLogger) PacketSend(pk packet.Packet, t time.Time) error {
	p.dumpLock.Lock()
	defer p.dumpLock.Unlock()
	return p.logPacket(pk, t, false)
}

func (p *packetLogger) PacketReceive(pk packet.Packet, t time.Time) error {
	p.dumpLock.Lock()
	defer p.dumpLock.Unlock()
	return p.logPacket(pk, t, false)
}

func (p *packetLogger) logPacket(pk packet.Packet, t time.Time, toServer bool) error {
	if p.packetLogWriter != nil {
		if p.timeStart.IsZero() {
			p.timeStart = t
		}
		p.packetLogWriter.WriteString(t.Sub(p.timeStart).Truncate(time.Millisecond).String() + "\n")
		utils.DumpStruct(p.packetLogWriter, pk)
		p.packetLogWriter.Write([]byte("\n\n\n"))
		p.packetLogWriter.Flush()
	}

	if !p.clientSide && !slices.Contains(mutedPackets, pk.ID()) {
		var dir string = dirS2C
		if toServer {
			dir = dirC2S
		}
		pkName := reflect.TypeOf(pk).String()[8:]
		logrus.Debugf("%s %s", dir, pkName)
	}
	return nil
}

func (p *packetLogger) Close() error {
	if p.packetLogWriter != nil {
		p.packetLogWriter.Flush()
		return p.closePacketLog()
	}
	return nil
}

func NewPacketLogger(verbose, clientSide bool) (*packetLogger, error) {
	p := &packetLogger{
		clientSide: clientSide,
	}
	if verbose {
		var logName = "packets.log"
		if clientSide {
			logName = "packets-client.log"
		}
		f, err := os.Create(logName)
		if err != nil {
			return nil, err
		}
		p.packetLogWriter = bufio.NewWriter(f)
		p.closePacketLog = f.Close
	}
	return p, nil
}
