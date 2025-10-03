package proxy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type ProxySettings struct {
	ConnectInfo *connectinfo.ConnectInfo `opt:"Address" flag:"address" desc:"locale.remote_address"`

	Debug         bool   `opt:"Debug" flag:"debug" desc:"locale.debug_mode"`
	ExtraDebug    bool   `opt:"Extra Debug" flag:"extra-debug" desc:"extra debug info (packet.log)"`
	Capture       bool   `opt:"Packet Capture" flag:"capture" desc:"Capture pcap2 file"`
	ClientCache   bool   `opt:"Client Cache" flag:"client-cache" default:"true" desc:"Enable Client Cache"`
	ListenAddress string `opt:"Listen Address" flag:"listen" default:"0.0.0.0:19132" desc:"example :19132 or 127.0.0.1:19132"`
}

type PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time)
type ingameCommand struct {
	Exec func(cmdline []string) bool
	Cmd  protocol.Command
}

var NewPacketCapturer func() *Handler

var errCancelConnect = fmt.Errorf("cancelled connecting")

var serverPool = packet.NewServerPool()
var clientPool = packet.NewClientPool()

func DecodePacket(header packet.Header, payload []byte, shieldID int32) (pk packet.Packet, ok bool) {
	pkFunc, ok := serverPool[header.PacketID]
	if !ok {
		pkFunc, ok = clientPool[header.PacketID]
	}
	if ok {
		pk = pkFunc()
	} else {
		pk = &packet.Unknown{PacketID: header.PacketID, Payload: payload}
	}

	ok = true
	defer func() {
		if recoveredErr := recover(); recoveredErr != nil {
			logrus.Errorf("%T: %s", pk, recoveredErr.(error))
			logrus.Debugf("payload: %s", hex.EncodeToString(payload))
			ok = false
		}
	}()
	pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), shieldID, false))
	return pk, ok
}
