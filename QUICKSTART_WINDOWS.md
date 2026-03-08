# C7 CLIENT - Quick Start Guide

## Windows GUI - Fastest Way to Get Started

### Prerequisites (5 minutes)

1. **Download and Install Go**
   - Visit: https://golang.org/dl/
   - Choose Windows installer (msi)
   - Run the installer and follow the steps
   - **Verify**: Open Command Prompt and type `go version`

### Build (2-5 minutes)

1. **Download C7 CLIENT**
   - Download or clone the repository
   - Extract to a folder (e.g., `C:\C7CLIENT`)

2. **Open PowerShell**
   - Right-click on the folder
   - Select "Open PowerShell hier"
   - Or press `Win + X` and select PowerShell

3. **Run Build Command**
   ```powershell
   .\build-windows-gui.ps1
   ```

4. **Wait for Build to Complete**
   - You'll see progress messages
   - Build takes 2-5 minutes
   - Total size: ~30-40 MB

### Run (1 minute)

Option 1: Double-click `builds/c7client-gui-windows-amd64.exe`

Option 2: In PowerShell:
```powershell
.\builds\c7client-gui-windows-amd64.exe
```

## What You Get

### GUI Features
- ✅ Easy-to-use graphical interface
- ✅ No command line needed
- ✅ Server browser and settings
- ✅ Real-time logging
- ✅ Multiple utility modules

### Available Modules (in GUI)
- **Player Tracking** - Track players with distance/direction
- **Worlds** - Save worlds from servers
- **Skins** - Download player skins
- **Packs** - Download resource packs
- **And more...**

## Commands Inside the GUI

### Player Tracking Module

Once connected to a server:

```
/list-players         - See all online players
/track <name>         - Track a specific player
/track-info           - Show tracked player info
/untrack              - Stop tracking
```

## Troubleshooting

### "PowerShell scripts are disabled"

**Solution:**
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
```

### "Go not found"

**Solution:**
1. Install Go from https://golang.org/dl/
2. Restart your terminal
3. Verify with `go version`

### Build fails halfway

**Solution:**
1. Free up at least 2GB disk space
2. Close antivirus/security tools temporarily
3. Try again or use `build-windows-gui.bat` instead

### GUI window doesn't open

**Solution:**
1. Check console output for errors
2. Make sure you're connected to the internet
3. Try running with admin privileges right-click → Run as administrator

## File Structure

After building, your folder will contain:

```
C7CLIENT/
├── build-windows-gui.ps1  ← Use this to build
├── build-windows-gui.bat  ← Or this
├── builds/
│   └── c7client-gui-windows-amd64.exe  ← Your GUI application!
├── cmd/
├── handlers/
├── utils/
└── ...other files...
```

## Next Steps

1. **Run It** - Double-click the .exe file in `builds/`
2. **Configure** - Set up your server connection in the GUI
3. **Use Modules** - Select utilities you want to use
4. **Connect** - Click Connect and start using!

## Alternative: Command Line

If you prefer command line instead of GUI:

```bash
# Player tracking
c7client c7 -address play.server.com

# Download a world
c7client worlds -address play.server.com

# Download skins
c7client skins -address play.server.com
```

## System Requirements

| Item | Requirement |
|------|-------------|
| OS | Windows 7+ |
| RAM | 512 MB minimum |
| Disk | 200 MB available |
| Internet | Yes, required |

## Additional Resources

- [Detailed Windows Build Guide](WINDOWS_GUI_BUILD.md)
- [Player Tracking Documentation](PLAYER_TRACKING.md)
- [C7 Framework Documentation](C7_FRAMEWORK.md)
- [Project Repository](https://github.com/bedrock-tool/bedrocktool)

## Getting Help

1. Check the console output for error messages
2. Make sure Go is properly installed
3. Ensure you have a stable internet connection
4. Try running as Administrator
5. Check the detailed guides above

---

**That's it!** You now have a working C7 CLIENT GUI application. Enjoy! 🎮
