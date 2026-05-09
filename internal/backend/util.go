package backend

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
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
		return fmt.Errorf("failed to read .gitignore file: %q %w", gitIgnorePath, err)
	case string(content) == defaultGitIgnore:
		return nil
	}

	if err := os.WriteFile(gitIgnorePath, []byte(defaultGitIgnore), 0o644); err != nil {
		return fmt.Errorf("failed to write .gitignore file: %q %w", gitIgnorePath, err)
	}
	return nil
}
