package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromSource_NonExistentDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "does-not-exist")

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix})
	require.NoError(t, err)
	require.Empty(t, cmds)

	// directory must NOT have been created
	_, statErr := os.Stat(dir)
	require.True(t, os.IsNotExist(statErr))
}

func TestLoadFromSource_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.md"), []byte("say hello"), 0o644))

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "user:hello", cmds[0].ID)
	require.Equal(t, "say hello", cmds[0].Content)
}

func TestLoadAll_MixedSources(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(existing, "cmd.md"), []byte("content"), 0o644))

	missing := filepath.Join(t.TempDir(), "nope")

	cmds, err := loadAll([]commandSource{
		{path: existing, prefix: userCommandPrefix},
		{path: missing, prefix: projectCommandPrefix},
	})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "user:cmd", cmds[0].ID)
}

func TestParseFrontmatter_Full(t *testing.T) {
	t.Parallel()

	input := "---\ndescription: Run tests\nagent: build\nmodel: anthropic/claude-sonnet-4-5\n---\n\nRun the full test suite."
	body, fm, ok := parseFrontmatter(input)
	require.True(t, ok)
	require.Equal(t, "Run tests", fm.Description)
	require.Equal(t, "build", fm.Agent)
	require.Equal(t, "anthropic/claude-sonnet-4-5", fm.Model)
	require.Equal(t, "Run the full test suite.", body)
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()

	input := "Just plain content with no frontmatter."
	_, _, ok := parseFrontmatter(input)
	require.False(t, ok)
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	t.Parallel()

	input := "---\n---\n\nBody content here."
	body, fm, ok := parseFrontmatter(input)
	require.True(t, ok)
	require.Empty(t, fm.Description)
	require.Empty(t, fm.Agent)
	require.Empty(t, fm.Model)
	require.Equal(t, "Body content here.", body)
}

func TestParseFrontmatter_PartialFields(t *testing.T) {
	t.Parallel()

	input := "---\ndescription: Quick check\n---\n\nDo a quick check."
	body, fm, ok := parseFrontmatter(input)
	require.True(t, ok)
	require.Equal(t, "Quick check", fm.Description)
	require.Empty(t, fm.Agent)
	require.Empty(t, fm.Model)
	require.Equal(t, "Do a quick check.", body)
}

func TestParseFrontmatter_WindowsLineEndings(t *testing.T) {
	t.Parallel()

	input := "---\r\ndescription: CRLF test\r\n---\r\n\r\nBody with CRLF."
	body, fm, ok := parseFrontmatter(input)
	require.True(t, ok)
	require.Equal(t, "CRLF test", fm.Description)
	require.Equal(t, "Body with CRLF.", body)
}

func TestParseFrontmatter_WithBOM(t *testing.T) {
	t.Parallel()

	input := "\uFEFF---\ndescription: BOM test\n---\n\nBody after BOM."
	body, fm, ok := parseFrontmatter(input)
	require.True(t, ok)
	require.Equal(t, "BOM test", fm.Description)
	require.Equal(t, "Body after BOM.", body)
}

func TestParseFrontmatter_UnclosedDelimiter(t *testing.T) {
	t.Parallel()

	input := "---\ndescription: broken\n\nNo closing delimiter."
	_, _, ok := parseFrontmatter(input)
	require.False(t, ok)
}

func TestLoadCommand_WithFrontmatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "---\ndescription: Test command\nagent: planner\nmodel: openai/gpt-4\n---\n\nRun $ACTION on $TARGET."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0o644))

	cmds, err := loadFromSource(commandSource{path: dir, prefix: projectCommandPrefix})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "Test command", cmds[0].Description)
	require.Equal(t, "planner", cmds[0].Agent)
	require.Equal(t, "openai/gpt-4", cmds[0].Model)
	require.Equal(t, "Run $ACTION on $TARGET.", cmds[0].Content)
	require.Len(t, cmds[0].Arguments, 2)
}

func TestLoadCommand_WithoutFrontmatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "Just run the tests with $COVERAGE."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run.md"), []byte(content), 0o644))

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Empty(t, cmds[0].Description)
	require.Empty(t, cmds[0].Agent)
	require.Equal(t, content, cmds[0].Content)
	require.Len(t, cmds[0].Arguments, 1)
	require.Equal(t, "COVERAGE", cmds[0].Arguments[0].ID)
}
