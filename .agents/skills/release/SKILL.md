---
name: release
description: Use when publishing a new release of MegaCLI — tagging a version, pushing to GitHub, triggering GoReleaser, or updating the release workflow.
---

# Release Skill

Publishes a new version of MegaCLI to GitHub Releases via GoReleaser.

## Prerequisites

- Git remote `origin` must point to `git@github.com:rorikonn/MegaCLI.git`
- The branch must be clean (no uncommitted changes)
- GitHub Actions must have the release workflow (`.github/workflows/megacli-release.yml`)

## Release Process

### 1. Pre-flight Checks

```bash
# Ensure working tree is clean
git status --porcelain

# Ensure we're on the main branch
git branch --show-current

# Ensure remote is configured
git remote get-url origin
```

### 2. Determine Version

Ask the user what version to release, or check the latest tag:

```bash
git tag --sort=-v:refname | head -5
```

Version format: `vMAJOR.MINOR.PATCH` (e.g., `v1.0.0`, `v0.2.1`)

### 3. Security Audit Before Push

**CRITICAL**: Before pushing, scan for sensitive information:

```bash
# Check for API keys, secrets, tokens
grep -rn "sk-\|api_key.*=.*['\"][^$]\|secret.*=.*['\"][^$]\|token.*=.*['\"][^$]" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.json" . | grep -v "go.sum" | grep -v ".git/"
```

If any real credentials are found, **STOP** and alert the user.

### 4. Tag and Push

```bash
# Create annotated tag
git tag -a vX.Y.Z -m "Release vX.Y.Z"

# Push the tag (this triggers the release workflow)
git push origin vX.Y.Z
```

### 5. Verify Release

After pushing the tag, GitHub Actions will:
1. Check out the code
2. Run GoReleaser with `.goreleaser.release.yml`
3. Build binaries for Linux/macOS/Windows (amd64 + arm64)
4. Create a GitHub Release with the binaries attached

Check the status:
```bash
gh run list --workflow=megacli-release.yml --limit=1
```

Or visit: `https://github.com/rorikonn/MegaCLI/actions`

## Configuration Files

| File | Purpose |
|------|---------|
| `.goreleaser.release.yml` | GoReleaser config (builds, archives, changelog) |
| `.github/workflows/megacli-release.yml` | GitHub Actions workflow triggered by tags |
| `scripts/install.ps1` | Windows installer script |
| `scripts/install.sh` | macOS/Linux installer script |

## Install Commands (for users)

**Windows:**
```powershell
irm https://raw.githubusercontent.com/rorikonn/MegaCLI/master/scripts/install.ps1 | iex
```

**macOS / Linux:**
```bash
curl -sSf https://raw.githubusercontent.com/rorikonn/MegaCLI/master/scripts/install.sh | sh
```

## Troubleshooting

- **Workflow not triggered**: Ensure the tag matches `v*.*.*` pattern
- **Build fails**: Check Go version in `go.mod` matches the workflow
- **Binary not found in release**: Check `.goreleaser.release.yml` archive names
- **Permission denied on push**: Ensure SSH key has push access to the repo

## Notes

- The `GITHUB_TOKEN` is automatically provided by GitHub Actions — no secrets setup needed for basic releases
- Binaries are installed to `~/.megacli/bin/` by the install scripts
- The `.goreleaser.release.yml` is separate from the upstream `.goreleaser.yml` (which is Charm's config)
