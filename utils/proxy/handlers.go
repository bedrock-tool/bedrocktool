package proxy

import (
	"net"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Handlers []*Handler

type Handler struct {
	Name string

	SessionStart       func(s *Session, serverName string) error
	PlayerDataModifier func(s *Session, identity *login.IdentityData, data *login.ClientData)
	GameDataModifier   func(s *Session, gameData *minecraft.GameData)
	FilterResourcePack func(s *Session, id string) bool
	OnFinishedPack     func(s *Session, pack resource.Pack) error

	PacketRaw      func(s *Session, header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time)
	PacketCallback func(s *Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error)

	OnServerConnect func(s *Session) (cancel bool, err error)
	OnConnect       func(s *Session) (cancel bool)

	OnSessionEnd func(s *Session, wg *sync.WaitGroup)
	OnBlobs      func(s *Session, blobs []protocol.CacheBlob)

	OnPlayerMove func(s *Session)
}

func (h Handlers) SessionStart(s *Session, serverName string) error {
	for _, handler := range h {
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

func (h Handlers) GameDataModifier(s *Session, gameData *minecraft.GameData) {
	for _, handler := range h {
		if handler.GameDataModifier == nil {
			continue
		}
		handler.GameDataModifier(s, gameData)
	}
}

func (h Handlers) PlayerDataModifier(s *Session, identity *login.IdentityData, data *login.ClientData) {
	for _, handler := range h {
		if handler.PlayerDataModifier == nil {
			continue
		}
		handler.PlayerDataModifier(s, identity, data)
	}
}

func (h Handlers) FilterResourcePack(s *Session, id string) bool {
	for _, handler := range h {
		if handler.FilterResourcePack == nil {
			continue
		}
		if handler.FilterResourcePack(s, id) {
			return true
		}
	}
	return false
}

func (h Handlers) OnFinishedPack(s *Session, pack resource.Pack) error {
	for _, handler := range h {
		if handler.OnFinishedPack == nil {
			continue
		}
		err := handler.OnFinishedPack(s, pack)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h Handlers) PacketRaw(s *Session, header packet.Header, payload []byte, src, dst net.Addr, timeReceived time.Time) {
	for _, handler := range h {
		if handler.PacketRaw == nil {
			continue
		}
		handler.PacketRaw(s, header, payload, src, dst, timeReceived)
	}
}
func (h Handlers) PacketCallback(s *Session, pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	var err error
	for _, handler := range h {
		if handler.PacketCallback == nil {
			continue
		}
		pk, err = handler.PacketCallback(s, pk, toServer, timeReceived, preLogin)
		if err != nil {
			return nil, err
		}
		if pk == nil {
			return nil, nil
		}
	}
	return pk, nil
}

func (h Handlers) OnServerConnect(s *Session) (cancel bool, err error) {
	for _, handler := range h {
		if handler.OnServerConnect == nil {
			continue
		}
		cancel, err = handler.OnServerConnect(s)
		if err != nil {
			return false, err
		}
		if cancel {
			return true, nil
		}
	}
	return false, nil
}

func (h Handlers) OnConnect(s *Session) (cancel bool) {
	for _, handler := range h {
		if handler.OnConnect == nil {
			continue
		}
		if handler.OnConnect(s) {
			return true
		}
	}
	return false
}

func (h Handlers) OnSessionEnd(s *Session, wg *sync.WaitGroup) {
	for _, handler := range h {
		if handler.OnSessionEnd == nil {
			continue
		}
		handler.OnSessionEnd(s, wg)
	}
}

func (h Handlers) OnBlobs(s *Session, blobs []protocol.CacheBlob) {
	for _, handler := range h {
		if handler.OnBlobs == nil {
			continue
		}
		handler.OnBlobs(s, blobs)
	}
}

func (h Handlers) OnPlayerMove(s *Session) {
	for _, handler := range h {
		if handler.OnPlayerMove == nil {
			continue
		}
		handler.OnPlayerMove(s)
	}
}
