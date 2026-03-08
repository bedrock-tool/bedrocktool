# C7 CLIENT Framework

## Overview

C7 CLIENT is a modular utility framework for Minecraft Bedrock Edition that provides various quality-of-life features through a proxy connection. The framework is designed to be extensible, allowing multiple independent modules to run simultaneously.

## Architecture

### Modular Design

C7 CLIENT follows a modular architecture where each feature is implemented as an independent module:

```
┌─────────────────────────────────┐
│      C7 Client Handler          │
│  (Module Manager & Coordinator) │
└────────────┬────────────────────┘
             │
    ┌────────┴────────┐
    │                 │
┌───▼────┐     ┌─────▼──────┐
│ Module │     │   Module   │
│   #1   │     │     #2     │
└────────┘     └────────────┘
```

### Core Components

1. **Module Interface** - Defines the contract all modules must implement
2. **C7Handler** - Manages module lifecycle and coordinates packet handling
3. **ModuleSettings** - Configuration structure for enabling/disabling modules
4. **BaseModule** - Provides default implementations for optional module methods

## Using C7 CLIENT

### Basic Usage

```bash
c7client c7 -address server.address.com
```

### With Module Configuration

```bash
c7client c7 -address server.com -player-tracking=true
```

### Available Flags

- `-address` - Server address to connect to
- `-player-tracking` - Enable/disable player tracking module (default: true)
- (More module flags as modules are added)

## Current Modules

### Player Tracking

**Status:** ✅ Active  
**Default:** Enabled  
**Flag:** `-player-tracking`

Track other players on the server with real-time distance and direction information.

**Commands:**
- `/list-players` - List all online players
- `/track <name>` - Start tracking a player
- `/track-info` - Show tracking information
- `/untrack` - Stop tracking

**Features:**
- Real-time position updates
- Distance calculation
- Cardinal direction display
- Automatic player list management

See [PLAYER_TRACKING.md](PLAYER_TRACKING.md) for detailed documentation.

## Creating a New Module

### Step 1: Implement the Module Interface

Create a new file in `/handlers/c7client/your_module.go`:

```go
package c7client

import (
	"context"
	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type YourModule struct {
	BaseModule  // Inherit default implementations
	
	ctx     context.Context
	handler *C7Handler
	log     *logrus.Entry
	
	// Your module's fields here
}

func NewYourModule() *YourModule {
	return &YourModule{
		log: logrus.WithField("module", "YourModule"),
	}
}

func (m *YourModule) Name() string {
	return "your_module"
}

func (m *YourModule) Description() string {
	return "What your module does"
}

func (m *YourModule) Init(ctx context.Context, handler *C7Handler) error {
	m.ctx = ctx
	m.handler = handler
	
	// Initialize your module
	m.log.Info("Module initialized")
	return nil
}

func (m *YourModule) OnConnect(session *proxy.Session) error {
	// Register commands, set up listeners, etc.
	
	session.AddCommand(func(args []string) bool {
		// Your command logic
		return true
	}, protocol.Command{
		Name:        "yourcommand",
		Description: "Command description",
	})
	
	return nil
}

func (m *YourModule) PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error) {
	// Handle packets relevant to your module
	
	switch pk := pk.(type) {
	case *packet.YourPacketType:
		// Process packet
	}
	
	return pk, nil
}

func (m *YourModule) OnSessionEnd(session *proxy.Session) {
	// Cleanup when session ends
	m.log.Info("Session ended")
}

func (m *YourModule) Cleanup() {
	// Final cleanup
	m.log.Info("Module cleaned up")
}
```

### Step 2: Add Module Settings

Edit `/handlers/c7client/module.go`:

```go
type ModuleSettings struct {
	PlayerTracking bool `opt:"Player Tracking" flag:"player-tracking" default:"true" desc:"Enable player tracking"`
	YourModule     bool `opt:"Your Module" flag:"your-module" default:"false" desc:"Enable your module"`
}
```

### Step 3: Register the Module

Edit `/handlers/c7client/handler.go` in the `NewC7Handler` function:

```go
// Register modules based on settings
if moduleSettings.PlayerTracking {
	handler.RegisterModule(NewPlayerTrackingModule())
}
if moduleSettings.YourModule {
	handler.RegisterModule(NewYourModule())
}
```

### Step 4: Test Your Module

```bash
# Build the project
go build ./cmd/c7client

# Test with your module enabled
./c7client c7 -address test.server.com -your-module=true
```

## Module Lifecycle

Modules go through the following lifecycle:

1. **Creation** - Module is instantiated via `NewYourModule()`
2. **Registration** - Added to handler via `RegisterModule()`
3. **Initialization** - `Init()` called with context and handler reference
4. **Session Start** - `OnSessionStart()` called when proxy session begins
5. **Connection** - `OnConnect()` called when connected to server
6. **Runtime** - `PacketCallback()` called for each packet
7. **Session End** - `OnSessionEnd()` called when session closes
8. **Cleanup** - `Cleanup()` called for final cleanup

```
NewModule() → Register → Init → SessionStart → OnConnect → [PacketCallback...] → SessionEnd → Cleanup
```

## Best Practices

### 1. Use the BaseModule

Inherit from `BaseModule` to get default implementations:

```go
type YourModule struct {
	BaseModule
	// ...
}
```

### 2. Proper Logging

Always use structured logging with a module identifier:

```go
m.log = logrus.WithField("module", "ModuleName")
m.log.Info("Something happened")
m.log.Errorf("Error: %v", err)
```

### 3. Thread Safety

Use mutexes for shared state:

```go
type YourModule struct {
	BaseModule
	mu   sync.RWMutex
	data map[string]string
}

func (m *YourModule) SetData(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}
```

### 4. Error Handling

Always return errors from lifecycle methods:

```go
func (m *YourModule) Init(ctx context.Context, handler *C7Handler) error {
	if err := m.initialize(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	return nil
}
```

### 5. Graceful Cleanup

Clean up resources in both `OnSessionEnd` and `Cleanup`:

```go
func (m *YourModule) OnSessionEnd(session *proxy.Session) {
	m.stopTimers()
	m.closeConnections()
}

func (m *YourModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]string)
}
```

### 6. Command Color Codes

Use Minecraft color codes for better UX:

```go
session.SendMessage("§aSuccess message")
session.SendMessage("§cError message")
session.SendMessage("§eWarning message")
session.SendMessage("§7Gray text §f| §aGreen text")
```

Common colors:
- `§0` - Black
- `§1` - Dark Blue
- `§2` - Dark Green
- `§3` - Dark Aqua
- `§4` - Dark Red
- `§5` - Dark Purple
- `§6` - Gold
- `§7` - Gray
- `§8` - Dark Gray
- `§9` - Blue
- `§a` - Green
- `§b` - Aqua
- `§c` - Red
- `§d` - Light Purple
- `§e` - Yellow
- `§f` - White

## Module Communication

Modules can communicate through the shared `C7Handler`:

```go
// In YourModule
session := m.handler.GetSession()
```

For more complex inter-module communication, consider implementing a message bus or event system.

## Testing Modules

### Unit Testing

Create test files for your module:

```go
// your_module_test.go
package c7client

import (
	"context"
	"testing"
)

func TestYourModule_Init(t *testing.T) {
	module := NewYourModule()
	ctx := context.Background()
	
	// Mock handler
	handler := &C7Handler{}
	
	err := module.Init(ctx, handler)
	if err != nil {
		t.Errorf("Init failed: %v", err)
	}
}
```

### Integration Testing

Test with a real server:

```bash
c7client c7 -address test.server.com -your-module=true -debug=true
```

## Future Module Ideas

- **Auto Respawn** - Automatically respawn on death
- **Waypoint System** - Save and navigate to waypoints
- **Server Statistics** - Track server performance metrics
- **Chat Logger** - Log chat messages with timestamps
- **Block Logger** - Track block changes
- **Entity Counter** - Count and categorize entities
- **Inventory Manager** - Backup/restore inventory states
- **Custom Overlays** - Display custom HUD elements

## Troubleshooting

### Module Not Loading

Check that:
1. Module is registered in `NewC7Handler()`
2. Module flag is set correctly in command line
3. `Init()` doesn't return an error

### Commands Not Working

Verify:
1. Commands registered in `OnConnect()`
2. Command names don't conflict with existing commands
3. Session is not nil when registering

### Packet Handling Issues

Remember:
1. Return `pk, nil` to pass packet through
2. Return `nil, nil` to block packet
3. Return `pk, error` to stop processing with error

## Contributing

To contribute a new module:

1. Follow the module creation guide above
2. Add comprehensive tests
3. Document all commands and features
4. Update this README with module info
5. Submit a pull request

## License

See main project LICENSE file.
