# bedrocktool

A Minecraft Bedrock proxy that sits between your game and a server and saves
what it sees: worlds (with entities, players, maps and menus), skins and
resource packs. It can also record a session to a `.pcap2` file and replay it
later, so a world can be saved again with different settings without
rejoining the server.

- [Downloads](#downloads)
- [How it works](#how-it-works)
- [Using the GUI](#using-the-gui)
- [Using the command line](#using-the-command-line)
- [Server addresses](#server-addresses)
- [Downloading worlds](#downloading-worlds)
- [Capturing and replaying with pcap2](#capturing-and-replaying-with-pcap2)
- [Scripting](#scripting)
- [Drawing worlds and exporting map art](#drawing-worlds-and-exporting-map-art)
- [Other commands](#other-commands)
- [Where files are saved](#where-files-are-saved)
- [Building from source](#building-from-source)
- [Issues](#issues)

## Downloads

Get the latest build from the [releases page](https://github.com/bedrock-tool/bedrocktool/releases).
There are two builds for each platform:

- **GUI**: double-click it and use the window.
- **CLI**: run it from a terminal with a command and flags.

## How it works

1. bedrocktool logs in to Xbox Live with your account (the first time, it
   shows a code to enter at microsoft.com/link).
2. It connects to the server you picked and starts listening on your
   computer (port `19132` by default).
3. You join the proxy from Minecraft: **Servers → Add Server**, address
   `127.0.0.1` (or this computer's LAN IP when playing on another device),
   port `19132` (or whatever `-listen` was set to).
4. Play as usual. Everything the server sends you passes through
   bedrocktool, which saves what you asked for. Walk around to load more of
   the world.
5. Disconnect (or press Ctrl+C in the terminal). Worlds are saved when you
   leave, when you change dimension, and when you run `/save-world`.

## Using the GUI

Start the GUI build without arguments.

1. Pick a mode at the top: **Worlds**, **Skins** or **Packs**.
2. Under **Server**, type an address or use the buttons to pick a Realm,
   a featured server, an experience or a friend's world.
3. Turn options on or off. Each one says what it does under its name.
4. Press **Start**, then join from Minecraft as described above.

The **Worlds** page shows a live map while you play, and lists each world as
it's saved.

## Using the command line

```
bedrocktool <command> [flags]
```

Running the CLI build with no arguments asks for a command interactively.

| Command | What it does |
|---|---|
| `worlds` | Download the world (see [Downloading worlds](#downloading-worlds)) |
| `skins` | Save the skins of players on a server |
| `packs` | Download a server's resource packs |
| `capture` | Only record a session to a `.pcap2` file |
| `chat-log` | Write the chat to a file |
| `list-realms` | List the Realms your account can join |
| `render` | Draw any saved world (folder or `.mcworld`) from above as a PNG, and export its map art |
| `merge` | Merge several saved worlds into one |
| `dump-actors` | Dump a saved world's entities to JSON (and write them back) |
| `update` | Update bedrocktool to the latest version |

List every command with `bedrocktool -h`, and a command's flags with
`bedrocktool <command> -h`:

```bash
bedrocktool worlds -h
```

Flags are written `-flag value` or `-flag=value`. A true/false flag is turned
on by naming it (`-image`) and off with `=false` (`-save-entities=false`).

### Flags every proxy command has

These work with `worlds`, `skins`, `packs`, `capture` and `chat-log`.

| Flag | Default | Description |
|---|---|---|
| `-address` | | The server to connect to (see [Server addresses](#server-addresses)) |
| `-listen` | `0.0.0.0:19132` | Where the game joins the proxy, e.g. `:19133` to use another port |
| `-capture` | `true` | Also record the session to a `.pcap2` file in `captures/` |
| `-client-cache` | `true` | Let the game cache chunks (faster joins) |
| `-debug` | `false` | Log more, for bug reports |
| `-extra-debug` | `false` | Log every packet to `packet.log` |

### Examples

```bash
bedrocktool worlds -address play.example.net
```

```bash
bedrocktool worlds -address play.example.net:19133 -image -save-players -map-art
```

```bash
bedrocktool worlds -address "realm:My Realm" -chunk-radius 32
```

```bash
bedrocktool skins -address play.example.net -filter "^Steve"
```

```bash
bedrocktool packs -address play.example.net -folders
```

```bash
bedrocktool render -world "worlds/play.example.net/world.mcworld" -out map.png -map-art
```

## Server addresses

`-address` (and the address box in the GUI) takes any of these:

| Form | Example |
|---|---|
| Host name or IP, with its port when it isn't the default | `play.example.net`, `1.2.3.4:19133` |
| A Realm by name or ID | `realm:My Realm`, `realm:1234567` |
| A featured experience | `experience:The Hive` |
| A friend's world (Xbox session ID) | `xbox:00000000-0000-0000-0000-000000000001` |
| A capture to replay | `captures/play.example.net-2026-01-01_12-00-00.pcap2` |

Run `bedrocktool list-realms` to see your Realms' names and IDs.

## Downloading worlds

```bash
bedrocktool worlds -address <server> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `-generator` | `void` | What Minecraft fills the chunks you didn't load with: `void` (empty), `flat` (superflat land) or `normal` (terrain from the server's seed). Many servers send a fake seed or 0, so `normal` terrain often won't match the saved area. The old `-void=false` still works and means `flat` |
| `-image` | `false` | Also save a PNG of the map next to the world |
| `-save-entities` | `true` | Save mobs and other entities |
| `-save-players` | `false` | Save other players as entities wearing their skins. On The Hive, its NPC and advert players are saved either way |
| `-map-art` | `false` | Export the maps hanging in item frames as PNGs next to the world |
| `-save-inventories` | `true` | Save chests and other containers you opened, and your inventory |
| `-block-updates` | `false` | Apply block changes the server sends after a chunk was loaded |
| `-entity-culling` | `false` | Leave out entities that died or were removed (experimental) |
| `-exclude-mobs` | | Entities to leave out, comma separated; `*` works, e.g. `minecraft:zombie,hive:*` |
| `-chunk-radius` | `76` | The render distance to ask the server for |
| `-script` | | A JavaScript file to customise what's saved (see [Scripting](#scripting)) |
| `-record-ui` | `false` | Record the server's menus and scoreboards into the world (see below) |
| `-replay-join` | `false` | When replaying a `.pcap2`, wait for Minecraft to join so you can watch it |

### In-game commands

Type these in chat while connected through the proxy:

| Command | What it does |
|---|---|
| `/setname <name>` | Name the world being saved |
| `/generator <void\|flat\|normal>` | Change what fills the chunks that weren't saved, for this world and the next |
| `/void` | Switch between `void` and `flat` |
| `/exclude-mob <id>` | Leave a mob out of the save, e.g. `/exclude-mob minecraft:bat` |
| `/save-world` | Save now and start a new world |

Swinging your arm with the map item (in your hotbar) zooms the in-game map.

### Recording menus and scoreboards (`-record-ui`)

With **Record Menus** on, bedrocktool watches the menus the server shows you
and how you got to them:

- a menu that opened right after you used an item (a compass, a clock...)
  opens when that item is used in the saved world;
- a menu that opened after you interacted with an entity (an NPC) opens when
  that entity, with the same name tag, is interacted with;
- pressing a button that led to another menu leads to the same menu;
- scoreboards (sidebar, list, below name) come back with the lines they last
  showed.

So open a menu with an item, click through the pages you want, and the saved
world's behavior pack gets a script (`scripts/main.js`, using
`@minecraft/server` and `@minecraft/server-ui`) that rebuilds them. Only the
menus you actually opened are recorded, and buttons only lead where you went
with them. A menu that opened some other way (from a command, for example) is
still in the script; open it with:

```
/scriptevent bedrocktool:form <key>
```

where `<key>` is the menu's key in the script's `FORMS` table. Menus
don't do what the server did when a button was pressed (teleport you, give
items...): they only lead to the next menu.

## Capturing and replaying with pcap2

Every proxy command records the session to `captures/<server>-<date>.pcap2`
unless it's started with `-capture=false`. The `capture` command only does
that. A capture holds everything the server sent, so it can be replayed into
any command later, as if it were the server:

```bash
bedrocktool worlds -address "captures/play.example.net-2026-01-01_12-00-00.pcap2"
```

The server side is played back from the file instead of connecting anywhere,
so no Xbox login is needed. With `worlds`, the replay runs by itself without
Minecraft, as fast as it can be read (a 7 minute session takes seconds); add
`-replay-join` to join it from Minecraft and watch instead. The other
commands wait for Minecraft to join, the same as with a live server. This is
how to:

- save a world again with other settings (`-save-players`, `-map-art`, a
  different `-script` or `-exclude-mobs`...) without going back to the
  server;
- get skins or packs out of a session you already recorded;
- reproduce a bug: attach the `.pcap2` to the issue.

In the GUI, type the capture's path into the address box. Paths can be
absolute, or relative to bedrocktool's folder. Worlds saved from a replay go
to `worlds/<capture name>/`.

## Scripting

A script passed with `-script` (or **Script** in the GUI) can watch
what's received and decide what's saved. Callbacks are registered with
`events.register(name, callback)`; returning `false` from one that allows it
leaves that thing out.

| Event | Callback | Return |
|---|---|---|
| `EntityAdd` | `(entity, metadata, time, isNew)` | `false` to not save the entity |
| `EntityUpdate` | `(update, time)` | |
| `EntityPropertyChange` | `(entity, name, previous, newValue)` | `false` to ignore the change |
| `PlayerAdd` | `(player, metadata, time)` | `true` to save the player, `false` to leave it out, nothing for the usual rules |
| `ChunkAdd` | `(pos, time)` | `false` to not save the chunk |
| `ChunkData` | `(pos)` | |
| `BlockUpdate` | `(name, properties, pos, time)` | `false` to ignore the change |
| `SpawnParticle` | `(name, pos, time)` | |
| `Packet` | `(name, packet, toServer, time)` | `false` to drop the packet |

`PlayerAdd` gets the player's `UUID`, `Username`, `XUID`, `IsNPC` (listed
without an XUID: the server made it up), `HasSkin`, `RuntimeID`, `UniqueID`
and `Position`. Its answer wins over `-save-players` and the Hive rule:

```js
// save only the NPCs whose name tag mentions "Shop"
events.register('PlayerAdd', (player, metadata) => {
    if (!player.IsNPC) return false;
    return String(metadata.Name ?? '').includes('Shop');
});
```

Scripts can also call `displayChatMessage(text)`, `setIngameMap(enabled)`,
read loaded chunks with `chunks.get(x, z)` and write files with
`fs.create(name)`. See [scripting/bedrocktool.d.ts](scripting/bedrocktool.d.ts)
for every type, and [scripting/example.js](scripting/example.js) to start
from. Point your editor at the `scripting` folder for completions.

## Drawing worlds and exporting map art

The CLI's `render` command works on any saved Bedrock world, not just ones
bedrocktool downloaded: a world folder (the one with `db/` and `level.dat`
in it) or a `.mcworld` file.

```bash
bedrocktool render -world "C:/path/to/My World.mcworld" -out "my world.png"
```

**The top-down map** is drawn the same way as the live map: every block's
colour comes from its texture (the server's own for custom blocks, when the
world has its resource packs), grass, leaves and water take their biome's
colours, water shows the ground under it and gets darker the deeper it is,
and the land is lit from the north like a map item, so hills and buildings
stand out. One pixel is one block.

**Map art** is every filled map in the world's item frames and chests,
saved as pictures:

```bash
bedrocktool render -world "C:/path/to/My World.mcworld" -out "my world.png" -map-art
```

This writes a `my world_mapart` folder next to the PNG with:

- `tiles/<map id>.png`: each map as a 128x128 picture;
- `grid_x<x>_y<y>_z<z>.png`: each wall of maps (two or more frames side by
  side, facing the same way) stitched into one picture, the way you see it
  standing in front of the wall. Floors and ceilings of maps work too.

| Flag | Default | Description |
|---|---|---|
| `-world` | | The world: a folder or a `.mcworld` file |
| `-out` | `world.png` | Where to write the top-down map |
| `-map-art` | `false` | Also export the map art |
| `-map-art-out` | `<out>_mapart` | Where to put the map art |
| `-no-image` | `false` | Skip the top-down map (to only export map art) |

A map only has a picture if its pixels were saved in the world. Maps the
game drew itself always have them. In worlds bedrocktool downloads, a map's
pixels are there when the server sent them, which it does when the map is
in view; `render -map-art` says when it finds maps without pixels. A world
saved by a version of bedrocktool from before map saving won't have them,
but replaying its `.pcap2` with `worlds` saves it again with them.

`worlds -map-art` exports the same pictures straight away while
downloading.

## Other commands

- `merge -out <folder> <world> <world>...`: merges worlds (for example the
  pieces of a map saved over several sessions). `-bounds` shows where each
  one is.
- `dump-actors -world <world folder>`: writes the world's entities to
  `actors.json`; `-write` writes the file back into the world.
- `chat-log -address <server>`: writes the chat to a file.

## Where files are saved

Next to bedrocktool (the folder it's started from):

| Folder | What's in it |
|---|---|
| `worlds/<server>/` | Saved worlds, as folders and `.mcworld` files to open in Minecraft; map PNGs and `_mapart` folders when turned on |
| `captures/` | `.pcap2` recordings |
| `skins/` | Saved skins |
| `packs/` | Downloaded resource packs |

## Building from source

You need Go (the version in [go.mod](go.mod)).

```bash
go build ./cmd/bedrocktool
```

Pack decryption is added with the `packs` build tag; its code is in a
git-crypt encrypted part of the repository, so that tag only builds for those
with the key. The release builds (GUI included) are made with
[build.go](build.go):

```bash
go install gioui.org/cmd/gogio@latest
```

```bash
go run ./build.go windows gui,cli
```

## Issues

If something crashes or doesn't save right, please
[open an issue](https://github.com/bedrock-tool/bedrocktool/issues) with a
screenshot or the log, and the `.pcap2` capture of the session if you can.
