package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	githubAPIURL = "https://api.github.com/repos/rorikonn/MegaCLI/releases/latest"
	userAgent    = "megacli/1.0"
)

// Default is the default [Client].
var Default Client = &github{}

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
	return i.Current == "devel" || i.Current == "unknown" || strings.Contains(i.Current, "dirty") || goInstallRegexp.MatchString(i.Current)
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

// Check checks if a new version is available.
func Check(ctx context.Context, current string, client Client) (Info, error) {
	info := Info{
		Current: current,
		Latest:  current,
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.Current = strings.TrimPrefix(info.Current, "v")
	info.URL = release.HTMLURL
	return info, nil
}

// Apply downloads and installs the specified version, replacing the
// current binary. Returns the new version string.
func Apply(ctx context.Context, version string) (string, error) {
	assetName := buildAssetName(version)

	downloadURL := fmt.Sprintf(
		"https://github.com/rorikonn/MegaCLI/releases/download/v%s/%s",
		version, assetName,
	)

	tmpDir, err := os.MkdirTemp("", "megacli-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(ctx, downloadURL, archivePath); err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}

	binaryPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract update: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable path: %w", err)
	}

	if err := replaceBinary(exe, binaryPath); err != nil {
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	return version, nil
}

func buildAssetName(version string) string {
	var osLabel string
	switch runtime.GOOS {
	case "linux":
		osLabel = "Linux"
	case "darwin":
		osLabel = "Darwin"
	case "windows":
		osLabel = "Windows"
	default:
		osLabel = runtime.GOOS
	}

	var archLabel string
	switch runtime.GOARCH {
	case "amd64":
		archLabel = "x86_64"
	case "arm64":
		archLabel = "arm64"
	default:
		archLabel = runtime.GOARCH
	}

	if runtime.GOOS == "windows" {
		return fmt.Sprintf("megacli_%s_%s_%s.zip", version, osLabel, archLabel)
	}
	return fmt.Sprintf("megacli_%s_%s_%s.tar.gz", version, osLabel, archLabel)
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destDir)
	}
	return extractFromTarGz(archivePath, destDir)
}

func extractFromZip(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var binaryPath string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "megacli" || name == "megacli.exe" {
			binaryPath = filepath.Join(destDir, name)
			out, err := os.Create(binaryPath)
			if err != nil {
				return "", err
			}
			rc, err := f.Open()
			if err != nil {
				out.Close()
				return "", err
			}
			_, err = io.Copy(out, rc)
			rc.Close()
			out.Close()
			if err != nil {
				return "", err
			}
			os.Chmod(binaryPath, 0o755)
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("no binary found in archive")
	}
	return binaryPath, nil
}

func extractFromTarGz(tarPath, destDir string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		name := filepath.Base(header.Name)
		if name == "megacli" || name == "megacli.exe" {
			binaryPath = filepath.Join(destDir, name)
			out, err := os.Create(binaryPath)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return "", err
			}
			os.Chmod(binaryPath, 0o755)
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("no binary found in archive")
	}
	return binaryPath, nil
}

func replaceBinary(currentExe, newBinary string) error {
	if runtime.GOOS == "windows" {
		return replaceBinaryWindows(currentExe, newBinary)
	}

	// On Unix, the running process holds the old exe in memory, so we
	// can rename over it safely.
	if err := os.Rename(newBinary, currentExe); err != nil {
		if err := copyFile(newBinary, currentExe); err != nil {
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}
	return nil
}

func replaceBinaryWindows(currentExe, newBinary string) error {
	// Windows locks running executables. Create a bat script that
	// waits for us to exit, then moves the new binary over the old.
	script := fmt.Sprintf("@echo off\r\n"+
		"timeout /t 2 /nobreak >nul\r\n"+
		"move /y \"%s\" \"%s\" >nul 2>&1\r\n"+
		"del \"%%~f0\" >nul 2>&1\r\n",
		newBinary, currentExe,
	)

	scriptPath := currentExe + ".update.bat"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("failed to create update script: %w", err)
	}

	cmd := exec.Command("cmd", "/c", "start", "", "/min", scriptPath)
	return cmd.Start()
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}

	if err := d.Close(); err != nil {
		return err
	}

	return os.Chmod(dst, 0o755)
}

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Client is a client that can get the latest release.
type Client interface {
	Latest(ctx context.Context) (*Release, error)
}

type github struct{}

// Latest implements [Client].
func (c *github) Latest(ctx context.Context) (*Release, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
