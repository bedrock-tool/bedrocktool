package proxy

import (
	"net"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Handlers []*Handler

type Handler struct {
	Name string

	SessionStart       func(s *Session, serverName string) error
	GameDataModifier   func(gameData *minecraft.GameData)
	FilterResourcePack func(id string) bool
	OnFinishedPack     func(pack resource.Pack) error

	PacketRaw      func(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time)
	PacketCallback func(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error)

	OnServerConnect func() (cancel bool, err error)
	OnConnect       func() (cancel bool)

	OnSessionEnd func()
	OnProxyEnd   func()
}

func (h *Handlers) SessionStart(s *Session, serverName string) error {
	for _, handler := range *h {
		if handler.SessionStart == nil {
			continue
		}
		err := handler.SessionStart(s, serverName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handlers) GameDataModifier(gameData *minecraft.GameData) {
	for _, handler := range *h {
		if handler.GameDataModifier == nil {
			continue
		}
		handler.GameDataModifier(gameData)
	}
}

func (h *Handlers) FilterResourcePack(id string) bool {
	for _, handler := range *h {
		if handler.FilterResourcePack == nil {
			continue
		}
		if handler.FilterResourcePack(id) {
			return true
		}
	}
	return false
}

func (h *Handlers) OnFinishedPack(pack resource.Pack) error {
	for _, handler := range *h {
		if handler.OnFinishedPack == nil {
			continue
		}
		err := handler.OnFinishedPack(pack)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handlers) PacketRaw(header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
	for _, handler := range *h {
		if handler.PacketRaw == nil {
			continue
		}
		handler.PacketRaw(header, payload, src, dst, timeReceived)
	}
}
func (h *Handlers) PacketCallback(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	var err error
	for _, handler := range *h {
		if handler.PacketCallback == nil {
			continue
		}
		pk, err = handler.PacketCallback(pk, toServer, timeReceived, preLogin)
		if err != nil {
			return nil, err
		}
		if pk == nil {
			return nil, nil
		}
	}
	return pk, nil
}

func (h *Handlers) OnServerConnect() (cancel bool, err error) {
	for _, handler := range *h {
		if handler.OnServerConnect == nil {
			continue
		}
		cancel, err = handler.OnServerConnect()
		if err != nil {
			return false, err
		}
		if cancel {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handlers) OnConnect() (cancel bool) {
	for _, handler := range *h {
		if handler.OnConnect == nil {
			continue
		}
		if handler.OnConnect() {
			return true
		}
	}
	return false
}

func (h *Handlers) OnSessionEnd() {
	for _, handler := range *h {
		if handler.OnSessionEnd == nil {
			continue
		}
		handler.OnSessionEnd()
	}
}

func (h *Handlers) OnProxyEnd() {
	for _, handler := range *h {
		if handler.OnProxyEnd == nil {
			continue
		}
		handler.OnProxyEnd()
	}
}
