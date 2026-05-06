package update

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
)

const repo = "rorikonn/MegaCLI"

// Info contains information about an available update.
type Info struct {
	Current string
	Latest  string
	URL     string
}

// Matches a version string like:
// v0.0.0-0.20251231235959-06c807842604
var goInstallRegexp = regexp.MustCompile(`^v?\d+\.\d+\.\d+-\d+\.\d{14}-[0-9a-f]{12}$`)

// IsDevelopment returns true when the current version is a dev build.
func (i Info) IsDevelopment() bool {
	return i.Current == "devel" || i.Current == "unknown" ||
		strings.Contains(i.Current, "dirty") ||
		goInstallRegexp.MatchString(i.Current)
}

// Available returns true if there's an update available.
func (i Info) Available() bool {
	cpr := strings.Contains(i.Current, "-")
	lpr := strings.Contains(i.Latest, "-")
	if cpr && !lpr {
		return true
	}
	if lpr && !cpr {
		return false
	}
	return i.Current != i.Latest
}

// newUpdater creates a configured selfupdate.Updater matching our release
// archive naming: megacli_{version}_{Os}_{arch}.{ext}
// Os is title-cased (Linux, Darwin, Windows), arch uses x86_64 for amd64.
func newUpdater() (*selfupdate.Updater, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, err
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		OS:        osLabel(),
		Arch:      archLabel(),
	})
	if err != nil {
		return nil, err
	}
	return updater, nil
}

// Check checks if a new version is available.
func Check(ctx context.Context, current string) (Info, error) {
	current = strings.TrimPrefix(current, "v")
	info := Info{
		Current: current,
		Latest:  current,
	}

	updater, err := newUpdater()
	if err != nil {
		return info, fmt.Errorf("failed to create updater: %w", err)
	}

	release, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repo))
	if err != nil {
		return info, fmt.Errorf("failed to detect latest version: %w", err)
	}
	if !found {
		return info, nil
	}

	info.Latest = release.Version()
	info.URL = release.ReleaseNotes
	return info, nil
}

// Apply downloads and installs the specified version, replacing the
// current binary. Returns the new version string.
func Apply(ctx context.Context, version string) (string, error) {
	updater, err := newUpdater()
	if err != nil {
		return "", fmt.Errorf("failed to create updater: %w", err)
	}

	release, found, err := updater.DetectVersion(ctx, selfupdate.ParseSlug(repo), "v"+version)
	if err != nil {
		return "", fmt.Errorf("failed to detect version %s: %w", version, err)
	}
	if !found {
		return "", fmt.Errorf("version %s not found", version)
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	if err := updater.UpdateTo(ctx, release, exe); err != nil {
		return "", fmt.Errorf("failed to apply update: %w", err)
	}

	return release.Version(), nil
}

func osLabel() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func archLabel() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}
