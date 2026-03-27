# SESS CLI - Release Process

## Overview

SESS uses GitHub Actions to automatically build and release binaries for multiple platforms. When you push a version tag, the workflow automatically:

1. Builds binaries for 6 platforms (Linux, macOS, Windows - amd64 & arm64)
2. Creates compressed archives (.tar.gz for Unix, .zip for Windows)
3. Generates SHA256 checksums
4. Publishes an `install.sh` asset for one-line installs
5. Creates a GitHub Release with all artifacts
6. Includes installation instructions in the release notes

## Supported Platforms

The release workflow builds for:

- **Linux:** amd64, arm64
- **macOS:** amd64 (Intel), arm64 (Apple Silicon)
- **Windows:** amd64, arm64

All builds use pure Go (no CGO), so they work without additional dependencies.

## Release Workflow

### 1. Prepare for Release

Make sure all changes are committed and pushed to the main branch:

```bash
git checkout main
git pull origin main
```

### 2. Update Version Information

Update version references in documentation:

- `README.md` - Update version number in features section
- `MVP1-SUMMARY.md` - Update if completing a major milestone
- Any other version references

### 3. Create and Push a Version Tag

```bash
# Create a tag (following semantic versioning)
git tag -a v0.2.0 -m "Release v0.2.0 - MVP1 Complete"

# Push the tag to GitHub
git push origin v0.2.0
```

**Tag Format:** `v<major>.<minor>.<patch>`

- Example: `v0.2.0`, `v1.0.0`, `v1.2.3`

### 4. Monitor the Release

1. Go to **Actions** tab in GitHub: `https://github.com/Orctatech-Engineering-Team/Sess/actions`
2. Click on the "Release" workflow run
3. Watch the build process (~3-5 minutes)

### 5. Verify the Release

Once complete:

1. Go to **Releases**: `https://github.com/Orctatech-Engineering-Team/Sess/releases`
2. Verify all 6 binaries are present:
   - `sess-linux-amd64.tar.gz`
   - `sess-linux-arm64.tar.gz`
   - `sess-darwin-amd64.tar.gz`
   - `sess-darwin-arm64.tar.gz`
   - `sess-windows-amd64.zip`
   - `sess-windows-arm64.zip`
3. Verify `checksums.txt` is present
4. Verify `install.sh` is present
5. Check that release notes are generated

### 6. Test the Release

Download and test the binary for your platform:

```bash
# Example for Linux/macOS
curl -fsSL https://github.com/Orctatech-Engineering-Team/Sess/releases/download/v0.2.0/install.sh | sudo bash
sess --version
# Should output: SESS v0.2.0
```

## Versioning Strategy

We follow [Semantic Versioning](https://semver.org/):

### Format: `MAJOR.MINOR.PATCH`

**MAJOR (0.x.x → 1.x.x):**

- Incompatible API changes
- Breaking changes to command interface
- Database schema changes requiring migration

**MINOR (0.0.x → 0.1.x):**

- New features (backward compatible)
- New commands
- Major enhancements

**PATCH (0.0.0 → 0.0.1):**

- Bug fixes
- Performance improvements
- Documentation updates
- Minor tweaks

### Version History

- **v0.1.0** - Initial release with basic session start
- **v0.2.0** - MVP1 - Database persistence, pause/resume, projects listing
- **v0.3.0** - (Planned) Phase 3 - `sess end` command, PR creation
- **v1.0.0** - (Future) Production-ready, stable API

## Pre-release Versions

For testing releases before official launch:

```bash
# Create a pre-release tag
git tag -a v0.2.0-rc1 -m "Release candidate 1 for v0.2.0"
git push origin v0.2.0-rc1
```

Pre-release tags (with `-rc`, `-beta`, `-alpha`) will trigger the same workflow.

Mark the release as "pre-release" in GitHub UI if needed.

## Hotfix Releases

For urgent bug fixes:

1. Create a hotfix branch from the tag:

   ```bash
   git checkout -b hotfix/v0.2.1 v0.2.0
   ```

2. Fix the bug and commit:

   ```bash
   git commit -m "Fix critical bug in pause command"
   ```

3. Create a new patch version tag:

   ```bash
   git tag -a v0.2.1 -m "Hotfix: Fix pause command bug"
   git push origin v0.2.1
   ```

4. Merge back to main:

   ```bash
   git checkout main
   git merge hotfix/v0.2.1
   git push origin main
   ```

## Workflow Files

### Release Workflow (`.github/workflows/release.yml`)

Triggers on: Push to tags matching `v*.*.*`

**Steps:**

1. Checkout code
2. Setup Go 1.25
3. Build binaries for all platforms
4. Create archives (.tar.gz/.zip)
5. Generate SHA256 checksums
6. Attach `install.sh` alongside release artifacts
7. Create GitHub Release with artifacts

**Build flags:**

```bash
go build -ldflags="-s -w"
```

- `-s -w` - Strip debug info (smaller binaries)
- Version info automatically injected via `github.com/earthboundkid/versioninfo/v2` from git tags and commits

### Build Workflow (`.github/workflows/build.yml`)

Triggers on: Push to main/dev, Pull Requests

**Purpose:** Continuous integration - verify builds work

**Steps:**

1. Build on Linux, macOS, Windows
2. Run `go vet`
3. Run tests (if present)
4. Upload artifacts for Linux build

## Manual Release (Without GitHub Actions)

If you need to create a release manually:

```bash
# Ensure you're on the tagged commit
git checkout v0.2.0

# Build for a specific platform
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -o sess \
  ./cmd/sess

# Verify version
./sess --version

# Create archive
tar czf sess-linux-amd64.tar.gz sess

# Generate checksum
sha256sum sess-linux-amd64.tar.gz > checksums.txt
```

Then manually create the release on GitHub and upload the files.

**Note:** The `versioninfo` package automatically reads version from git tags and commits, so you don't need to manually inject version strings.

## Installation Instructions for Users

After a release, users can install with:

### Linux/macOS (recommended)

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/Sess/releases/latest/download/install.sh | sudo bash
```

This installs to `/usr/local/bin/sess` by default.
The installer supports Linux and macOS.

To install into `/usr/bin` instead:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/Sess/releases/latest/download/install.sh | sudo env SESS_INSTALL_DIR=/usr/bin bash
```

To install a specific version:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/Sess/releases/latest/download/install.sh | sudo env SESS_VERSION=v0.2.0 bash
```

### Linux/macOS (manual archive install)

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/Sess/releases/latest/download/sess-linux-amd64.tar.gz | tar xz
sudo install -m 0755 sess /usr/local/bin/sess
```

### Homebrew (Future)

Once we have more releases, create a Homebrew tap:

```bash
brew tap Orctatech-Engineering-Team/sess
brew install sess
```

### Windows (PowerShell)

```powershell
$version = "0.2.0"
Invoke-WebRequest -Uri "https://github.com/Orctatech-Engineering-Team/Sess/releases/download/v$version/sess-windows-amd64.zip" -OutFile "sess.zip"
Expand-Archive sess.zip -DestinationPath $env:USERPROFILE\bin
# Add $env:USERPROFILE\bin to PATH
```

### Windows (Scoop - Future)

```powershell
scoop bucket add orctatech https://github.com/Orctatech-Engineering-Team/scoop-bucket
scoop install sess
```

## Verifying Downloads

Users should verify downloads using checksums:

```bash
# Download checksum file
curl -L https://github.com/Orctatech-Engineering-Team/Sess/releases/download/v0.2.0/checksums.txt -o checksums.txt

# Verify (Linux/macOS)
sha256sum -c checksums.txt --ignore-missing

# Verify (Windows)
Get-FileHash sess-windows-amd64.zip -Algorithm SHA256
# Compare with checksums.txt manually
```

## Troubleshooting

### Build Fails

**Check Go version:**

- Workflow uses Go 1.25
- Ensure go.mod requires the correct version

**Check dependencies:**

```bash
go mod tidy
go mod verify
```

### Release Not Created

**Check tag format:**

- Must match `v*.*.*` (e.g., `v0.2.0`)
- Not `0.2.0` or `version-0.2.0`

**Check permissions:**

- Workflow needs `contents: write` permission
- This is already set in the workflow file

### Binary Size Issues

Binaries are ~15MB due to embedded SQLite. This is normal.

To reduce size further:

- Use UPX compression (risky, can trigger antivirus)
- Remove unused features (not recommended)

## Future Enhancements

### Planned Improvements

1. **Homebrew Tap**
   - Create formula for easy installation
   - Auto-update on new releases

2. **Scoop Manifest** (Windows)
   - Add to scoop bucket
   - Easy Windows installation

3. **APT/YUM Repositories** (Linux)
   - Debian/Ubuntu packages
   - RedHat/Fedora packages

4. **Auto-update Feature**
   - Built-in `sess upgrade` command
   - Check for new versions automatically

5. **Snapcraft** (Linux)
   - Universal Linux package
   - Auto-updates via snap store

6. **Docker Image**
   - Container for CI/CD environments
   - Multi-platform support

## Release Checklist

Before creating a release:

- [ ] All features tested locally
- [ ] Documentation updated (README, ARCHITECTURE, etc.)
- [ ] CHANGELOG updated (if you create one)
- [ ] Version bumped in all necessary places
- [ ] All tests passing (when tests exist)
- [ ] No open critical bugs
- [ ] Release notes drafted
- [ ] Tag created with proper format (`v*.*.*`)
- [ ] Tag pushed to GitHub
- [ ] Release workflow completed successfully
- [ ] Binaries verified on at least one platform
- [ ] Announce release (if applicable)

## Support

If you encounter issues with the release process:

1. Check GitHub Actions logs for error messages
2. Verify tag format is correct
3. Ensure repository permissions are set
4. Open an issue if problems persist

---

**Next Release:** v0.3.0 (Phase 3 - `sess end` command)

**Target Date:** TBD

**Features:** PR creation, rebase automation, session completion
