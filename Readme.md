# Bedrock Tool

A Minecraft Bedrock proxy focused on one job: downloading and saving worlds from servers.

## Quick Start

### Windows GUI (recommended)

1. Open the latest release: [GitHub Releases](https://github.com/bedrock-tool/bedrocktool/releases/latest)
2. Download C7 Proxy Client.exe
3. Run it and choose Worlds
4. Enter your server address and start

### CLI

```bash
c7client worlds -address play.example.com
```

## Scope

This repository now focuses the proxy flow on world download only.

Included proxy mode:
- worlds: download and save world data

Removed proxy modes:
- c7
- skins
- packs
- chat-log
- capture
- debug-proxy

Other non-proxy utilities (for example merge and realms helpers) remain available if registered in your build.

## Main Docs

- [Windows quick start](QUICKSTART_WINDOWS.md)
- [Windows GUI build guide](WINDOWS_GUI_BUILD.md)
- [Build and distribution](BUILD_AND_DISTRIBUTION.md)
- [Release downloads](RELEASES.md)
- [Release process](RELEASE_PROCESS.md)

## Usage

```text
Usage: c7client <flags> <subcommand> <subcommand args>

Subcommands:
  worlds        download a world from a server
  help          describe subcommands and their syntax
  ...           non-proxy helper commands may also be present
```

## In-Game Commands (worlds mode)

- /setname <name>: set output world name
- /void: toggle void generation behavior
- /exclude-mob <mob...>: add mobs to ignore list
- /save-world: immediately save and rotate world state

## Notes

- Requires a valid Microsoft/Xbox account for normal server auth flows.
- Use only where you have permission and follow server terms.
- Not affiliated with Mojang or Microsoft.

## License

See [LICENSE](LICENSE).
