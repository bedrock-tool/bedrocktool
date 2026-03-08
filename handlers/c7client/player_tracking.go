package c7client

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/utils/proxy"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

// PlayerTrackingModule tracks other players and displays a compass
type PlayerTrackingModule struct {
	BaseModule
	
	ctx     context.Context
	handler *C7Handler
	log     *logrus.Entry
	
	// Player tracking
	mu              sync.RWMutex
	players         map[uuid.UUID]*TrackedPlayer
	trackedPlayerID uuid.UUID
	trackedUsername string
	trackingEnabled bool
	
	// Map overlay
	mapOverlay *MapOverlay
}

// TrackedPlayer represents a player being tracked
type TrackedPlayer struct {
	UUID     uuid.UUID
	Username string
	Position mgl32.Vec3
	EntityID uint64
}

func NewPlayerTrackingModule() *PlayerTrackingModule {
	return &PlayerTrackingModule{
		players: make(map[uuid.UUID]*TrackedPlayer),
		log:     logrus.WithField("module", "PlayerTracking"),
	}
}

func (m *PlayerTrackingModule) Name() string {
	return "player_tracking"
}

func (m *PlayerTrackingModule) Description() string {
	return "Track other players with a visual compass on your map"
}

func (m *PlayerTrackingModule) Init(ctx context.Context, handler *C7Handler) error {
	m.ctx = ctx
	m.handler = handler
	m.mapOverlay = NewMapOverlay()
	m.log.Info("Player tracking module initialized")
	return nil
}

func (m *PlayerTrackingModule) OnSessionStart(session *proxy.Session) error {
	m.log.Info("Player tracking session started")
	return nil
}

func (m *PlayerTrackingModule) OnConnect(session *proxy.Session) error {
	m.log.Info("Player tracking connected")
	
	// Register commands
	session.AddCommand(func(args []string) bool {
		var playersList []string
		m.mu.RLock()
		for _, p := range m.players {
			playersList = append(playersList, p.Username)
		}
		m.mu.RUnlock()
		
		if len(playersList) == 0 {
			session.SendMessage("§cNo other players online")
		} else {
			session.SendMessage("§aOnline players: §f" + strings.Join(playersList, ", "))
		}
		return true
	}, protocol.Command{
		Name:        "list-players",
		Description: "List all players on the server",
	})
	
	session.AddCommand(func(args []string) bool {
		if len(args) == 0 {
			session.SendMessage("§eUsage: /track <player_name>")
			return true
		}
		
		targetName := strings.Join(args, " ")
		found := false
		
		m.mu.Lock()
		for id, p := range m.players {
			if strings.EqualFold(p.Username, targetName) {
				m.trackedPlayerID = id
				m.trackedUsername = p.Username
				m.trackingEnabled = true
				found = true
				m.mu.Unlock()
				session.SendMessage(fmt.Sprintf("§aNow tracking player: §f%s", p.Username))
				m.log.Infof("Now tracking player: %s", p.Username)
				return true
			}
		}
		m.mu.Unlock()
		
		if !found {
			session.SendMessage(fmt.Sprintf("§cPlayer '%s' not found. Use /list-players to see available players.", targetName))
		}
		return true
	}, protocol.Command{
		Name:        "track",
		Description: "Track a player with compass overlay",
		Overloads: []protocol.CommandOverload{
			{
				Parameters: []protocol.CommandParameter{
					{
						Name:     "player_name",
						Type:     protocol.CommandArgTypeString,
						Optional: false,
					},
				},
			},
		},
	})
	
	session.AddCommand(func(args []string) bool {
		m.mu.Lock()
		wasTracking := m.trackingEnabled
		trackedName := m.trackedUsername
		m.trackingEnabled = false
		m.trackedPlayerID = uuid.Nil
		m.trackedUsername = ""
		m.mu.Unlock()
		
		if wasTracking {
			session.SendMessage(fmt.Sprintf("§aStopped tracking §f%s", trackedName))
			m.log.Infof("Stopped tracking %s", trackedName)
		} else {
			session.SendMessage("§cNot currently tracking anyone")
		}
		return true
	}, protocol.Command{
		Name:        "untrack",
		Description: "Stop tracking the current player",
	})
	
	session.AddCommand(func(args []string) bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		
		if !m.trackingEnabled {
			session.SendMessage("§cNot tracking anyone")
			return true
		}
		
		if tracked, ok := m.players[m.trackedPlayerID]; ok {
			playerPos := session.Player.Position
			dx := tracked.Position.X() - playerPos.X()
			dz := tracked.Position.Z() - playerPos.Z()
			distance := math.Sqrt(float64(dx*dx + dz*dz))
			
			// Calculate direction
			angle := math.Atan2(float64(dx), float64(-dz))
			degrees := angle * 180 / math.Pi
			if degrees < 0 {
				degrees += 360
			}
			
			direction := getDirection(degrees)
			
			session.SendMessage(fmt.Sprintf("§aTracking: §f%s §7| §aDistance: §f%.1fm §7| §aDirection: §f%s", 
				tracked.Username, distance, direction))
		} else {
			session.SendMessage("§cTracked player not found")
		}
		return true
	}, protocol.Command{
		Name:        "track-info",
		Description: "Show information about tracked player",
	})
	
	return nil
}

func (m *PlayerTrackingModule) PacketCallback(pk packet.Packet, toServer bool, session *proxy.Session) (packet.Packet, error) {
	switch pk := pk.(type) {
	case *packet.AddPlayer:
		m.addPlayer(pk)
		
	case *packet.MovePlayer:
		m.updatePlayerPosition(pk.EntityRuntimeID, pk.Position)
		
	case *packet.PlayerList:
		if pk.ActionType == packet.PlayerListActionAdd {
			for _, entry := range pk.Entries {
				m.mu.Lock()
				if player, ok := m.players[entry.UUID]; ok {
					player.Username = entry.Username
				} else {
					m.players[entry.UUID] = &TrackedPlayer{
						UUID:     entry.UUID,
						Username: entry.Username,
						Position: mgl32.Vec3{},
					}
				}
				m.mu.Unlock()
			}
		} else if pk.ActionType == packet.PlayerListActionRemove {
			for _, entry := range pk.Entries {
				m.removePlayer(entry.UUID)
			}
		}
		
	case *packet.RemoveActor:
		// Check if it's a player entity
		m.mu.Lock()
		for id, p := range m.players {
			if p.EntityID == pk.EntityUniqueID {
				delete(m.players, id)
				if id == m.trackedPlayerID {
					m.trackingEnabled = false
					m.trackedPlayerID = uuid.Nil
					session.SendMessage("§cTracked player left the game")
				}
				break
			}
		}
		m.mu.Unlock()
	}
	
	return pk, nil
}

func (m *PlayerTrackingModule) addPlayer(pk *packet.AddPlayer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	player := &TrackedPlayer{
		UUID:     pk.UUID,
		Username: pk.Username,
		Position: pk.Position,
		EntityID: pk.EntityRuntimeID,
	}
	m.players[pk.UUID] = player
	m.log.Debugf("Added player: %s", pk.Username)
}

func (m *PlayerTrackingModule) updatePlayerPosition(entityID uint64, position mgl32.Vec3) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, p := range m.players {
		if p.EntityID == entityID {
			p.Position = position
			break
		}
	}
}

func (m *PlayerTrackingModule) removePlayer(playerID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if player, ok := m.players[playerID]; ok {
		m.log.Debugf("Removed player: %s", player.Username)
		delete(m.players, playerID)
		
		if playerID == m.trackedPlayerID {
			m.trackingEnabled = false
			m.trackedPlayerID = uuid.Nil
			m.trackedUsername = ""
		}
	}
}

func (m *PlayerTrackingModule) OnSessionEnd(session *proxy.Session) {
	m.log.Info("Player tracking session ended")
}

func (m *PlayerTrackingModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.players = make(map[uuid.UUID]*TrackedPlayer)
	m.trackingEnabled = false
	m.log.Info("Player tracking cleaned up")
}

// Helper function to get cardinal direction from degrees
func getDirection(degrees float64) string {
	directions := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	index := int((degrees + 22.5) / 45) % 8
	return directions[index]
}

// MapOverlay handles rendering overlays on maps (future extension point)
type MapOverlay struct {
	mu sync.Mutex
}

func NewMapOverlay() *MapOverlay {
	return &MapOverlay{}
}

// DrawCompass draws a compass arrow on the image (for future map integration)
func (o *MapOverlay) DrawCompass(img *image.RGBA, playerPos, targetPos mgl32.Vec3, playerName string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	// Calculate direction vector
	dx := float64(targetPos.X() - playerPos.X())
	dz := float64(targetPos.Z() - playerPos.Z())
	distance := math.Sqrt(dx*dx + dz*dz)
	
	// Draw compass in the top right corner
	centerX := img.Bounds().Max.X - 20
	centerY := 20
	
	// Calculate angle
	angle := math.Atan2(dx, -dz)
	
	// Draw compass circle
	drawCircle(img, centerX, centerY, 15, color.RGBA{50, 50, 50, 200})
	
	// Draw cardinal directions
	drawLine(img, centerX, centerY-12, centerX, centerY-15, color.RGBA{200, 200, 200, 255})
	
	// Draw arrow pointing to target
	arrowLength := 10.0
	arrowX := centerX + int(arrowLength*math.Sin(angle))
	arrowY := centerY - int(arrowLength*math.Cos(angle))
	
	drawLine(img, centerX, centerY, arrowX, arrowY, color.RGBA{255, 50, 50, 255})
	
	// Draw distance text
	distText := fmt.Sprintf("%s: %.0fm", playerName, distance)
	drawText(img, distText, img.Bounds().Max.X-len(distText)*6-5, img.Bounds().Max.Y-10, color.RGBA{255, 255, 255, 255})
}

// Helper drawing functions
func drawCircle(img *image.RGBA, cx, cy, radius int, col color.RGBA) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				px := cx + x
				py := cy + y
				if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
					img.SetRGBA(px, py, col)
				}
			}
		}
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy
	
	for {
		if x0 >= 0 && x0 < img.Bounds().Max.X && y0 >= 0 && y0 < img.Bounds().Max.Y {
			img.SetRGBA(x0, y0, col)
		}
		
		if x0 == x1 && y0 == y1 {
			break
		}
		
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func drawText(img *image.RGBA, text string, x, y int, col color.RGBA) {
	// Simple background
	for i := 0; i < len(text)*6; i++ {
		for j := -2; j < 10; j++ {
			px := x + i
			py := y + j
			if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
				img.SetRGBA(px, py, color.RGBA{0, 0, 0, 200})
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
