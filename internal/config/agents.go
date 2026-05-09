package config

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/megacli/megacli/internal/home"
	"gopkg.in/yaml.v3"
)

const agentDefFileName = "AGENT.md.tpl"

// agentDefFrontmatter holds the YAML frontmatter fields from an
// AGENT.md.tpl file. Fields mirror config.Agent but use YAML tags.
type agentDefFrontmatter struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	Role         AgentRole           `yaml:"role"`
	Model        SelectedModelType   `yaml:"model"`
	Disabled     bool                `yaml:"disabled"`
	AllowedTools []string            `yaml:"allowed_tools"`
	AllowedMCP   map[string][]string `yaml:"allowed_mcp"`
	ContextPaths []string            `yaml:"context_paths"`
}

// ParseAgentDef parses an AGENT.md.tpl file from disk and returns an
// Agent with PromptTemplate populated from the body. The agent ID is
// derived from the parent directory name.
func ParseAgentDef(path string) (Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, err
	}
	return ParseAgentDefContent(content, filepath.Base(filepath.Dir(path)))
}

// ParseAgentDefContent parses an AGENT.md.tpl from raw bytes. The id
// parameter is used as the agent ID (typically the directory name).
func ParseAgentDefContent(content []byte, id string) (Agent, error) {
	frontmatter, body, err := splitAgentFrontmatter(string(content))
	if err != nil {
		return Agent{}, err
	}

	var def agentDefFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &def); err != nil {
		return Agent{}, fmt.Errorf("parsing agent frontmatter: %w", err)
	}

	agent := Agent{
		ID:             id,
		Name:           def.Name,
		Description:    def.Description,
		Role:           def.Role,
		Model:          def.Model,
		Disabled:       def.Disabled,
		AllowedTools:   def.AllowedTools,
		AllowedMCP:     def.AllowedMCP,
		ContextPaths:   def.ContextPaths,
		PromptTemplate: strings.TrimSpace(body),
	}

	return agent, nil
}

// splitAgentFrontmatter extracts YAML frontmatter and body from an
// AGENT.md.tpl file. Identical algorithm to the skills package.
func splitAgentFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) != ""
	})
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return "", "", errors.New("no YAML frontmatter found")
	}

	endOffset := slices.IndexFunc(lines[start+1:], func(line string) bool {
		return strings.TrimSpace(line) == "---"
	})
	if endOffset == -1 {
		return "", "", errors.New("unclosed frontmatter")
	}
	end := start + 1 + endOffset

	frontmatter = strings.Join(lines[start+1:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}

// DiscoverAgentDirs scans the given directories for agent folder
// definitions. Each immediate subdirectory containing an AGENT.md.tpl
// file is parsed as an agent definition. Agent-specific skills/ and
// subagents/ subdirectories are automatically attached.
//
// Later entries in dirs override earlier ones when agent IDs collide.
func DiscoverAgentDirs(dirs []string) map[string]Agent {
	agents := make(map[string]Agent)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("Failed to read agent directory", "path", dir, "error", err)
			}
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			agentDir := filepath.Join(dir, entry.Name())
			defPath := filepath.Join(agentDir, agentDefFileName)

			if _, err := os.Stat(defPath); err != nil {
				continue
			}

			agent, err := ParseAgentDef(defPath)
			if err != nil {
				slog.Warn("Failed to parse agent definition",
					"path", defPath, "error", err)
				continue
			}

			// Attach agent-specific skills directory if it exists.
			skillsDir := filepath.Join(agentDir, "skills")
			if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
				agent.SkillsDirs = append(agent.SkillsDirs, skillsDir)
			}

			// Discover subagents from the subagents/ subdirectory.
			subagentsDir := filepath.Join(agentDir, "subagents")
			subEntries, err := os.ReadDir(subagentsDir)
			if err == nil {
				for _, subEntry := range subEntries {
					if !subEntry.IsDir() {
						continue
					}
					subDefPath := filepath.Join(subagentsDir, subEntry.Name(), agentDefFileName)
					if _, err := os.Stat(subDefPath); err != nil {
						continue
					}
					subAgent, err := ParseAgentDef(subDefPath)
					if err != nil {
						slog.Warn("Failed to parse subagent definition",
							"path", subDefPath, "error", err)
						continue
					}
					subAgent.Role = AgentRoleSubagent
					agents[subAgent.ID] = subAgent
				}
			}

			slog.Debug("Discovered folder-based agent",
				"id", agent.ID, "dir", agentDir)
			agents[agent.ID] = agent
		}
	}
	return agents
}

// ProjectAgentDirs returns the project-level directories to scan for
// folder-based agent definitions.
func ProjectAgentDirs(workingDir string) []string {
	return []string{
		filepath.Join(workingDir, ".megacli", "agents"),
		filepath.Join(workingDir, ".agents"),
		filepath.Join(workingDir, ".opencode", "agents"),
	}
}

// GlobalAgentDirs returns the global directories to scan for
// folder-based agent definitions.
func GlobalAgentDirs() []string {
	paths := []string{
		filepath.Join(home.Config(), appName, "agents"),
		filepath.Join(home.Config(), "agents"),
		filepath.Join(home.Config(), "opencode", "agents"),
	}

	if runtime.GOOS == "windows" {
		appData := cmp.Or(
			os.Getenv("LOCALAPPDATA"),
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local"),
		)
		paths = append(
			paths,
			filepath.Join(appData, appName, "agents"),
			filepath.Join(appData, "agents"),
		)
	}

	return paths
}
