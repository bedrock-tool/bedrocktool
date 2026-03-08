# Release Process Guide

## How to Create a Release (for Maintainers)

### 1. Tag a Version

Linux/macOS:
```bash
git tag -a v1.0.0 -m "Release version 1.0.0

- New player tracking module
- Improved GUI responsiveness
- Bug fixes and performance improvements"

git push origin v1.0.0
```

Windows Command Prompt:
```cmd
git tag -a v1.0.0 -m "Release version 1.0.0 - New features and bug fixes"
git push origin v1.0.0
```

### 2. Automated Build Triggers

Once you push the tag, GitHub Actions automatically:
1. ✅ Checks out the code
2. ✅ Sets up Go environment
3. ✅ Installs gogio compiler
4. ✅ Builds Windows GUI executable
5. ✅ Generates SHA-256 checksums
6. ✅ Creates release notes
7. ✅ Uploads to GitHub Releases

**Build time**: ~3-5 minutes

### 3. Monitor the Build

1. Go to [GitHub Actions](../../actions)
2. Watch the workflow run
3. Once complete, check [Releases](../../releases)
4. The .exe file will be available for download

## Version Numbering

Follow semantic versioning: `v[MAJOR].[MINOR].[PATCH]`

Examples:
- `v1.0.0` - First release
- `v1.0.1` - Bug fix
- `v1.1.0` - New feature
- `v2.0.0` - Breaking changes

## Release Checklist

- [ ] Code changes merged and tested
- [ ] Version number decided
- [ ] CHANGELOG updated (optional)
- [ ] Git tag created: `git tag -a vX.Y.Z -m "..."`
- [ ] Tag pushed: `git push origin vX.Y.Z`
- [ ] GitHub Actions workflow completes
- [ ] Release appears on [Releases page](../../releases)
- [ ] .exe file downloads successfully
- [ ] SHA-256 checksum verified (optional)
- [ ] Release notes are clear and complete

## Workflow Details

### Trigger Conditions

The workflow runs on:
- ✅ When a tag matching `v*.*.*` is pushed
- ✅ Manual trigger via "Run workflow" button on Actions page

### Build Environment
- **OS**: Ubuntu 24.04 (Linux)
- **Go**: Latest version
- **Target**: Windows 64-bit (amd64)

### Outputs
- `c7client-gui-windows-amd64.exe` - The main executable
- `checksums.txt` - SHA-256 hash for verification
- Release notes - Automatically generated

## Manual Build (If Needed)

If the automated build fails:

```bash
# Install Go and gogio (on a Linux system)
go install gioui.org/cmd/gogio@latest

# Build
mkdir -p builds
gogio \
  -arch amd64 \
  -target windows \
  -version "1.0.0" \
  -icon icon.png \
  -o "builds/c7client-gui-windows-amd64.exe" \
  ./cmd/bedrocktool

# Generate checksum
sha256sum builds/c7client-gui-windows-amd64.exe > builds/checksums.txt

# Manually upload to GitHub release
```

See [WINDOWS_GUI_BUILD.md](WINDOWS_GUI_BUILD.md) for more details.

## Troubleshooting

### Workflow Fails to Run

**Problem**: Tag pushed but workflow doesn't start

**Solutions**:
1. Verify tag matches pattern `v*.*.*`
2. Check [GitHub Actions](../../actions) page
3. Push a new tag if needed

### Build Fails

**Problem**: Workflow runs but build step fails

**Solutions**:
1. Check error message in Actions logs
2. Verify icon.png exists and is readable
3. Check Go/gogio compatibility
4. Try manual build to diagnose
5. Check GitHub Actions secrets if using any

### Wrong Executable Uploaded

**Problem**: Release has wrong .exe or missing files

**Solutions**:
1. Delete the release (GitHub allows this)
2. Delete and re-push the tag
3. Let workflow rebuild

## Future Enhancements

Possible additions:
- [ ] Multiple Windows builds (32-bit, ARM)
- [ ] macOS build
- [ ] Linux AppImage
- [ ] Installer executable (.msi)
- [ ] Code signing with certificate
- [ ] Automatic version detection from code
- [ ] Release notes auto-generated from commits

## Questions?

See:
- [WINDOWS_GUI_BUILD.md](WINDOWS_GUI_BUILD.md) - Build details
- [BUILD_AND_DISTRIBUTION.md](BUILD_AND_DISTRIBUTION.md) - Distribution guide
- [RELEASES.md](RELEASES.md) - User-facing release info
- GitHub Actions documentation

---

**Last Updated**: March 8, 2026  
**Maintained**: Yes  
**Status**: Automated releases active
