package proxy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr)
type ingameCommand struct {
	Exec func(cmdline []string) bool
	Cmd  protocol.Command
}

type Handler struct {
	Name string

	ProxyReference     func(c *Context)
	GameDataModifier   func(gameData *minecraft.GameData)
	OnAddressAndName   func(address, hostname string) error
	FilterResourcePack func(id string) bool

	PacketRaw      func(header packet.Header, payload []byte, src, dst net.Addr)
	PacketCallback func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error)

	OnClientConnect func()
	OnServerConnect func() (cancel bool, err error)
	OnConnect       func() (cancel bool)

	OnSessionEnd func()
	OnProxyEnd   func()
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
