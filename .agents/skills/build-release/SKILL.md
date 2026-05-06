---
name: build-release
description: Use when building a dev package of MegaCLI, publishing a new release, tagging a version, pushing to GitHub, triggering GoReleaser, or updating the release workflow.
---

# Build & Release Skill

Covers two workflows for MegaCLI: **dev builds** (daily development) and
**releases** (published to GitHub Releases via GoReleaser).

## Version Architecture

Three link-time variables in `internal/version/version.go` control behavior:

| Variable | Dev build | Release build |
|----------|-----------|---------------|
| `Version` | `{tag}-{shortCommit}[-dirty]` | `{tag}` (clean) |
| `Commit` | full commit hash | full commit hash |
| `ReleaseBuild` | `"false"` (default) | `"true"` |

`version.IsRelease()` returns `ReleaseBuild == "true"`. The auto-update in
`internal/app/app.go` (`checkForUpdates`) only runs when `IsRelease()` is
true — dev builds skip it entirely.

---

## Dev Build (开发包)

Produces a binary with a version like `0.4.0-1a2b3c4` (or
`0.4.0-1a2b3c4-dirty` if there are uncommitted changes). Auto-update is
**disabled**.

### Preferred: Taskfile

```bash
task build:dev
```

### Manual Commands

**PowerShell (Windows):**

```powershell
$tag = git describe --tags --abbrev=0
$commit = git rev-parse --short HEAD
$dirty = if (git status --porcelain) { "-dirty" } else { "" }
go build -o megacli.exe -trimpath -ldflags="-s -w -X github.com/megacli/megacli/internal/version.Version=$($tag.TrimStart('v'))-$commit$dirty -X github.com/megacli/megacli/internal/version.Commit=$(git rev-parse HEAD)" .
```

**Bash (Linux/macOS):**

```bash
TAG=$(git describe --tags --abbrev=0)
COMMIT=$(git rev-parse --short HEAD)
DIRTY=$([ -n "$(git status --porcelain)" ] && echo "-dirty" || echo "")
go build -o megacli -trimpath -ldflags="-s -w -X github.com/megacli/megacli/internal/version.Version=${TAG#v}-${COMMIT}${DIRTY} -X github.com/megacli/megacli/internal/version.Commit=$(git rev-parse HEAD)" .
```

### Why Auto-Update Is Disabled

The ldflags do **not** set `ReleaseBuild=true`, so `version.IsRelease()`
returns false. `checkForUpdates()` returns immediately at its entry guard.

---

## Release (正式发布)

Publishes a new version to GitHub Releases via GoReleaser. The binary has a
clean version like `0.4.0` and auto-update is **enabled**.

### Prerequisites

- Git remote `origin` must point to `git@github.com:rorikonn/MegaCLI.git`
- GitHub Actions must have the release workflow
  (`.github/workflows/megacli-release.yml`)
- If there are uncommitted changes, commit them first **without asking the
  user**. Use an appropriate semantic commit message based on the changes.

### 1. Pre-flight Checks

```bash
git status --porcelain
git branch --show-current
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

Version format: `vMAJOR.MINOR.PATCH`. Do NOT ask the user for a version
number — compute it automatically from the latest tag and the keyword.

### 3. Security Audit Before Push

**CRITICAL**: Before pushing, scan for sensitive information. Use the
internal grep tool (not bash grep) to search for secrets:

```
Pattern: sk-|api_key.*=.*['"][^$\{]|secret.*=.*['"][^$\{]|token.*=.*['"][^$\{]
Include: *.go,*.yml,*.yaml,*.json
```

If any real credentials are found, **STOP** and alert the user.

### 4. Tag and Push

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

### 5. Verify Release

After pushing the tag, GitHub Actions will:
1. Check out the code
2. Run GoReleaser with `.goreleaser.release.yml`
3. Build binaries for Linux/macOS/Windows (amd64 + arm64)
4. Create a GitHub Release with the binaries attached

GoReleaser ldflags set `ReleaseBuild=true`, enabling auto-update for release
binaries.

Check the status at: `https://github.com/rorikonn/MegaCLI/actions`

---

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
- **Binary not found in release**: Check `.goreleaser.release.yml` archive
  names
- **Permission denied on push**: Ensure SSH key has push access to the repo

## Notes

- The `GITHUB_TOKEN` is automatically provided by GitHub Actions — no
  secrets setup needed for basic releases
- Binaries are installed to `~/.megacli/bin/` by the install scripts
- The `.goreleaser.release.yml` is separate from the upstream
  `.goreleaser.yml` (which is Charm's config)
