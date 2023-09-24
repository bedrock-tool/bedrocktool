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
	Name     string
	ProxyRef func(pc *Context)
	//
	AddressAndName func(address, hostname string) error

	// called to change game data
	ToClientGameDataModifier func(gd *minecraft.GameData)

	// Called with raw packet data
	PacketRaw func(header packet.Header, payload []byte, src, dst net.Addr)

	// called on every packet after login
	PacketCB func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error)

	// called after client connected
	OnClientConnect func(conn minecraft.IConn)
	//SecondaryClientCB func(conn minecraft.IConn)

	// called after server connected & downloaded resource packs
	OnServerConnect func() (cancel bool, err error)
	// called after game started
	ConnectCB func() bool

	// called when the proxy session stops or is reconnected
	OnEnd func()
	// called when the proxy ends
	Deferred func()
}

var NewPacketCapturer func() *Handler

var errCancelConnect = fmt.Errorf("cancelled connecting")

var serverPool = packet.NewServerPool()
var clientPool = packet.NewClientPool()

func DecodePacket(header packet.Header, payload []byte) (pk packet.Packet, ok bool) {
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
	pk.Marshal(protocol.NewReader(bytes.NewBuffer(payload), 0, false))
	return pk, ok
}
