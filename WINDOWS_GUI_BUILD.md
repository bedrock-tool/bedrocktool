# C7 CLIENT Windows GUI Build Guide

## Overview

C7 CLIENT can be compiled into a standalone Windows GUI application. This guide walks you through the entire process.

## Prerequisites

### Required Software

1. **Go 1.24 or later**
   - Download from: https://golang.org/dl/
   - Installation guide: https://golang.org/doc/install
   - Verify installation: Open Command Prompt and run `go version`

2. **Git** (recommended, for version information)
   - Download from: https://git-scm.com/download/win
   - Or use: `winget install Git.Git` (if you have Windows Package Manager)

3. **Windows 10/11** with at least 2GB free disk space

### System Requirements

| Requirement | Minimum | Recommended |
|------------|---------|-------------|
| OS | Windows 7 SP1 | Windows 10/11 |
| RAM | 512 MB | 2 GB |
| Disk Space | 100 MB | 500 MB |
| Processor | 1 GHz | 2 GHz+ |
| Architecture | 32-bit or 64-bit | 64-bit |

## Build Methods

There are two ways to build the GUI on Windows:

### Method 1: PowerShell Script (Recommended)

#### Step 1: Open PowerShell

1. Press `Win + X` and select "Windows PowerShell (Admin)"
   - Or: Search for "PowerShell" and run as Administrator
   - Or: Open Command Prompt and type `powershell`

2. Navigate to the project directory:
```powershell
cd C:\path\to\bedrockworld
```

#### Step 2: Run the Build Script

```powershell
# Allow script execution (one-time command)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force

# Run the build script
.\build-windows-gui.ps1
```

#### Step 3: Wait for Build to Complete

The build process typically takes 2-5 minutes depending on system speed.

You'll see output indicating the build progress:
```
[✓] Go is installed
[✓] gogio is available
[✓] builds directory ready
[✓] Git tag: v1.2.3
Building C7 CLIENT GUI for Windows (amd64)...
```

#### Optional: Build 32-bit Version

```powershell
.\build-windows-gui.ps1 -Build32Bit
```

This creates both 64-bit and 32-bit executables.

### Method 2: Batch Script

If you prefer a simpler method, use the batch script:

#### Step 1: Open Command Prompt

1. Press `Win + R`, type `cmd`, and press Enter
2. Navigate to the project:
```cmd
cd C:\path\to\bedrockworld
```

#### Step 2: Run the Build Script

```cmd
build-windows-gui.bat
```

#### Step 3: Follow Prompts

The script will ask if you want to build a 32-bit version as well.

### Method 3: Manual Build with Go

For advanced users who want direct control:

```powershell
# Install gogio if not already installed
go install gioui.org/cmd/gogio@latest

# Build 64-bit GUI
gogio -arch amd64 -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-amd64.exe" ./cmd/bedrocktool

# Optionally, build 32-bit
gogio -arch 386 -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-386.exe" ./cmd/bedrocktool
```

## Build Output

After successful build, you'll find your executable in the `builds/` directory:

```
builds/
├── c7client-gui-windows-amd64.exe    (64-bit version - recommended)
└── c7client-gui-windows-386.exe      (32-bit version - optional)
```

### File Sizes

- **64-bit executable**: ~30-40 MB
- **32-bit executable**: ~25-35 MB

## Running the Application

### Method 1: Direct Execution

1. Navigate to the `builds` folder
2. Double-click `c7client-gui-windows-amd64.exe`
3. The GUI window will open automatically

### Method 2: Command Line

```cmd
.\builds\c7client-gui-windows-amd64.exe
```

### Method 3: Create a Shortcut

1. Right-click on the executable
2. Select "Send to" → "Desktop (create shortcut)"
3. Double-click the shortcut to run

## GUI Features

Once the application is running, you'll have access to:

### Available Utilities

- **C7 Client** - Modular utility features
  - Player Tracking - Track players with distance and direction
  - Future modules coming soon

- **Worlds** - Capture worlds from servers
- **Skins** - Download player skins
- **Packs** - Download resource packs
- And more...

### Main Interface

The GUI provides:
- Easy-to-use utility selection menu
- Server connection configuration
- Real-time status updates
- Logging and error messages
- Settings and preferences

## Troubleshooting

### Build Fails with "gogio not found"

**Solution:**
```powershell
go install gioui.org/cmd/gogio@latest
```

Then try building again.

### Error: "The system cannot find the path specified"

**Solution:**
1. Make sure you're in the correct directory: `C:\path\to\bedrockworld`
2. Verify the directory contains `build.go`, `go.mod`, and `icon.png`
3. Use `cd` to navigate, not just typing the path

### Build Fails Halfway Through

**Solution:**
1. Ensure you have at least 2GB free disk space
2. Close any antivirus programs temporarily
3. Try running the build again
4. If issues persist, try Method 3 (manual build)

### GUI Window Doesn't Open

**Solution:**
1. Check the console for error messages
2. Ensure you're logged into a valid Xbox/Microsoft account
3. Check your internet connection

## Advanced Options

### Custom Icon

To use a custom icon:

1. Prepare a 512x512 PNG image
2. Save it as `icon.png` in the project root
3. Run the build script again

### Version Information

The build system automatically detects your version from git tags:

```powershell
# View current version
git describe --tags

# Build with specific version
gogio -arch amd64 -target windows -version X.Y.Z -icon icon.png -o "builds/c7client-gui-windows-amd64.exe" ./cmd/bedrocktool
```

### Cross-Compilation

You can build for different architectures:

- **64-bit (amd64)** - Recommended for modern systems
- **32-bit (386)** - For older systems

```powershell
# Build for 32-bit
gogio -arch 386 -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-386.exe" ./cmd/bedrocktool

# Build for ARM (if needed)
gogio -arch arm -target windows -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-arm.exe" ./cmd/bedrocktool
```

## Distribution

To share your build:

### Creating a Package

```powershell
# Create a ZIP file for distribution
Compress-Archive -Path "builds/c7client-gui-windows-amd64.exe" -DestinationPath "c7client-windows-gui.zip"
```

### System Requirements for Users

Users who run your application need:
- Windows 7 SP1 or later
- 512 MB RAM (1 GB recommended)
- 100 MB free disk space
- Internet connection for server connectivity

## Performance Tips

### Optimize Performance

1. **Close unnecessary background apps** before running
2. **Use 64-bit version** for better performance
3. **Update Windows** for latest graphics drivers
4. **Use a stable internet connection**

### Memory Usage

- Typical idle: ~30-50 MB
- During world capture: ~100-200 MB
- Peak usage depends on world size

## Next Steps

After building:

1. **Test the application**
   - Run it locally
   - Test different features
   - Verify all modules work

2. **Configure settings**
   - Set default server
   - Configure modules
   - Adjust preferences

3. **Create shortcuts** for easy access

4. **Share with others**
   - Distribute the exe file
   - Create installation guide
   - Provide user documentation

## Getting Help

If you encounter issues:

1. **Check this guide** for common solutions
2. **Review the console output** for error messages
3. **Check system requirements** match your setup
4. **Visit the project repository** for latest information
5. **Check internet connection** and firewall settings

## Security Notes

⚠️ **Important:**

- The application connects to Minecraft servers
- Your Xbox/Microsoft credentials are used for authentication
- Ensure you trust the source of any executable you download
- Run antivirus scans on downloaded binaries

## Building on Other Platforms

For other operating systems:

- **macOS**: Use `./build-macos-gui.sh` (similar process)
- **Linux**: Use `./build-linux-gui.sh` (similar process)
- **Android**: Use `./build-android-gui.sh` (requires Android SDK)

## Technical Details

### Build Technology

- **Language**: Go 1.24+
- **GUI Framework**: Gio (gioui.org)
- **Build Tool**: gogio (Gio compiler)
- **Dependencies**: Automatically managed by Go modules

### Build Types

- **GUI Debug**: Full debugging symbols, slower
- **GUI Release**: Optimized, smaller file size
- **CLI**: Command-line interface (alternative)

## Support for Updates

To rebuild with the latest code:

```powershell
# Update the repository
git pull

# Rebuild
.\build-windows-gui.ps1
```

Your new executable will be in the `builds/` folder with an updated timestamp.

---

**Build Status**: ✅ Windows GUI support is fully functional and tested.

Last Updated: March 8, 2026
