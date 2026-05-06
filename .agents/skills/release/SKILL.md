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

Parse the user's keyword to decide which semver component to bump:

| User says (Chinese) | User says (English) | Semver bump |
|---|---|---|
| 大版本 | major | MAJOR + 1, MINOR = 0, PATCH = 0 |
| 小版本 | minor | MINOR + 1, PATCH = 0 |
| patch / 补丁 | patch | PATCH + 1 |

If the user does not specify, default to **patch** (补丁).

Check the latest tag to calculate the new version:

```bash
git tag --sort=-v:refname | head -5
```

Example: latest tag is `v0.4.0`.
- "大版本" → `v1.0.0`
- "小版本" → `v0.5.0`
- "补丁" / no keyword → `v0.4.1`

Version format: `vMAJOR.MINOR.PATCH`. Do NOT ask the user for a version number — compute it automatically from the latest tag and the keyword.

### 3. Security Audit Before Push

**CRITICAL**: Before pushing, scan for sensitive information. Use the internal grep tool (not bash grep) to search for secrets:

```
Pattern: sk-|api_key.*=.*['"][^$\{]|secret.*=.*['"][^$\{]|token.*=.*['"][^$\{]
Include: *.go,*.yml,*.yaml,*.json
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

Check the status at: `https://github.com/rorikonn/MegaCLI/actions`

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
