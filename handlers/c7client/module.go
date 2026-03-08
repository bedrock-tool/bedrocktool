package c7client

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Module represents a C7 Client utility module
type Module interface {
	// Name returns the module name
	Name() string
	
	// Description returns what the module does
	Description() string
	
	// Init is called when the module is initialized
	Init(ctx context.Context, handler *C7Handler) error
	
	// OnSessionStart is called when a session starts
	OnSessionStart(session *proxy.Session) error
	
	// OnConnect is called when connected to the server
	OnConnect(session *proxy.Session) error
	
	// PacketCallback handles packets
	PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error)
	
	// OnSessionEnd is called when the session ends
	OnSessionEnd(session *proxy.Session)
	
	// Cleanup is called when the module is destroyed
	Cleanup()
}

// BaseModule provides default implementations for Module interface
type BaseModule struct{}

func (b *BaseModule) Init(ctx context.Context, handler *C7Handler) error {
	return nil
}

func (b *BaseModule) OnSessionStart(session *proxy.Session) error {
	return nil
}

func (b *BaseModule) OnConnect(session *proxy.Session) error {
	return nil
}

func (b *BaseModule) PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error) {
	return pk, nil
}

func (b *BaseModule) OnSessionEnd(session *proxy.Session) {
}

func (b *BaseModule) Cleanup() {
}

// ModuleSettings represents configuration for modules
type ModuleSettings struct {
	PlayerTracking     bool `opt:"Player Tracking" flag:"player-tracking" default:"true" desc:"Enable player tracking compass"`
	InventorySecurity  bool `opt:"Inventory Security" flag:"inventory-security" default:"false" desc:"Enable inventory transaction security monitoring"`
	Baritone           bool `opt:"Baritone" flag:"baritone" default:"false" desc:"Enable automated pathfinding and navigation"`
	// Future modules can add their settings here
}
