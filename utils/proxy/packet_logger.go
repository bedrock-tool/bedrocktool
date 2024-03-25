package proxy

import (
	"bufio"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/crypt"
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
}

var dirS2C = color.GreenString("S") + "->" + color.CyanString("C")
var dirC2S = color.CyanString("C") + "->" + color.GreenString("S")

func newExtraDebug(name string) (func(pk packet.Packet), func()) {
	var logPlain, logCrypt, logCryptEnc io.WriteCloser
	var packetsLogF *bufio.Writer
	var dmpLock sync.Mutex

	// open plain text log
	logPlain, err := os.Create(name)
	if err != nil {
		logrus.Error(err)
	}
	// open gpg log
	logCryptEnc, err = crypt.Encer(name + ".gpg")
	if err != nil {
		logrus.Error(err)
	}
	packetsLogF = bufio.NewWriter(io.MultiWriter(logPlain, logCryptEnc))

	return func(pk packet.Packet) {
			dmpLock.Lock()
			utils.DumpStruct(packetsLogF, pk)
			packetsLogF.Write([]byte("\n\n\n"))
			packetsLogF.Flush()
			dmpLock.Unlock()
		}, func() {
			dmpLock.Lock()
			if packetsLogF != nil {
				packetsLogF.Flush()
			}
			if logPlain != nil {
				logPlain.Close()
			}
			if logCryptEnc != nil {
				logCryptEnc.Close()
			}
			if logCrypt != nil {
				logCrypt.Close()
			}
			dmpLock.Unlock()
		}
}

func NewDebugLogger(extraVerbose bool) *Handler {
	var extraDebug func(pk packet.Packet)
	var extraDebugEnd func()
	if extraVerbose {
		extraDebug, extraDebugEnd = newExtraDebug("packets.log")
	}

	return &Handler{
		Name: "Debug",
		PacketCB: func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
			if extraDebug != nil {
				extraDebug(pk)
			}

			if !slices.Contains(mutedPackets, pk.ID()) {
				var dir string = dirS2C
				if toServer {
					dir = dirC2S
				}
				pkName := reflect.TypeOf(pk).String()[1:]
				logrus.Debugf("%s 0x%02x, %s", dir, pk.ID(), pkName)
			}
			return pk, nil
		},
		Deferred: func() {
			if extraDebugEnd != nil {
				extraDebugEnd()
			}
		},
	}
}
