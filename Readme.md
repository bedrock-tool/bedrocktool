# C7 CLIENT
a minecraft bedrock proxy that can among other things save worlds from servers

**Now with Windows GUI Support!** 🎉

<br/>

## 🚀 Quick Start

### ⚡ Windows GUI - Pre-Compiled Executable (Easiest!)

**Skip building entirely - download and run!**

👉 [**→ Download Latest Release**](releases/latest)

1. Download `c7client-gui-windows-amd64.exe`
2. Double-click to run
3. Enjoy! 🎮

**No installation, no dependencies, no setup required.** Just download and play!

[More download options and verification instructions →](RELEASES.md)

---

### Windows GUI - Build Locally (Alternative)

If you prefer to build the executable yourself:

```powershell
# PowerShell (Recommended)
.\build-windows-gui.ps1

# Or Command Prompt
build-windows-gui.bat
```

Your executable: `builds/c7client-gui-windows-amd64.exe`

👉 [**→ 5 Minute Windows GUI Guide**](QUICKSTART_WINDOWS.md)

### Command Line (Advanced)

```bash
c7client worlds -address play.example.com
```

<br/>

## 📚 Documentation

| Guide | Purpose |
|-------|---------|
| [Download Pre-Built Release](RELEASES.md) | Get the .exe directly (easiest!) |
| [Quick Start Windows](QUICKSTART_WINDOWS.md) | Get running in 5 minutes |
| [Windows Build Guide](WINDOWS_GUI_BUILD.md) | Detailed build instructions |
| [Build & Distribution](BUILD_AND_DISTRIBUTION.md) | Building and sharing |
| [Player Tracking](PLAYER_TRACKING.md) | Track players feature || [Inventory Security](INVENTORY_SECURITY.md) | Security audit module || [C7 Framework](C7_FRAMEWORK.md) | Developer framework |

<br/>

## ✨ Features

### Windows GUI Application
✅ Modern graphical interface  
✅ Easy server connection setup  
✅ Real-time status updates  
✅ All utilities accessible  
✅ Standalone executable (no installation)  
✅ Runs on Windows 7+

### C7 CLIENT - Modular Utilities
✅ **Player Tracking**
  - Real-time player positions
  - Distance information
  - Cardinal direction display

✅ **Inventory Security Monitor** 🔒
  - Audit inventory transactions
  - Detect potential vulnerabilities
  - Server security testing
  - Exploit pattern detection
  - ⚠️ For security research only
  
✅ **Extensible Framework**
  - Easy to add new modules
  - Enable/disable modules
  - Clean architecture

### Other Features
✅ **Worlds** - Download and save worlds  
✅ **Skins** - Download player skins  
✅ **Packs** - Download resource packs  
✅ **Chat Logging** - Log server chat  
✅ **Packet Capture** - Analyze network traffic  
✅ **And more...**

<br/>

## 📥 Downloads

### [here](https://github.com/bedrock-tool/bedrocktool/releases)

<br/>

## issues:

if you find an issue or a crash, please report it by opening a github issue with a screenshot of the crash or issue, thanks

<br/>

```
Usage: c7client <flags> <subcommand> <subcommand args>

Subcommands:
        c7               C7 CLIENT modular utilities (player tracking, etc.)
        capture          capture packets in a pcap file
        help             describe subcommands and their syntax
        list-realms      prints all realms you have access to
        merge            merge 2 or more worlds
        packs            download resource packs from a server
        realms-token     print xbl3.0 token for realms api
        skins            download all skins from players on a server
        skins-proxy      download skins from players on a server with proxy
        worlds           download a world from a server


Top-level flags (use "c7client flags" for a full list):
  -debug=false: debug mode (enables extra logging useful for finding bugs)
  -dns=false: enable dns server for consoles (use this if you need to connect on a console)

C7 Client Features:
  The 'c7' subcommand provides modular utility features:
    - Player Tracking: Track other players with distance and direction info
    - More modules coming soon...
  
  Example: c7client c7 -address play.server.com -player-tracking=true
```

## 🎮 Running the App

### Windows GUI

1. Download the executable from **Releases** or build it yourself
2. Double-click `c7client-gui-windows-amd64.exe`
3. Select a utility feature
4. Enter server address
5. Click Connect!

### Command Line

```bash
# Player Tracking
c7client c7 -address play.yourserver.com -player-tracking=true

# Download World
c7client worlds -address play.yourserver.com

# Download Skins
c7client skins -address play.yourserver.com
```

## 🔧 Commands (In-Game)

After connecting to a server:

```
/list-players         # See all online players
/track <name>         # Track a specific player
/track-info           # Show tracked player info
/untrack              # Stop tracking
```

## 📖 Getting Help

- **[Windows Build Issues?](WINDOWS_GUI_BUILD.md#troubleshooting)** - Common solutions
- **[Player Tracking Questions?](PLAYER_TRACKING.md)** - Feature guide
- **[Want to Extend?](C7_FRAMEWORK.md)** - Developer guide
- **[Report a Bug](https://github.com/bedrock-tool/bedrocktool/issues)** - GitHub Issues

## ⭐ System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| OS | Windows 7 SP1 | Windows 10/11 |
| RAM | 512 MB | 2 GB |
| Storage | 100 MB | 500 MB |
| Display | VGA | Full HD |
| Internet | Required | Broadband |

## ⚖️ License

See [LICENSE](LICENSE) file for details.

## 🤝 Contributing

Contributions welcome! Please:
- Fork the repository
- Create a feature branch
- Test your changes
- Submit a pull request

## 📝 Notes

- This is a proxy tool - it connects you to game servers
- Requires valid Xbox/Microsoft account for some features
- Respects Minecraft EULA and server terms of service
- No affiliation with Mojang Studios or Microsoft

---

**Made with ❤️ for the Minecraft Community**

*Last Updated: March 8, 2026*