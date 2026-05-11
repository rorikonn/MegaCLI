package backend

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//go:embed gitignore/default
var defaultGitIgnore string

func createDotCrushDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitIgnorePath)

	switch {
	case os.IsNotExist(err):
		// First run — create the gitignore.
	case err != nil:
		slog.Warn("Failed to read .gitignore file", "path", gitIgnorePath, "error", err)
		return nil
	case strings.ReplaceAll(string(content), "\r\n", "\n") == strings.ReplaceAll(defaultGitIgnore, "\r\n", "\n"):
		return nil
	}

	if err := os.WriteFile(gitIgnorePath, []byte(defaultGitIgnore), 0o644); err != nil {
		slog.Warn("Failed to write .gitignore file", "path", gitIgnorePath, "error", err)
	}
	return nil
}
