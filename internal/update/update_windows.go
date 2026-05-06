package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// replaceBinaryWindows stages the new binary next to the current exe.
// The actual swap happens on next startup via ApplyPendingUpdate.
func replaceBinaryWindows(currentExe, newBinary string) error {
	stagedPath := currentExe + ".new"
	if err := copyFile(newBinary, stagedPath); err != nil {
		return fmt.Errorf("failed to stage update binary: %w", err)
	}
	return nil
}

// ApplyPendingUpdate checks for a staged update (.new file) and applies it.
// On Windows, a running exe can be renamed but not overwritten or deleted.
// This function renames the current exe out of the way, moves the staged
// binary into place, then cleans up the old file.
// Must be called early at startup before any long-running work.
func ApplyPendingUpdate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	stagedPath := exe + ".new"
	if _, err := os.Stat(stagedPath); os.IsNotExist(err) {
		// No pending update.
		return nil
	}

	oldPath := exe + ".old"

	// Clean up any leftover .old from a previous update.
	_ = os.Remove(oldPath)

	// Rename the running exe out of the way (allowed on NTFS).
	if err := os.Rename(exe, oldPath); err != nil {
		return fmt.Errorf("failed to rename current binary: %w", err)
	}

	// Move staged binary into place.
	if err := os.Rename(stagedPath, exe); err != nil {
		// Try to restore the original.
		_ = os.Rename(oldPath, exe)
		return fmt.Errorf("failed to move staged binary: %w", err)
	}

	// Best-effort cleanup of old binary (may fail because it's us).
	_ = os.Remove(oldPath)

	return nil
}
