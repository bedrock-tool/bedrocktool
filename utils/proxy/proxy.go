package proxy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type ProxySettings struct {
	ConnectInfo *connectinfo.ConnectInfo `opt:"Address" flag:"address" desc:"locale.remote_address"`

	Debug         bool   `opt:"Debug" flag:"debug" desc:"locale.debug_mode"`
	ExtraDebug    bool   `opt:"Extra Debug" flag:"extra-debug" desc:"Also log every packet to packet.log (large)"`
	Capture       bool   `opt:"Record Session" flag:"capture" default:"true" desc:"Save a .pcap2 recording you can replay later"`
	ClientCache   bool   `opt:"Client Cache" flag:"client-cache" default:"true" desc:"Let Minecraft cache chunks, so they load faster"`
	ListenAddress string `opt:"Listen Address" flag:"listen" default:"0.0.0.0:19132" desc:"Where Minecraft joins the proxy, e.g. :19133 for another port"`
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
			if utils.IsDebug() {
				panic(recoveredErr)
			}
		}
	}()
	pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), shieldID, false))
	return pk, ok
}
