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

// agentDefFileNames lists accepted agent definition file names in
// priority order. The first match wins when scanning a directory.
var agentDefFileNames = []string{"AGENT.md.tpl", "AGENT.md"}

// findAgentDef returns the path to the first existing agent definition
// file in dir, or an empty string if none is found.
func findAgentDef(dir string) string {
	for _, name := range agentDefFileNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// agentDefFrontmatter holds the YAML frontmatter fields from an
// AGENT.md.tpl (or AGENT.md) file. Fields mirror config.Agent but
// use YAML tags.
type agentDefFrontmatter struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	Role         AgentRole           `yaml:"role"`
	Mode         AgentRole           `yaml:"mode"`
	Model        SelectedModelType   `yaml:"model"`
	Disabled     bool                `yaml:"disabled"`
	AllowedTools []string            `yaml:"allowed_tools"`
	AllowedMCP   map[string][]string `yaml:"allowed_mcp"`
	ContextPaths []string            `yaml:"context_paths"`
}

// ParseAgentDef parses an AGENT.md.tpl (or AGENT.md) file from disk
// and returns an Agent with PromptTemplate populated from the body.
// The agent ID is derived from the parent directory name.
func ParseAgentDef(path string) (Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, err
	}
	return ParseAgentDefContent(content, filepath.Base(filepath.Dir(path)))
}

// ParseAgentDefFile parses a standalone agent definition file (e.g.
// subagents/my-agent.md) and returns an Agent. The id parameter is
// used directly as the agent ID.
func ParseAgentDefFile(path, id string) (Agent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, err
	}
	return ParseAgentDefContent(content, id)
}

// subagentIDFromFile extracts the agent ID from a file name by
// stripping known suffixes (.md.tpl, .md). Returns the id and true
// if the file had a recognised suffix, or ("", false) otherwise.
func subagentIDFromFile(filename string) (string, bool) {
	for _, suffix := range []string{".md.tpl", ".md"} {
		if strings.HasSuffix(filename, suffix) {
			id := strings.TrimSuffix(filename, suffix)
			if id != "" {
				return id, true
			}
		}
	}
	return "", false
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

	// Allow "mode" as an alias for "role"; "role" takes precedence.
	if def.Role == "" && def.Mode != "" {
		def.Role = def.Mode
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

// DiscoverAgentDirs scans the given directories for agent
// definitions. Each immediate subdirectory containing an AGENT.md.tpl
// or AGENT.md file is parsed as an agent definition. Agent-specific
// skills/ and subagents/ subdirectories are automatically attached.
//
// Subagents may be defined as either:
//   - A subdirectory of subagents/ containing an AGENT.md.tpl or
//     AGENT.md file (folder format).
//   - A standalone .md or .md.tpl file directly inside subagents/
//     (file format). The agent ID is derived from the file name.
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
			defPath := findAgentDef(agentDir)
			if defPath == "" {
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
			discoverSubagents(agentDir, agents)

			slog.Debug("Discovered folder-based agent",
				"id", agent.ID, "dir", agentDir)
			agents[agent.ID] = agent
		}
	}
	return agents
}

// discoverSubagents scans agentDir/subagents/ for subagent
// definitions in both folder and file format.
func discoverSubagents(agentDir string, agents map[string]Agent) {
	subagentsDir := filepath.Join(agentDir, "subagents")
	subEntries, err := os.ReadDir(subagentsDir)
	if err != nil {
		return
	}

	for _, subEntry := range subEntries {
		if subEntry.IsDir() {
			// Folder-based subagent: subagents/<name>/AGENT.md(.tpl).
			subDefPath := findAgentDef(filepath.Join(subagentsDir, subEntry.Name()))
			if subDefPath == "" {
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
			continue
		}

		// File-based subagent: subagents/<name>.md or <name>.md.tpl.
		id, ok := subagentIDFromFile(subEntry.Name())
		if !ok {
			continue
		}
		filePath := filepath.Join(subagentsDir, subEntry.Name())
		subAgent, err := ParseAgentDefFile(filePath, id)
		if err != nil {
			slog.Warn("Failed to parse subagent file",
				"path", filePath, "error", err)
			continue
		}
		subAgent.Role = AgentRoleSubagent
		agents[subAgent.ID] = subAgent
	}
}

// ProjectAgentDirs returns the project-level directories to scan for
// folder-based agent definitions.
func ProjectAgentDirs(workingDir string, compat []string) []string {
	paths := []string{
		filepath.Join(workingDir, ".megacli", "agents"),
		filepath.Join(workingDir, ".agents"),
	}
	if HasCompat(compat, CompatOpenCode) {
		paths = append(paths, filepath.Join(workingDir, ".opencode", "agents"))
	}
	return paths
}

// GlobalAgentDirs returns the global directories to scan for
// folder-based agent definitions. ~/.megacli/agents/ is the preferred
// location on all platforms.
func GlobalAgentDirs(compat []string) []string {
	paths := []string{
		filepath.Join(home.DotMegaCLI(), "agents"),
		filepath.Join(home.Config(), appName, "agents"),
		filepath.Join(home.Config(), "agents"),
	}
	if HasCompat(compat, CompatOpenCode) {
		paths = append(paths, filepath.Join(home.Config(), "opencode", "agents"))
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
