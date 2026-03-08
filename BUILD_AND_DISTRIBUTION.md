# C7 CLIENT - Build & Distribution Guide

## Overview

C7 CLIENT is a Minecraft Bedrock proxy tool that can be compiled into:
- ✅ **Windows GUI Application** (Standalone .exe)
- ✅ **Command-line Tools** (Windows, macOS, Linux)
- ✅ **Server Applications** (Linux servers)

This guide covers building and distributing the Windows GUI.

## Architecture

```
C7 CLIENT
├── CLI Interface (Command-line)
├── GUI Interface (Graphical - Using Gio)
└── Core Proxy Engine
    ├── C7 Client Module Framework
    │   ├── Player Tracking Module (Active)
    │   └── Future Modules (Extensible)
    └── Traditional Features
        ├── Worlds
        ├── Skins
        ├── Packs
        └── And more...
```

## Windows GUI Build

### Option 1: Automated Build (Easiest)

#### Using PowerShell

```powershell
# Navigate to project directory
cd C:\path\to\c7client

# Run build script
.\build-windows-gui.ps1

# Optional: Build both 32-bit and 64-bit
.\build-windows-gui.ps1 -Build32Bit
```

#### Using Command Prompt

```cmd
cd C:\path\to\c7client
build-windows-gui.bat
```

### Option 2: Manual Build (Advanced)

```powershell
# 1. Install Go (if not already done)
# Visit: https://golang.org/dl/

# 2. Install build tool
go install gioui.org/cmd/gogio@latest

# 3. Build for Windows 64-bit
gogio -arch amd64 -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-amd64.exe" ./cmd/bedrocktool

# 4. Optional: Build for Windows 32-bit
gogio -arch 386 -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-386.exe" ./cmd/bedrocktool
```

## Build Requirements

### Minimum Requirements
- Windows 7 SP1 or later
- 2 GB RAM
- 500 MB free disk space
- Go 1.24 or later
- Internet connection (for downloading dependencies)

### Recommended
- Windows 10/11
- 8 GB RAM
- 2 GB free disk space
- SSD (faster builds)
- Git (for version information)

## Build Output

### Files Generated

```
builds/
├── c7client-gui-windows-amd64.exe    (64-bit - Recommended)
└── c7client-gui-windows-386.exe      (32-bit - Legacy support)
```

### Size & Performance

| Version | Size | Compatibility |
|---------|------|---------------|
| 64-bit (amd64) | ~35 MB | Modern systems, Windows 10/11 |
| 32-bit (386) | ~30 MB | Legacy systems, older Windows |

## Distributing Your Build

### Creating a Portable Release

1. **Create Release Folder**
```cmd
mkdir c7client-release
cd c7client-release
copy ..\builds\c7client-gui-windows-amd64.exe .
```

2. **Add Documentation**
```cmd
copy ..\QUICKSTART_WINDOWS.md README.txt
copy ..\PLAYER_TRACKING.md FEATURES.txt
copy ..\WINDOWS_GUI_BUILD.md BUILDING.txt
```

3. **Create ZIP Archive**
```powershell
Compress-Archive -Path "." -DestinationPath "c7client-windows-gui.zip"
```

### Release Package Contents

```
c7client-windows-gui/
├── c7client-gui-windows-amd64.exe     (Main application)
├── README.txt                          (Quick start)
├── FEATURES.txt                        (Feature list)
├── BUILDING.txt                        (Build instructions)
└── LICENSE.txt                         (License)
```

### Publishing

**Option 1: GitHub Releases**
1. Go to repository Releases page
2. Create new release
3. Upload the .zip file
4. Add release notes

**Option 2: Direct Download**
1. Host file on web server
2. Share direct link
3. Provide checksum for verification

## Continuous Integration / Automated Builds

### GitHub Actions Example

Create `.github/workflows/build-windows.yml`:

```yaml
name: Build Windows GUI

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Install gogio
        run: go install gioui.org/cmd/gogio@latest
      
      - name: Create builds directory
        run: mkdir builds
      
      - name: Build GUI 64-bit
        run: gogio -arch amd64 -target windows -version ${{ github.ref_name }} -icon icon.png -o "builds/c7client-gui-windows-amd64.exe" ./cmd/bedrocktool
      
      - name: Build GUI 32-bit
        run: gogio -arch 386 -target windows -version ${{ github.ref_name }} -icon icon.png -o "builds/c7client-gui-windows-386.exe" ./cmd/bedrocktool
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: builds/*.exe
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Troubleshooting Build Issues

### Common Problems

| Problem | Cause | Solution |
|---------|-------|----------|
| "Go not found" | Go not installed | Install from golang.org/dl |
| "gogio not found" | Build tool missing | Run `go install gioui.org/cmd/gogio@latest` |
| Build fails halfway | Low disk space | Free up 2GB+ disk space |
| Permission denied | Script execution disabled | Run `Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser` |
| Version mismatch | Go version too old | Update Go to 1.24+ |

### Checking System Status

```powershell
# Check Go version
go version

# Check gogio availability
gogio -h

# Check disk space
Get-PSDrive C

# Check available RAM
Get-ComputerInfo -Property CsPhyicallyInstalledSystemMemory
```

## Customization

### Changing Application Icon

1. Create a 512x512 PNG image
2. Replace `icon.png` in project root
3. Rebuild with build script

### Custom Version Number

```powershell
# Build with custom version
gogio -arch amd64 -target windows -version "2.1.0-beta" -icon icon.png -o "builds/c7client-gui-windows-amd64.exe" ./cmd/bedrocktool
```

### Adding Custom Resources

See the `cmd/bedrocktool/` directory for how to add custom resources to the application.

## Performance Optimization

### Build Optimization

```powershell
# The build script automatically optimizes using:
# - Trimpath: Removes file paths for reproducibility
# - Correct architecture targeting
# - Gio's built-in optimization flags
```

### Runtime Optimization

The compiled GUI:
- Runs with minimal overhead
- Uses ~30-50 MB idle memory
- Scales to higher memory usage during operations
- Supports both 32-bit and 64-bit systems

## Security Considerations

### Code Signing (Optional)

For production releases, consider signing the executable:

```powershell
# Requires Code Signing Certificate
signtool sign /f certificate.pfx /p password /t http://timestamp.server.com c7client-gui-windows-amd64.exe
```

### Checksum Verification

Provide SHA-256 checksums:

```powershell
Get-FileHash -Path "builds\c7client-gui-windows-amd64.exe" -Algorithm SHA256
```

## Distribution Channels

### Recommended Channels

1. **GitHub Releases** - Official source, automatic updates
2. **Microsoft Store** (if applicable) - Official app store
3. **Direct website download** - Simple, reliable
4. **Package managers** (winget, chocolatey) - User-friendly

### Not Recommended

- Email distribution (security concerns)
- Sketchy websites (maintenance issues)
- Without integrity checks (security)

## User Installation Instructions

When distributing, provide:

1. **System Requirements**
   - Windows 7 SP1+
   - 512 MB RAM
   - 100 MB free disk

2. **Installation Steps**
   - Download .exe or .zip
   - Extract if needed
   - Double-click to run

3. **First Run Setup**
   - Select utility module
   - Configure server
   - Accept Xbox login
   - Connect

## Updating the Build

### When to Rebuild

1. After code changes
2. After updating Go version
3. After updating dependencies
4. For version updates

### Update Procedure

```powershell
# Update repository
git pull

# Update dependencies
go mod tidy

# Rebuild
.\build-windows-gui.ps1
```

## Monitoring & Support

### Setup Support Channels

1. **GitHub Issues** - Bug reports
2. **Discussions Page** - Q&A
3. **Email Support** - Serious bugs
4. **Discord** (if applicable) - Community help

### Gathering Diagnostics

If users report issues:

```cmd
# Run with debug output
c7client-gui-windows-amd64.exe -debug=true

# Capture logs
c7client-gui-windows-amd64.exe > log.txt 2>&1
```

## Maintenance

### Regular Tasks

- **Weekly**: Check for dependency updates
- **Monthly**: Run security scans
- **Quarterly**: Test on various Windows versions
- **Yearly**: Update Go and dependencies

### Testing Checklist

Before releasing:
- [ ] Build succeeds without errors
- [ ] App launches without crashes
- [ ] All modules work correctly
- [ ] Network connectivity works
- [ ] Login process functions
- [ ] Tested on Windows 10/11
- [ ] Tested on 64-bit system
- [ ] File size reasonable
- [ ] Startup time acceptable
- [ ] No console window appears

## FAQ

**Q: Can I distribute the .exe file freely?**  
A: Yes, if you follow the project's license (check LICENSE file)

**Q: Do users need Go installed?**  
A: No, the .exe is completely standalone

**Q: How do I update the app after distributing?**  
A: Rebuild and release new version; users download the new .exe

**Q: Can I modify the source code?**  
A: Yes, as long as you follow the license requirements

**Q: Is there a license requirement for distribution?**  
A: Check the LICENSE file in the project root

## Additional Resources

- [Windows Build Guide](WINDOWS_GUI_BUILD.md)
- [Quick Start](QUICKSTART_WINDOWS.md)
- [Player Tracking Features](PLAYER_TRACKING.md)
- [Framework Documentation](C7_FRAMEWORK.md)
- [Go Installation Guide](https://golang.org/doc/install)
- [Gio GUI Framework](https://gioui.org)

---

**Happy building!** 🚀

Last Updated: March 8, 2026
