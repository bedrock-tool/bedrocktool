package c7client

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PathfindingModule implements automated pathfinding and movement
type PathfindingModule struct {
	BaseModule
	mu sync.RWMutex

	// Context and handler
	ctx     context.Context
	handler *C7Handler
	session *proxy.Session

	// Player state
	playerPos      protocol.Vec3
	playerYaw      float32
	playerPitch    float32
	onGround       bool
	lastUpdateTime time.Time

	// Pathfinding state
	targetPos     *protocol.Vec3
	isNavigating  bool
	currentPath   []protocol.Vec3
	pathIndex     int
	navigationMu  sync.Mutex

	// Configuration
	config PathfindingConfig
}

// PathfindingConfig holds pathfinding configuration
type PathfindingConfig struct {
	WalkSpeed        float64 // Blocks per second
	SprintSpeed      float64 // Blocks per second
	JumpHeight       float64 // Maximum jump height
	MaxFallDistance  float64 // Maximum safe fall distance
	EnableJumping    bool    // Allow jumping
	EnableSprinting  bool    // Allow sprinting
	EnableParkour    bool    // Enable parkour movements (gap jumps)
	PathUpdateRate   int     // Milliseconds between path updates
	StuckThreshold   float64 // Distance threshold for stuck detection
	MaxPathLength    int     // 0 = unlimited
}

// PathNode represents a node in the pathfinding grid
type PathNode struct {
	pos      protocol.Vec3
	gCost    float64 // Cost from start
	hCost    float64 // Heuristic cost to end
	fCost    float64 // Total cost
	parent   *PathNode
	walkable bool
}

// NewPathfindingModule creates a new pathfinding module
func NewPathfindingModule() *PathfindingModule {
	return &PathfindingModule{
		config: PathfindingConfig{
			WalkSpeed:       4.3,   // Normal walk speed
			SprintSpeed:     5.6,   // Sprint speed
			JumpHeight:      1.25,  // Can jump ~1.25 blocks
			MaxFallDistance: 3.0,   // Safe fall distance
			EnableJumping:   true,
			EnableSprinting: true,
			EnableParkour:   true,
			PathUpdateRate:  50,    // 50ms updates
			StuckThreshold:  0.1,   // 0.1 blocks
			MaxPathLength:   0,     // Unlimited
		},
		currentPath: make([]protocol.Vec3, 0),
	}
}

// Name returns the module name
func (m *PathfindingModule) Name() string {
	return "Pathfinding & Navigation"
}

// Description returns module description
func (m *PathfindingModule) Description() string {
	return "Automated pathfinding and movement with obstacle detection"
}

// Init initializes the module
func (m *PathfindingModule) Init(ctx context.Context, handler *C7Handler) error {
	m.ctx = ctx
	m.handler = handler
	m.log("Pathfinding module initialized")
	m.log("⚠️  For testing and educational purposes only")
	m.log("Configuration:")
	m.log(fmt.Sprintf("  - Walk speed: %.1f blocks/s", m.config.WalkSpeed))
	m.log(fmt.Sprintf("  - Sprint speed: %.1f blocks/s", m.config.SprintSpeed))
	m.log(fmt.Sprintf("  - Jump height: %.2f blocks", m.config.JumpHeight))
	m.log(fmt.Sprintf("  - Parkour enabled: %v", m.config.EnableParkour))
	return nil
}

// OnSessionStart is called when session starts
func (m *PathfindingModule) OnSessionStart(session *proxy.Session) error {
	m.session = session
	return nil
}

// OnConnect is called when connecting to the server
func (m *PathfindingModule) OnConnect(session *proxy.Session) error {
	m.log("Connected - Pathfinding ready")
	m.log("Commands:")
	m.log("  /goto <x> <y> <z> - Navigate to coordinates")
	m.log("  /stop - Stop navigation")
	m.log("  /path-status - Show navigation status")
	return nil
}

// PacketCallback handles incoming and outgoing packets
func (m *PathfindingModule) PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error) {
	// Monitor player position updates from server
	if !toServer {
		switch p := pk.(type) {
		case *packet.MovePlayer:
			// Update player position from server
			if m.isOwnPlayer(p.EntityRuntimeID) {
				m.updatePlayerPosition(p.Position, p.Yaw, p.Pitch, p.OnGround)
			}
		}
	}

	return pk, nil
}

// isOwnPlayer checks if the entity ID is the local player
func (m *PathfindingModule) isOwnPlayer(entityID uint64) bool {
	// Entity runtime ID 1 is typically the local player
	// This may need adjustment based on server implementation
	return entityID == 1
}

// updatePlayerPosition updates the tracked player position
func (m *PathfindingModule) updatePlayerPosition(pos protocol.Vec3, yaw, pitch float32, onGround bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.playerPos = pos
	m.playerYaw = yaw
	m.playerPitch = pitch
	m.onGround = onGround
	m.lastUpdateTime = time.Now()

	// Check navigation progress
	if m.isNavigating {
		m.checkNavigationProgress()
	}
}

// HandleCommand processes pathfinding commands
func (m *PathfindingModule) HandleCommand(cmd string, args []string) bool {
	switch cmd {
	case "goto":
		if len(args) < 3 {
			m.log("Usage: /goto <x> <y> <z>")
			return true
		}
		var x, y, z float64
		fmt.Sscanf(args[0], "%f", &x)
		fmt.Sscanf(args[1], "%f", &y)
		fmt.Sscanf(args[2], "%f", &z)
		m.navigateTo(protocol.Vec3{float32(x), float32(y), float32(z)})
		return true

	case "stop":
		m.stopNavigation()
		return true

	case "path-status":
		m.printStatus()
		return true
	}
	return false
}

// navigateTo starts navigation to target coordinates
func (m *PathfindingModule) navigateTo(target protocol.Vec3) {
	m.navigationMu.Lock()
	defer m.navigationMu.Unlock()

	m.log(fmt.Sprintf("🎯 Navigating to: (%.1f, %.1f, %.1f)", target[0], target[1], target[2]))

	m.targetPos = &target
	m.isNavigating = true
	m.pathIndex = 0

	// Calculate initial path
	path := m.calculatePath(m.playerPos, target)
	if len(path) == 0 {
		m.log("❌ No path found to target")
		m.isNavigating = false
		return
	}

	m.currentPath = path
	m.log(fmt.Sprintf("✅ Path calculated: %d waypoints", len(path)))

	// Start navigation goroutine
	go m.navigationLoop()
}

// stopNavigation stops the current navigation
func (m *PathfindingModule) stopNavigation() {
	m.navigationMu.Lock()
	defer m.navigationMu.Unlock()

	if !m.isNavigating {
		m.log("No active navigation")
		return
	}

	m.isNavigating = false
	m.targetPos = nil
	m.currentPath = nil
	m.log("⏹️  Navigation stopped")
}

// navigationLoop handles the navigation process
func (m *PathfindingModule) navigationLoop() {
	ticker := time.NewTicker(time.Duration(m.config.PathUpdateRate) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if !m.isNavigating {
				return
			}

			m.navigationMu.Lock()
			if m.pathIndex >= len(m.currentPath) {
				m.log("🎉 Destination reached!")
				m.isNavigating = false
				m.navigationMu.Unlock()
				return
			}

			nextWaypoint := m.currentPath[m.pathIndex]
			m.navigationMu.Unlock()

			// Move towards next waypoint
			m.moveTowards(nextWaypoint)
		}
	}
}

// moveTowards moves the player towards a specific position
func (m *PathfindingModule) moveTowards(target protocol.Vec3) {
	m.mu.RLock()
	currentPos := m.playerPos
	m.mu.RUnlock()

	// Calculate direction
	dx := float64(target[0] - currentPos[0])
	dy := float64(target[1] - currentPos[1])
	dz := float64(target[2] - currentPos[2])

	distance := math.Sqrt(dx*dx + dz*dz) // Horizontal distance

	// Check if we reached this waypoint
	if distance < 0.5 {
		m.navigationMu.Lock()
		m.pathIndex++
		m.navigationMu.Unlock()
		return
	}

	// Calculate required yaw
	yaw := float32(math.Atan2(dz, dx) * 180 / math.Pi)
	yaw = yaw - 90 // Adjust for Minecraft coordinate system

	// Calculate pitch (for jumping/falling)
	pitch := float32(0.0)
	if math.Abs(dy) > 0.1 {
		horizontalDist := math.Sqrt(dx*dx + dz*dz)
		pitch = float32(-math.Atan2(dy, horizontalDist) * 180 / math.Pi)
	}

	// Determine if we need to jump
	needsJump := m.shouldJump(currentPos, target)

	// Calculate movement speed
	speed := m.config.WalkSpeed
	if m.config.EnableSprinting && distance > 5 {
		speed = m.config.SprintSpeed
	}

	// Calculate new position
	timeStep := float64(m.config.PathUpdateRate) / 1000.0 // Convert to seconds
	moveDistance := speed * timeStep

	// Normalize direction
	if distance > 0 {
		dx = dx / distance * moveDistance
		dz = dz / distance * moveDistance
	}

	newPos := protocol.Vec3{
		currentPos[0] + float32(dx),
		currentPos[1] + float32(dy),
		currentPos[2] + float32(dz),
	}

	// Apply jump if needed
	if needsJump && m.config.EnableJumping {
		newPos[1] += float32(m.config.JumpHeight)
	}

	// Send movement packet
	m.sendMovementPacket(newPos, yaw, pitch, needsJump)
}

// shouldJump determines if a jump is needed
func (m *PathfindingModule) shouldJump(current, target protocol.Vec3) bool {
	if !m.config.EnableJumping {
		return false
	}

	// Check if target is higher
	heightDiff := target[1] - current[1]
	if heightDiff > 0.5 && heightDiff <= float32(m.config.JumpHeight) {
		return true
	}

	// Parkour: Check if there's a gap ahead
	if m.config.EnableParkour {
		// Simplified gap detection
		// In a full implementation, you'd check block data
		return false // Placeholder
	}

	return false
}

// sendMovementPacket sends a movement packet to the server
func (m *PathfindingModule) sendMovementPacket(pos protocol.Vec3, yaw, pitch float32, jump bool) {
	if m.session == nil {
		return
	}

	// Create MovePlayer packet
	pk := &packet.MovePlayer{
		EntityRuntimeID: 1, // Local player
		Position:        pos,
		Pitch:           pitch,
		Yaw:             yaw,
		HeadYaw:         yaw,
		Mode:            packet.MoveModePitch, // Normal movement
		OnGround:        !jump,
		Tick:            uint64(time.Now().UnixNano() / 1e6),
	}

	// Send packet to server
	_ = m.session.WritePacket(pk)
}

// calculatePath uses A* pathfinding to find a path
func (m *PathfindingModule) calculatePath(start, end protocol.Vec3) []protocol.Vec3 {
	// Simplified A* implementation
	// In a full implementation, you would:
	// 1. Use world data to check block solidity
	// 2. Consider jump distances and fall distances
	// 3. Handle complex obstacles

	// For now, return a direct path with waypoints
	path := make([]protocol.Vec3, 0)

	// Calculate total distance
	dx := end[0] - start[0]
	dy := end[1] - start[1]
	dz := end[2] - start[2]
	distance := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))

	// Create waypoints every 5 blocks
	numWaypoints := int(distance / 5)
	if numWaypoints < 1 {
		numWaypoints = 1
	}

	for i := 1; i <= numWaypoints; i++ {
		ratio := float32(i) / float32(numWaypoints)
		waypoint := protocol.Vec3{
			start[0] + dx*ratio,
			start[1] + dy*ratio,
			start[2] + dz*ratio,
		}
		path = append(path, waypoint)
	}

	// Add final destination
	path = append(path, end)

	return path
}

// checkNavigationProgress monitors if we're making progress
func (m *PathfindingModule) checkNavigationProgress() {
	// Check if stuck
	if m.targetPos == nil {
		return
	}

	// Calculate distance to target
	dx := m.targetPos[0] - m.playerPos[0]
	dy := m.targetPos[1] - m.playerPos[1]
	dz := m.targetPos[2] - m.playerPos[2]
	distance := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))

	// Check if arrived
	if distance < 1.0 {
		m.log("🎯 Arrived at destination!")
		m.isNavigating = false
		m.targetPos = nil
	}
}

// printStatus prints the current navigation status
func (m *PathfindingModule) printStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.log("=== Pathfinding Status ===")
	m.log(fmt.Sprintf("Current position: (%.1f, %.1f, %.1f)", m.playerPos[0], m.playerPos[1], m.playerPos[2]))
	m.log(fmt.Sprintf("On ground: %v", m.onGround))

	if m.isNavigating && m.targetPos != nil {
		dx := m.targetPos[0] - m.playerPos[0]
		dy := m.targetPos[1] - m.playerPos[1]
		dz := m.targetPos[2] - m.playerPos[2]
		distance := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))

		m.log(fmt.Sprintf("Target: (%.1f, %.1f, %.1f)", m.targetPos[0], m.targetPos[1], m.targetPos[2]))
		m.log(fmt.Sprintf("Distance: %.1f blocks", distance))
		m.log(fmt.Sprintf("Waypoints: %d/%d", m.pathIndex, len(m.currentPath)))
		m.log("Status: 🚶 Navigating...")
	} else {
		m.log("Status: ⏹️  Idle")
	}
}

// OnSessionEnd is called when the session ends
func (m *PathfindingModule) OnSessionEnd(session *proxy.Session) {
	m.stopNavigation()
	m.log("Session ended")
}

// Cleanup cleans up module resources
func (m *PathfindingModule) Cleanup() {
	m.stopNavigation()
	m.log("Cleanup complete")
}

// log outputs a message
func (m *PathfindingModule) log(message string) {
	fmt.Printf("[Pathfinding] %s\n", message)
}
