# Player Tracking Compass Feature

## Overview
The player tracking compass feature is part of the **C7 CLIENT** modular utility system. It allows you to track other players on the server with real-time distance and direction information.

## Getting Started

### Using the C7 Client Command
To use player tracking and other C7 CLIENT features, use the `c7` subcommand:

```bash
c7client c7 -address play.example.com
```

Or with specific module settings:

```bash
c7client c7 -address play.example.com -player-tracking=true
```

## Features
- **Real-time Player Tracking**: Track any player on the server
- **Distance Information**: See exact distance to tracked player
- **Direction Compass**: Get cardinal direction (N, S, E, W, NE, etc.)
- **Simple Commands**: Easy-to-use in-game commands
- **Modular Design**: Part of the extensible C7 CLIENT framework

## Commands

### `/list-players`
Lists all players currently online on the server.

**Usage:**
```
/list-players
```

**Example Output:**
```
Online players: Steve, Alex, Herobrine
```

### `/track <player_name>`
Start tracking a specific player. You'll receive distance and direction information.

**Usage:**
```
/track <player_name>
```

**Example:**
```
/track Steve
```

**Output:**
```
Now tracking player: Steve
```

**Note:** Player names are case-insensitive.

### `/track-info`
Display current information about the tracked player (distance and direction).

**Usage:**
```
/track-info
```

**Example Output:**
```
Tracking: Steve | Distance: 123.5m | Direction: NW
```

### `/untrack`
Stop tracking the currently tracked player.

**Usage:**
```
/untrack
```

**Output:**
```
Stopped tracking Steve
```

## How It Works

1. **Start C7 CLIENT** with player tracking enabled:
   ```bash
   c7client c7 -address your.server.com
   ```

2. **Connect to the server** - The proxy will connect you automatically

3. **List players** using `/list-players` to see who's online

4. **Track a player** using `/track Steve`

5. **Check tracking** using `/track-info` to see distance and direction:
   ```
   Tracking: Steve | Distance: 45.2m | Direction: NE
   ```

6. **Stop tracking** using `/untrack` when done

## Module Configuration

Player tracking is enabled by default. You can disable it with:

```bash
c7client c7 -address server.com -player-tracking=false
```

## Technical Details

- **Player Detection**: Automatically detects all players via `AddPlayer` and `PlayerList` packets
- **Position Updates**: Tracks real-time position updates via `MovePlayer` packets  
- **Distance Calculation**: Uses 3D Euclidean distance (includes Y-axis)
- **Direction**: Calculated using `atan2` for accurate cardinal directions
- **Auto-cleanup**: Automatically removes players who disconnect

## Direction Reference

The compass uses 8 cardinal directions:
- **N** - North (0°)
- **NE** - Northeast (45°)
- **E** - East (90°)
- **SE** - Southeast (135°)
- **S** - South (180°)
- **SW** - Southwest (225°)
- **W** - West (270°)
- **NW** - Northwest (315°)

## Troubleshooting

**"Player 'name' not found"**
- Make sure the player is online
- Use `/list-players` to see exact player names
- Player names are case-insensitive but must match exactly

**"Not tracking anyone"**
- You need to use `/track <player_name>` first
- Check if the player is still online with `/list-players`

**Tracked player left notification**
- This appears when a tracked player disconnects
- Tracking automatically stops
- Use `/track` on another player to resume

## Example Workflow

```bash
# Start C7 CLIENT
c7client c7 -address play.example.com

# In-game after connecting:
/list-players
# Output: Online players: Alice, Bob, Charlie

/track Alice
# Output: Now tracking player: Alice

/track-info
# Output: Tracking: Alice | Distance: 87.3m | Direction: E

# Later, to track someone else:
/track Bob
# Output: Now tracking player: Bob

/track-info
# Output: Tracking: Bob | Distance: 23.1m | Direction: SW

# To stop tracking:
/untrack
# Output: Stopped tracking Bob
```

## C7 CLIENT Framework

Player tracking is built on the C7 CLIENT modular framework, which allows for:

- **Multiple Modules**: Run several utility features simultaneously
- **Easy Enable/Disable**: Turn modules on/off via command flags
- **Extensible**: New modules can be added easily
- **Independent**: Each module operates independently
- **Lightweight**: Only enabled modules consume resources

### Future Modules

The C7 CLIENT framework is designed to support additional modules such as:
- Auto-respawn utilities
- Custom overlays
- Server statistics
- Chat enhancements
- And more...

## Compatibility

- ✅ Works with all Minecraft Bedrock servers
- ✅ No client-side mods required
- ✅ Server-agnostic
- ✅ Compatible with other C7 CLIENT modules
- ✅ Lightweight and performant

## Advanced Usage

### Multiple Modules

When more modules are added, you can enable/disable them individually:

```bash
c7client c7 -address server.com -player-tracking=true -future-module=true
```

### Scripting Integration

Future versions may support scripting to automate tracking based on conditions.

## Support

For issues or feature requests, visit the GitHub repository.
