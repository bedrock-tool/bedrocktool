package c7client

import (
	"context"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

// C7Handler manages C7 Client modules
type C7Handler struct {
	ctx     context.Context
	session *proxy.Session
	log     *logrus.Entry
	
	modules        []Module
	enabledModules map[string]bool
	mu             sync.RWMutex
}

// NewC7Handler creates a new C7 Client handler with specified modules
func NewC7Handler(ctx context.Context, moduleSettings ModuleSettings) func() *proxy.Handler {
	return func() *proxy.Handler {
		handler := &C7Handler{
			ctx:            ctx,
			log:            logrus.WithField("part", "C7Handler"),
			modules:        make([]Module, 0),
			enabledModules: make(map[string]bool),
		}

		// Register modules based on settings
		if moduleSettings.PlayerTracking {
			handler.RegisterModule(NewPlayerTrackingModule())
		}
		if moduleSettings.InventorySecurity {
			handler.RegisterModule(NewInventorySecurityModule())
		}
		if moduleSettings.Pathfinding {
			handler.RegisterModule(NewPathfindingModule())
		}

		return &proxy.Handler{
			Name:           "C7 Client",
			SessionStart:   handler.onSessionStart,
			OnConnect:      handler.onConnect,
			PacketCallback: handler.packetCallback,
			OnSessionEnd:   handler.onSessionEnd,
		}
	}
}

// RegisterModule adds a module to the handler
func (h *C7Handler) RegisterModule(module Module) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.modules = append(h.modules, module)
	h.enabledModules[module.Name()] = true
	h.log.Infof("Registered module: %s", module.Name())
}

// GetSession returns the current session
func (h *C7Handler) GetSession() *proxy.Session {
	return h.session
}

func (h *C7Handler) onSessionStart(session *proxy.Session, serverName string) error {
	h.session = session
	h.log.Infof("C7 Client starting with %d modules", len(h.modules))
	
	// Initialize all modules
	for _, module := range h.modules {
		if err := module.Init(h.ctx, h); err != nil {
			h.log.Errorf("Failed to initialize module %s: %v", module.Name(), err)
			return err
		}
		
		if err := module.OnSessionStart(session); err != nil {
			h.log.Errorf("Failed to start module %s: %v", module.Name(), err)
			return err
		}
		
		h.log.Infof("Module %s initialized", module.Name())
	}
	
	return nil
}

func (h *C7Handler) onConnect(session *proxy.Session) error {
	h.log.Info("Connected to server, starting modules")
	
	for _, module := range h.modules {
		if err := module.OnConnect(session); err != nil {
			h.log.Errorf("Module %s connection failed: %v", module.Name(), err)
			return err
		}
	}
	
	return nil
}

func (h *C7Handler) packetCallback(pk packet.Packet, toServer bool, timeReceived time.Time, preLogin bool) (packet.Packet, error) {
	if preLogin {
		return pk, nil
	}
	
	var err error
	for _, module := range h.modules {
		pk, err = module.PacketCallback(pk, toServer, h.session)
		if err != nil {
			return pk, err
		}
		if pk == nil {
			return nil, nil
		}
	}
	
	return pk, nil
}

func (h *C7Handler) onSessionEnd(session *proxy.Session, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		h.log.Info("Shutting down modules")
		for _, module := range h.modules {
			module.OnSessionEnd(session)
			module.Cleanup()
		}
	}()
}
