// Package ipc provides cross-process communication between MegaCli instances.
// Instance discovery uses a file-based registry under ~/.megacli/instances/.
package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/megacli/megacli/internal/home"
)

// InstanceInfo describes a running MegaCli instance.
type InstanceInfo struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	CWD       string    `json:"cwd"`
	Agents    []string  `json:"agents"`
	Name      string    `json:"name,omitempty"`
	StartTime time.Time `json:"start_time"`
}

// registryDir returns the path to the instances directory.
func registryDir() string {
	return filepath.Join(home.Config(), "megacli", "instances")
}

// Register writes the current instance's info to the registry.
func Register(info InstanceInfo) error {
	dir := registryDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create registry dir: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.json", info.PID))
	return os.WriteFile(path, data, 0o644)
}

// Unregister removes the current instance from the registry.
func Unregister(pid int) {
	path := filepath.Join(registryDir(), fmt.Sprintf("%d.json", pid))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to unregister instance", "path", path, "error", err)
	}
}

// Discover scans the registry and returns all live instances (excluding self).
// Stale entries (dead PIDs) are cleaned up automatically.
func Discover(selfPID int) []InstanceInfo {
	dir := registryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var instances []InstanceInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var info InstanceInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if info.PID == selfPID {
			continue
		}

		if !isProcessAlive(info.PID) {
			slog.Debug("cleaning up stale instance", "pid", info.PID)
			Unregister(info.PID)
			continue
		}

		instances = append(instances, info)
	}
	return instances
}

// isProcessAlive checks if a process with the given PID exists.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check.
	// On Windows, FindProcess already validates the PID.
	if isWindows() {
		return true
	}
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// InstanceFilePath returns the path to a specific instance file.
func InstanceFilePath(pid int) string {
	return filepath.Join(registryDir(), strconv.Itoa(pid)+".json")
}
