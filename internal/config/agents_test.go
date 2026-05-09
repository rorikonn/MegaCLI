package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentDefContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		id      string
		want    Agent
		wantErr bool
	}{
		{
			name: "full frontmatter",
			content: `---
name: My Reviewer
description: Code review specialist
role: primary
model: large
allowed_tools:
  - view
  - grep
  - glob
---

You are a code review specialist.

Working directory: {{ .WorkingDir }}`,
			id: "my-reviewer",
			want: Agent{
				ID:             "my-reviewer",
				Name:           "My Reviewer",
				Description:    "Code review specialist",
				Role:           AgentRolePrimary,
				Model:          SelectedModelTypeLarge,
				AllowedTools:   []string{"view", "grep", "glob"},
				PromptTemplate: "You are a code review specialist.\n\nWorking directory: {{ .WorkingDir }}",
			},
		},
		{
			name: "minimal frontmatter",
			content: `---
description: A simple agent
---

Hello world.`,
			id: "simple",
			want: Agent{
				ID:             "simple",
				Description:    "A simple agent",
				PromptTemplate: "Hello world.",
			},
		},
		{
			name: "subagent role",
			content: `---
name: Lint Checker
description: Runs lint checks
role: subagent
model: small
---

Check code for lint errors.`,
			id: "lint-checker",
			want: Agent{
				ID:             "lint-checker",
				Name:           "Lint Checker",
				Description:    "Runs lint checks",
				Role:           AgentRoleSubagent,
				Model:          SelectedModelTypeSmall,
				PromptTemplate: "Check code for lint errors.",
			},
		},
		{
			name:    "no frontmatter",
			content: "Just some text without frontmatter",
			id:      "bad",
			wantErr: true,
		},
		{
			name: "unclosed frontmatter",
			content: `---
name: Bad
description: Missing closing delimiter`,
			id:      "bad",
			wantErr: true,
		},
		{
			name:    "with BOM and CRLF",
			content: "\uFEFF---\r\nname: BOM Agent\r\ndescription: Handles BOM\r\n---\r\n\r\nPrompt text.",
			id:      "bom-agent",
			want: Agent{
				ID:             "bom-agent",
				Name:           "BOM Agent",
				Description:    "Handles BOM",
				PromptTemplate: "Prompt text.",
			},
		},
		{
			name: "disabled agent",
			content: `---
name: Disabled
description: Should not run
disabled: true
---

This agent is disabled.`,
			id: "disabled",
			want: Agent{
				ID:             "disabled",
				Name:           "Disabled",
				Description:    "Should not run",
				Disabled:       true,
				PromptTemplate: "This agent is disabled.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAgentDefContent([]byte(tt.content), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDiscoverAgentDirs(t *testing.T) {
	t.Parallel()

	t.Run("discovers agent with skills and subagents", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Create agent directory.
		agentDir := filepath.Join(root, "my-agent")
		require.NoError(t, os.MkdirAll(agentDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(agentDir, "AGENT.md.tpl"),
			[]byte("---\nname: My Agent\ndescription: Test agent\nrole: primary\nmodel: large\n---\n\nYou are a test agent."),
			0o644,
		))

		// Create skills subdirectory with a skill.
		skillDir := filepath.Join(agentDir, "skills", "my-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: my-skill\ndescription: A test skill\n---\n\nDo things."),
			0o644,
		))

		// Create subagent.
		subDir := filepath.Join(agentDir, "subagents", "helper")
		require.NoError(t, os.MkdirAll(subDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(subDir, "AGENT.md.tpl"),
			[]byte("---\nname: Helper\ndescription: A helper subagent\nmodel: small\n---\n\nHelp with tasks."),
			0o644,
		))

		agents := DiscoverAgentDirs([]string{root})

		// Main agent.
		agent, ok := agents["my-agent"]
		require.True(t, ok, "my-agent should be discovered")
		assert.Equal(t, "My Agent", agent.Name)
		assert.Equal(t, "Test agent", agent.Description)
		assert.Equal(t, AgentRolePrimary, agent.Role)
		assert.Equal(t, SelectedModelTypeLarge, agent.Model)
		assert.Equal(t, "You are a test agent.", agent.PromptTemplate)
		require.Len(t, agent.SkillsDirs, 1)
		assert.Equal(t, filepath.Join(agentDir, "skills"), agent.SkillsDirs[0])

		// Subagent.
		sub, ok := agents["helper"]
		require.True(t, ok, "helper subagent should be discovered")
		assert.Equal(t, "Helper", sub.Name)
		assert.Equal(t, AgentRoleSubagent, sub.Role)
		assert.Equal(t, SelectedModelTypeSmall, sub.Model)
		assert.Equal(t, "Help with tasks.", sub.PromptTemplate)
	})

	t.Run("later directories override earlier", func(t *testing.T) {
		t.Parallel()
		dir1 := t.TempDir()
		dir2 := t.TempDir()

		// Same agent ID in both directories.
		for _, dir := range []string{dir1, dir2} {
			agentDir := filepath.Join(dir, "overlap")
			require.NoError(t, os.MkdirAll(agentDir, 0o755))
		}

		require.NoError(t, os.WriteFile(
			filepath.Join(dir1, "overlap", "AGENT.md.tpl"),
			[]byte("---\nname: First\ndescription: From dir1\n---\n\nFirst prompt."),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir2, "overlap", "AGENT.md.tpl"),
			[]byte("---\nname: Second\ndescription: From dir2\n---\n\nSecond prompt."),
			0o644,
		))

		agents := DiscoverAgentDirs([]string{dir1, dir2})
		agent, ok := agents["overlap"]
		require.True(t, ok)
		assert.Equal(t, "Second", agent.Name, "later directory should win")
		assert.Equal(t, "Second prompt.", agent.PromptTemplate)
	})

	t.Run("skips directories without AGENT.md.tpl", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Directory without AGENT.md.tpl.
		require.NoError(t, os.MkdirAll(filepath.Join(root, "no-agent"), 0o755))

		// Regular file, not a directory.
		require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello"), 0o644))

		agents := DiscoverAgentDirs([]string{root})
		assert.Empty(t, agents)
	})

	t.Run("nonexistent directory is silently skipped", func(t *testing.T) {
		t.Parallel()
		agents := DiscoverAgentDirs([]string{"/nonexistent/path/that/does/not/exist"})
		assert.Empty(t, agents)
	})
}

func TestMergeFolderAgents(t *testing.T) {
	t.Parallel()

	t.Run("folder agent merges into SetupAgents", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Create a folder-based agent.
		agentDir := filepath.Join(root, ".megacli", "agents", "reviewer")
		require.NoError(t, os.MkdirAll(agentDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(agentDir, "AGENT.md.tpl"),
			[]byte("---\nname: Reviewer\ndescription: Reviews code\nrole: primary\nmodel: large\nallowed_tools:\n  - view\n  - grep\n---\n\nYou review code."),
			0o644,
		))

		cfg := &Config{
			Options: &Options{},
		}
		cfg.workingDir = root
		cfg.SetupAgents()

		// Built-in agents should still exist.
		_, ok := cfg.Agents[AgentCoder]
		require.True(t, ok, "coder should exist")

		// Folder agent should be merged.
		reviewer, ok := cfg.Agents["reviewer"]
		require.True(t, ok, "reviewer should be discovered")
		assert.Equal(t, "Reviewer", reviewer.Name)
		assert.Equal(t, "Reviews code", reviewer.Description)
		assert.Equal(t, []string{"view", "grep"}, reviewer.AllowedTools)
		assert.Equal(t, "You review code.", reviewer.PromptTemplate)
	})

	t.Run("megacli.json overrides folder agent", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Create a folder-based agent.
		agentDir := filepath.Join(root, ".megacli", "agents", "custom")
		require.NoError(t, os.MkdirAll(agentDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(agentDir, "AGENT.md.tpl"),
			[]byte("---\nname: Custom From Folder\ndescription: Folder version\nmodel: large\n---\n\nFolder prompt."),
			0o644,
		))

		cfg := &Config{
			Options: &Options{},
			AgentOverrides: map[string]Agent{
				"custom": {
					Name:        "Custom From JSON",
					Description: "JSON version",
				},
			},
		}
		cfg.workingDir = root
		cfg.SetupAgents()

		custom, ok := cfg.Agents["custom"]
		require.True(t, ok)
		assert.Equal(t, "Custom From JSON", custom.Name, "JSON should override folder")
		assert.Equal(t, "JSON version", custom.Description)
		assert.Equal(t, "Folder prompt.", custom.PromptTemplate, "prompt template from folder should survive if JSON doesn't set prompt_file")
	})
}
