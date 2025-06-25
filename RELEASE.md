# Release Guide

This guide explains how to create and publish releases for Easy SSH Tunnel Manager.

## Quick Release

For automated releases using GitHub Actions:

1. **Create a tag and push:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically:**
   - Build for all platforms
   - Create release packages
   - Generate checksums
   - Create GitHub release with artifacts

## Manual Release

If you need to create a release manually:

### 1. Build All Platforms

```bash
# Clean previous builds
make clean

# Build all platforms
make build-all

# Create packages
make release
```

### 2. Upload to GitHub

1. Go to https://github.com/ivikasavnish/easytunnel/releases
2. Click "Create a new release"
3. Choose your tag version (e.g., `v1.0.0`)
4. Upload all files from `dist/v1.0.0/`
5. Publish the release

## Platform-Specific Instructions

### Debian/Ubuntu Package

The automated build creates a `.deb` package:

```bash
make deb
```

Users can install with:
```bash
wget https://github.com/ivikasavnish/easytunnel/releases/download/v1.0.0/easytunnel-v1.0.0-amd64.deb
sudo dpkg -i easytunnel-v1.0.0-amd64.deb
```

### macOS Homebrew

1. Update the formula in `packaging/easytunnel.rb`
2. Calculate SHA256 checksums:
   ```bash
   shasum -a 256 dist/v1.0.0/easytunnel-v1.0.0-darwin-*.tar.gz
   ```
3. Submit to homebrew-core or create your own tap

### Windows

Windows users can:
1. Download the `.zip` file
2. Extract to desired location
3. Add to PATH
4. Or use the `install-windows.bat` script

## Pre-Release Checklist

- [ ] Update version in `go.mod` if needed
- [ ] Update `README.md` with new features
- [ ] Update `QUICKSTART.md` if needed
- [ ] Test on multiple platforms
- [ ] Verify SSH tunnel functionality
- [ ] Check web interface works
- [ ] Verify debug scripts work

## Post-Release Tasks

- [ ] Update documentation
- [ ] Announce on relevant channels
- [ ] Update Homebrew formula if using a tap
- [ ] Monitor for issues

## Version Numbering

We use semantic versioning (semver):
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)

## Rollback

If a release has critical issues:

1. **Mark as pre-release** in GitHub
2. **Create hotfix** with patch version
3. **Test thoroughly** before re-releasing

## Build Environment

The release builds are reproducible and include:
- Version information embedded in binary
- Build timestamp
- Git commit hash
- Static linking for portability

All builds are performed with:
- Go 1.21+
- CGO_ENABLED=0 for static binaries
- Cross-compilation for all supported platforms
