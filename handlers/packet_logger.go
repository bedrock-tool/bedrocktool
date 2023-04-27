package handlers

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"reflect"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/crypt"
	"github.com/fatih/color"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var MutedPackets = []string{
	"packet.UpdateBlock",
	"packet.MoveActorAbsolute",
	"packet.SetActorMotion",
	"packet.SetTime",
	"packet.RemoveActor",
	"packet.AddActor",
	"packet.UpdateAttributes",
	"packet.Interact",
	"packet.LevelEvent",
	"packet.SetActorData",
	"packet.MoveActorDelta",
	"packet.MovePlayer",
	"packet.BlockActorData",
	"packet.PlayerAuthInput",
	"packet.LevelChunk",
	"packet.LevelSoundEvent",
	"packet.ActorEvent",
	"packet.NetworkChunkPublisherUpdate",
	"packet.UpdateSubChunkBlocks",
	"packet.SubChunk",
	"packet.SubChunkRequest",
	"packet.Animate",
	"packet.NetworkStackLatency",
	"packet.InventoryTransaction",
	"packet.PlaySound",
}

var dirS2C = color.GreenString("S") + "->" + color.CyanString("C")
var dirC2S = color.CyanString("C") + "->" + color.GreenString("S")
var pool = packet.NewPool()

func NewDebugLogger(extraVerbose bool) *utils.ProxyHandler {
	var logPlain, logCrypt, logCryptEnc io.WriteCloser
	var packetsLogF *bufio.Writer
	var dmpLock sync.Mutex

	if extraVerbose {
		// open plain text log
		logPlain, err := os.Create("packets.log")
		if err != nil {
			logrus.Error(err)
		}
		// open gpg log
		logCryptEnc, err = crypt.Encer("packets.log.gpg")
		if err != nil {
			logrus.Error(err)
		}
		if logPlain != nil || logCryptEnc != nil {
			packetsLogF = bufio.NewWriter(io.MultiWriter(logPlain, logCryptEnc))
		}
	}

	var proxy *utils.ProxyContext

	return &utils.ProxyHandler{
		Name: "Debug",
		ProxyRef: func(pc *utils.ProxyContext) {
			proxy = pc
		},
		PacketFunc: func(header packet.Header, payload []byte, src, dst net.Addr) {
			var pk packet.Packet
			if pkFunc, ok := pool[header.PacketID]; ok {
				pk = pkFunc()
			} else {
				pk = &packet.Unknown{PacketID: header.PacketID, Payload: payload}
			}

			defer func() {
				if recoveredErr := recover(); recoveredErr != nil {
					logrus.Errorf("%T: %s", pk, recoveredErr.(error))
				}
			}()
			pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), 0))

			if packetsLogF != nil {
				dmpLock.Lock()
				packetsLogF.Write([]byte(utils.DumpStruct(0, pk, true, false) + "\n\n\n"))
				dmpLock.Unlock()
			}

			pkName := reflect.TypeOf(pk).String()[1:]
			if !slices.Contains(MutedPackets, pkName) {
				var dir string = dirS2C
				if proxy.IsClient(src) {
					dir = dirC2S
				}
				logrus.Debugf("%s 0x%02x, %s", dir, pk.ID(), pkName)
			}
		},
		OnEnd: func() {
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
		},
	}
}

func init() {
	// hacky solution to allow proxy to add this
	utils.NewDebugLogger = NewDebugLogger
}
