# Pre-Compiled Windows GUI Downloads

## 🚀 Quick Download

**Latest Release**: [Download c7client-gui-windows-amd64.exe](../../releases/latest)

Simply download the `.exe` file and double-click to run. No installation, no setup, no dependencies needed.

## 📥 Installation Instructions

### Option 1: Direct Download & Run (Easiest)

1. Go to [Latest Release](../../releases/latest)
2. Download `c7client-gui-windows-amd64.exe`
3. Double-click the file to run
4. Configure your server connection
5. Done! 🎉

**Time required**: 30 seconds

### Option 2: Download & Pin to Start Menu

1. Download the .exe file (see Option 1)
2. Right-click the file
3. Select **"Send to"** → **"Desktop (create shortcut)"**
4. Or pin to Start menu:
   - Right-click → **"Pin to Start"**
5. Launch from Start menu or desktop shortcut

### Option 3: Store in Program Files

1. Create folder: `C:\Program Files\C7 Client\`
2. Move downloaded .exe into that folder
3. Create shortcut on desktop/Start menu

## 🔒 Verify File Integrity

Each release includes SHA-256 checksums for security verification.

### Windows (PowerShell)

```powershell
# Download checksums.txt from the release
Get-FileHash c7client-gui-windows-amd64.exe -Algorithm SHA256

# Compare with checksums.txt
type checksums.txt
```

### Windows (Command Prompt)

```cmd
certutil -hashfile c7client-gui-windows-amd64.exe SHA256
```

Compare the output with the checksum in `checksums.txt`

## 📋 Release History

Browse all available versions at [GitHub Releases Page](../../releases)

Each release includes:
- ✅ `c7client-gui-windows-amd64.exe` - The Windows GUI application
- ✅ `checksums.txt` - SHA-256 verification hash
- 📖 Release notes with changelog and features

## 🆚 Version Comparison

| Version | Date | Features | Download |
|---------|------|----------|----------|
| Latest | Check release page | All current features | [Latest Release](../../releases/latest) |
| All versions | Various | Historical releases | [All Releases](../../releases) |

## ⚙️ System Requirements

- **OS**: Windows 7 SP1 or later
- **RAM**: 512 MB minimum (1 GB recommended)
- **Disk Space**: ~100 MB
- **Internet**: Optional (for connecting to servers)
- **Administrator**: Not required

## 🎯 First Run

1. **Double-click** the .exe file
2. **Allow** Windows Defender/antivirus if prompted
3. **Grant** network access when asked
4. **Configure** your server connection
5. **Enjoy!** 🎮

**Typical startup time**: <2 seconds

## 🆘 Troubleshooting

### Application Won't Start

**Symptom**: Double-clicking does nothing or shows error

**Solutions**:
1. Ensure file downloaded completely (check file size: ~35 MB)
2. Try running as Administrator:
   - Right-click file → **"Run as Administrator"**
3. Disable antivirus temporarily and try again
4. Ensure Windows is up to date (Windows Update)

### Windows Defender Warning

**Symptom**: "Windows protected your PC" message

**Solution**:
1. Click **"More info"**
2. Click **"Run anyway"**
3. This is normal for new applications from the internet

### Network Connection Issues

**Symptom**: Can't connect to server

**Solutions**:
1. Verify internet connection works
2. Check firewall allows the application
3. Ensure server address and port are correct
4. Try connecting from another device to verify server is online

### Performance Issues

**Symptom**: App runs slowly or lags

**Solutions**:
1. Close other applications to free up RAM
2. Ensure internet connection is stable
3. Check Task Manager for CPU/RAM usage
4. Restart the application

## 🔄 Updating

When a new version is released:

1. Download the latest .exe from [Releases](../../releases/latest)
2. Replace the old file (or keep separate versions)
3. Run the new version

**Note**: Old and new versions use separate configuration files, so you can keep both installed

## 🏗️ Build Yourself

Don't want to download? Build the .exe yourself:

- [Quick Start Guide](QUICKSTART_WINDOWS.md) - 5 minutes
- [Full Build Guide](WINDOWS_GUI_BUILD.md) - Complete instructions
- [Windows GUI Complete](WINDOWS_GUI_COMPLETE.md) - Full summary

## 🌐 Distribution

### For Personal Use
- Download and use the .exe
- Run on your computer
- Share the download link with friends

### For Organizational Use
- Download and distribute internally
- Install on multiple computers
- See [Build and Distribution Guide](BUILD_AND_DISTRIBUTION.md)

### For Custom Builds
- Modify source code
- Build using PowerShell script
- Distribute custom version

## 📊 Download Statistics

You can view download statistics on the [Releases Page](../../releases)

## 📝 Release Notes

Each release includes detailed notes with:
- ✨ New features
- 🐛 Bug fixes
- 📈 Performance improvements
- 🔄 Breaking changes (if any)

## ❓ FAQ

**Q: Is the .exe safe to run?**  
A: Yes, the executable is built from open-source code you can audit. Each build is automated and repeatable.

**Q: Why is the file ~35 MB?**  
A: It includes the full Go runtime and Gio GUI framework as a standalone executable with no external dependencies.

**Q: Can I run multiple versions?**  
A: Yes, each version uses independent configuration, so you can keep old and new side-by-side.

**Q: Do I need to install anything?**  
A: No, the .exe works standalone. No installation, no dependencies, no admin rights needed (usually).

**Q: Can I distribute the .exe?**  
A: Yes, follow the terms of the LICENSE file. Generally you can distribute freely with attribution.

**Q: How often are releases made?**  
A: Depends on development activity. Check the [Releases Page](../../releases) for current frequency.

**Q: Can I use this on Mac/Linux?**  
A: Not directly, but you can build it yourself for those platforms using the build guide.

## 🔗 Related Links

- [Quick Start Guide](QUICKSTART_WINDOWS.md)
- [Build Guide](WINDOWS_GUI_BUILD.md)
- [GitHub Releases](../../releases)
- [Issue Tracker](../../issues)
- [Discussions](../../discussions)

## ✅ Checklist

- [ ] Downloaded latest .exe from releases
- [ ] Verified file size is ~35 MB
- [ ] Checked SHA-256 checksum (optional)
- [ ] Ran the .exe on Windows PC
- [ ] Configured server settings
- [ ] Successfully connected to server
- [ ] Enjoying the features! 🎉

---

**Latest Version**: Check [Releases](../../releases/latest)  
**Last Updated**: As shown in release dates  
**Maintained**: Yes - automatically built on each release
