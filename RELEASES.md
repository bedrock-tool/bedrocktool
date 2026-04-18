# Windows GUI Downloads

## Quick Download

Latest release: [Download from GitHub Releases](https://github.com/bedrock-tool/bedrocktool/releases/latest)

Primary asset:
- C7 Proxy Client.exe

Optional asset:
- checksums.txt

## Next Release

Planned tag: v0.5.5-beta

Planned scope:
- Proxy mode is world-download only (worlds)
- Removed proxy command registrations for c7, skins, packs, chat-log, capture, and debug-proxy
- GUI Settings mode list now only shows Worlds
- Proxy settings simplified to world flow (address/listen/client-cache)

Breaking changes:
- Users who relied on removed proxy modes must stay on an older tag or migrate workflows
- Auto-generated release notes now describe worlds-only scope

## Install

1. Download C7 Proxy Client.exe from [latest release](https://github.com/bedrock-tool/bedrocktool/releases/latest)
2. Run the executable
3. Select Worlds mode
4. Connect and capture world data

## Verify checksum (Windows)

PowerShell:
```powershell
Get-FileHash "C7 Proxy Client.exe" -Algorithm SHA256
Get-Content checksums.txt
```

Command Prompt:
```cmd
certutil -hashfile "C7 Proxy Client.exe" SHA256
```

## Release History

Browse all versions: [GitHub Releases](https://github.com/bedrock-tool/bedrocktool/releases)

## Troubleshooting

If startup or connection fails:
- Confirm file downloaded completely
- Allow app in antivirus/firewall
- Verify server address and port
- Confirm account login is valid

## Build it yourself

- [Quick Start Windows](QUICKSTART_WINDOWS.md)
- [Windows GUI Build](WINDOWS_GUI_BUILD.md)
- [Build and Distribution](BUILD_AND_DISTRIBUTION.md)
