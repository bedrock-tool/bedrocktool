# Release Process Guide

## Target for Next Release

- Planned tag: v0.5.5-beta
- Release focus: worlds-only proxy mode
- Asset name: C7 Proxy Client.exe

## How to Publish

1. Ensure main branch includes docs and code updates
2. Create an annotated tag
3. Push the tag
4. Verify GitHub Actions build and release upload

Linux/macOS:
```bash
git tag -a v0.5.5-beta -m "Release v0.5.5-beta

World-download-only proxy:
- keep worlds mode
- remove non-world proxy modes
- update GUI and release docs"

git push origin v0.5.5-beta
```

Windows Command Prompt:
```cmd
git tag -a v0.5.5-beta -m "Release v0.5.5-beta worlds-only proxy"
git push origin v0.5.5-beta
```

## Automation

When a tag matching v* is pushed, GitHub Actions will:
- Build Windows GUI executable
- Generate checksums
- Create/update GitHub release
- Upload C7 Proxy Client.exe and checksums.txt

Expected build time: around 3 to 6 minutes.

## Checklist

- [ ] README reflects worlds-only scope
- [ ] RELEASES.md has next-release notes
- [ ] build-windows-gui workflow notes are accurate
- [ ] Tag created and pushed
- [ ] Release assets uploaded
- [ ] Smoke test executable launch on Windows

## Versioning

Current sequence is beta tags:
- v0.5.0-beta ... v0.5.4-beta
- next: v0.5.5-beta

## Rollback

If release assets are incorrect:
1. Delete GitHub release for the tag
2. Re-run workflow_dispatch with release_tag, or retag with a new version
3. Re-verify assets and notes
