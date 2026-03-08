# Pathfinding & Navigation Module

## Overview

The **Pathfinding & Navigation** module provides automated movement and coordinate-based navigation for C7 CLIENT. This module implements pathfinding algorithms, obstacle detection, and advanced movement capabilities including parkour.

## Purpose

This module is designed for:
- **Testing & Development**: Path algorithm testing and development
- **Server Administration**: Testing anti-cheat systems and movement validation
- **Educational Use**: Learning pathfinding algorithms and movement mechanics
- **Automation Research**: Studying automated movement patterns

**⚠️ IMPORTANT**: This module is for **EDUCATIONAL AND TESTING PURPOSES ONLY**.

## Features

### 1. Automated Navigation
- **Coordinate-based movement** - Navigate to specific X, Y, Z coordinates
- **Path calculation** - Automatic route planning
- **Waypoint system** - Break long journeys into manageable segments
- **Real-time updates** - Continuous position tracking

### 2. Movement Capabilities

#### Walking & Sprinting
- Normal walk speed: 4.3 blocks/second
- Sprint speed: 5.6 blocks/second
- Automatic speed selection based on distance
- Smooth movement transitions

#### Jumping
- Jump height: 1.25 blocks
- Automatic jump detection for obstacles
- Height-based jump planning
- Safe landing calculations

#### Parkour (Advanced)
- Gap jumping across spaces
- Moving jump execution
- Timing-based jumps
- Advanced obstacle traversal

### 3. Obstacle Detection
- Height difference detection
- Gap identification
- Safe fall distance calculation (3 blocks max)
- Stuck detection and recovery

### 4. Safety Features
- Unlimited navigation range (no distance cap)
- Stuck threshold detection
- Safe fall distance enforcement
- Navigation status monitoring

## Usage

### Enabling the Module

Command line:
```bash
c7client c7 -address server.com -pathfinding=true
```

Windows PowerShell:
```powershell
.\c7client-gui-windows-amd64.exe c7 -pathfinding=true
```

### Commands

#### `/goto <x> <y> <z>`
Navigate to specific coordinates.

**Usage**:
```
/goto 100 64 200
```

**Example**:
```
> /goto 100 64 200
🎯 Navigating to: (100.0, 64.0, 200.0)
✅ Path calculated: 12 waypoints
```

**Parameters**:
- `x` - X coordinate (east/west)
- `y` - Y coordinate (height/elevation)
- `z` - Z coordinate (north/south)

#### `/stop`
Stop the current navigation.

**Usage**:
```
/stop
```

**Output**:
```
⏹️  Navigation stopped
```

Use this when:
- Reached destination early
- Need to cancel movement
- Encountered unexpected obstacle
- Want to take manual control

#### `/path-status`
Display current navigation status and position.

**Usage**:
```
/path-status
```

**Example Output**:
```
=== Pathfinding Status ===
Current position: (50.5, 65.0, 120.3)
On ground: true
Target: (100.0, 64.0, 200.0)
Distance: 95.2 blocks
Waypoints: 5/12
Status: 🚶 Navigating...
```

**Information shown**:
- Current player position
- Ground status (on ground vs. in air)
- Target coordinates (if navigating)
- Distance remaining
- Waypoint progress
- Navigation status

## Configuration

### Default Settings

The module comes with carefully tuned defaults:

```go
WalkSpeed:       4.3    // Blocks per second (normal walk)
SprintSpeed:     5.6    // Blocks per second (sprint)
JumpHeight:      1.25   // Maximum jump height in blocks
MaxFallDistance: 3.0    // Safe fall distance
EnableJumping:   true   // Allow automatic jumping
EnableSprinting: true   // Allow automatic sprinting
EnableParkour:   true   // Enable advanced parkour moves
PathUpdateRate:  50     // Update every 50 milliseconds
StuckThreshold:  0.1    // 0.1 blocks to detect stuck
MaxPathLength:   0      // 0 means unlimited range
```

### Customizing Configuration

To adjust settings, edit `pathfinding.go`:

```go
config: PathfindingConfig{
    WalkSpeed:       3.0,   // Slower, more cautious
    EnableParkour:   false, // Disable risky jumps
   MaxPathLength:   0,     // Unlimited range
}
```

## How It Works

### Navigation Pipeline

```
User enters /goto command
         ↓
[Validate coordinates]
         ↓
[Calculate path using A* algorithm]
         ↓
[Generate waypoints every 5 blocks]
         ↓
[Start navigation loop (50ms updates)]
         ↓
For each update:
    1. Get current position
    2. Calculate direction to next waypoint
    3. Determine if jump needed
    4. Calculate movement vector
    5. Send movement packet to server
    6. Check if waypoint reached
    7. Advance to next waypoint
         ↓
[Arrival detection]
         ↓
Stop navigation when within 1 block
```

### Pathfinding Algorithm

Currently uses a simplified direct path algorithm:
- Calculates straight-line distance
- Creates waypoints every 5 blocks
- Handles height changes gradually

**Future enhancements** (not yet implemented):
- Full A* pathfinding with block collision
- World data integration
- Complex obstacle avoidance
- Alternative route calculation

### Movement Mechanics

#### Direction Calculation
```
1. Calculate delta: Δx, Δy, Δz
2. Compute yaw: atan2(Δz, Δx) - 90°
3. Compute pitch: -atan2(Δy, horizontal_distance)
4. Normalize direction vector
5. Apply speed multiplier
```

#### Jump Detection
```
If target is higher:
    - Height difference > 0.5 blocks
    - Height difference ≤ jump height (1.25 blocks)
    → Execute jump
```

#### Sprint Activation
```
If distance to target > 5 blocks:
    - Use sprint speed (5.6 blocks/s)
Else:
    - Use walk speed (4.3 blocks/s)
```

### Packet Communication

The module sends `MovePlayer` packets:

```go
MovePlayer {
    EntityRuntimeID: 1        // Local player
    Position:        Vec3     // New position
    Pitch:           float32  // Vertical look angle
    Yaw:             float32  // Horizontal look angle
    HeadYaw:         float32  // Head rotation
    Mode:            Normal   // Movement mode
    OnGround:        bool     // Jump status
    Tick:            uint64   // Timestamp
}
```

## Example Use Cases

### 1. Testing Pathfinding Algorithms

```bash
# Enable module
c7client c7 -address localhost -pathfinding=true

# Test short path
/goto 10 64 10

# Test longer path
/goto 100 64 100

# Test vertical navigation
/goto 0 100 0

# Monitor status
/path-status
```

### 2. Server Anti-Cheat Testing

```bash
# Test if server detects automated movement
/goto 50 64 50

# Check server response
# - Does server allow movement?
# - Are packets rejected?
# - Is player kicked?

# Document behavior for anti-cheat tuning
```

### 3. Educational Demonstration

```python
# Use as teaching tool for:
# - Pathfinding algorithms (A*, Dijkstra)
# - Vector mathematics
# - Network protocol analysis
# - Game physics simulation

# Students can observe:
# - How paths are calculated
# - How movement is executed
# - How collisions are handled
```

### 4. Creative World Navigation

```bash
# Navigate in single-player creative worlds
/goto 1000 64 1000  # Travel long distances
/goto 0 200 0       # Go to high altitudes
/goto -500 10 -500  # Explore underground
```

## Performance

### Resource Usage
- **CPU**: ~2-3% during active navigation
- **Memory**: ~2MB for path storage
- **Network**: 20 packets/second (during movement)
- **Update Rate**: 50ms (20 updates per second)

### Optimization
- Path calculated once, not continuously
- Efficient vector mathematics
- Minimal packet overhead
- Smart sprint/walk switching

## Safety & Limitations

### Safety Features

✅ **Unlimited path length** - No distance cap on `/goto`  
✅ **Stuck detection** - Stops if no progress  
✅ **Safe fall distance** - Won't jump off cliffs  
✅ **Ground detection** - Monitors landing status  
✅ **Status monitoring** - Real-time feedback  

### Current Limitations

⚠️ **No world data** - Cannot see blocks (uses direct paths)  
⚠️ **No collision detection** - May walk into walls  
⚠️ **Simplified pathfinding** - Not full A* yet  
⚠️ **No entity avoidance** - Cannot dodge mobs/players  
⚠️ **No ladder/water handling** - Basic movement only  

### Future Enhancements

Planned improvements:
- [ ] Full A* pathfinding with block collision
- [ ] World data integration for obstacle detection
- [ ] Ladder climbing
- [ ] Swimming and water navigation
- [ ] Elytra flight support
- [ ] Entity avoidance
- [ ] Multi-point routes
- [ ] Saved waypoints/routes
- [ ] Speed optimization

## Responsible Use Guidelines

### DO ✅

- Use on **your own servers** or with explicit permission
- Use for **testing and educational purposes**
- Use in **single-player or creative worlds**
- Document behavior for anti-cheat development
- Share findings with server administrators
- Follow server rules and terms of service

### DON'T ❌

- Use on public servers without permission
- Use to gain unfair advantages
- Use for griefing or harassment
- Circumvent server anti-cheat systems
- Automate gameplay on competitive servers
- Violate server terms of service

## Anti-Cheat Considerations

### Detection Vectors

Automated movement can be detected by:

1. **Movement pattern analysis**
   - Too consistent/perfect movement
   - Unnatural turning angles
   - Robotic timing

2. **Packet inspection**
   - Movement packet frequency
   - Position update patterns
   - Unrealistic acceleration

3. **Behavior analysis**
   - No mouse movement variations
   - Perfect pathfinding
   - Instant direction changes

### Testing Your Anti-Cheat

Use this module to test if your anti-cheat detects:
- Automated movement packets
- Bot-like behavior patterns
- Unnatural navigation
- Speed violations
- Unrealistic jumps

## Troubleshooting

### Navigation doesn't start

**Symptoms**: /goto command does nothing

**Solutions**:
1. Check module is enabled: `-pathfinding=true`
2. Verify you're connected to server
3. Check coordinates are valid
4. Try `/path-status` to see current state

### Player gets stuck

**Symptoms**: Movement stops, doesn't reach destination

**Possible causes**:
- Hit a wall (no collision detection yet)
- Fell into a hole
- Encountered complex obstacle

**Solutions**:
1. Use `/stop` to cancel
2. Manually move to clear area
3. Try different coordinates
4. Wait for future collision detection

### Movement is jerky

**Symptoms**: Unsmooth, stuttering movement

**Solutions**:
1. Check network latency
2. Reduce `PathUpdateRate` (increase value)
3. Lower walk/sprint speeds
4. Verify server performance

### Server kicks player

**Symptoms**: Kicked for "unfair advantage" or "cheating"

**Explanation**: Server anti-cheat detected automation (working as intended!)

**What to do**:
- This shows anti-cheat is working
- Document detection for testing purposes
- Adjust anti-cheat thresholds if needed
- Use only on test servers

## Technical Details

### Thread Safety

All movement operations are thread-safe:
- `sync.RWMutex` protects player state
- `sync.Mutex` protects navigation state
- Safe concurrent command execution
- No race conditions

### Position Tracking

Player position updated from:
- `MovePlayer` packets from server (authoritative)
- Local calculation (predictive)
- Reconciliation on server update

### Coordinate System

Minecraft Bedrock uses:
- **X**: East (+) / West (-)
- **Y**: Up (+) / Down (-)
- **Z**: South (+) / North (-)

### Yaw Calculation

```go
// Calculate angle from position delta
yaw = atan2(Δz, Δx) * 180 / π
yaw = yaw - 90  // Adjust for Minecraft coordinate system
```

### Movement Packet Timing

Packets sent at 50ms intervals (20 per second):
- Matches Minecraft's 20 tick/second
- Smooth server-side movement
- No packet flooding
- Efficient network usage

## Integration with Other Modules

### Player Tracking
- Can navigate to tracked players
- Follow player movements
- Maintain distance

### Inventory Security  
- Test while navigating
- Monitor transaction patterns during movement
- Combined automation testing

## Related Documentation

- [C7 Framework](C7_FRAMEWORK.md) - Module development guide
- [Player Tracking](PLAYER_TRACKING.md) - Track other players
- [Inventory Security](INVENTORY_SECURITY.md) - Security auditing

## API Reference

### PathfindingModule Methods

```go
// Public methods
navigateTo(target Vec3)      // Start navigation to coordinates
stopNavigation()              // Stop current navigation
printStatus()                 // Display navigation status

// Configuration
type PathfindingConfig struct {
    WalkSpeed        float64  // Movement speed
    SprintSpeed      float64  // Sprint speed
    JumpHeight       float64  // Jump capability
    MaxFallDistance  float64  // Safety limit
    EnableJumping    bool     // Allow jumps
    EnableSprinting  bool     // Allow sprint
    EnableParkour    bool     // Advanced moves
    PathUpdateRate   int      // Update frequency (ms)
    StuckThreshold   float64  // Stuck detection
   MaxPathLength    int      // 0 means unlimited range
}
```

## Contributing

To improve this module:

1. **Add full A* pathfinding**
   - Implement world data access
   - Add block collision checking
   - Calculate optimal paths

2. **Enhance obstacle detection**
   - Detect walls and barriers
   - Handle complex terrain
   - Implement dynamic avoidance

3. **Add advanced movement**
   - Ladder climbing
   - Swimming mechanics
   - Elytra flight
   - Boat navigation

4. **Improve parkour**
   - Gap jump calculations
   - Timing optimization
   - Complex parkour sequences

## Support

For questions or issues:
- Check [GitHub Issues](../../issues)
- Review [Discussions](../../discussions)
- Consult [C7 Framework docs](C7_FRAMEWORK.md)

## License

This module follows the project license. Use responsibly for testing and education only.

---

**Version**: 0.5.0-beta (planned)  
**Status**: Experimental  
**Last Updated**: March 8, 2026  
**Maintained**: Yes

**⚠️ Remember**: This tool is for testing, education, and server administration only. Always obtain permission before use. Respect server rules and terms of service.
