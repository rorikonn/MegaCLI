# Agent System

MegaCLI supports multiple agents with distinct system prompts, tool sets, and
model configurations. Agents come in two roles:

- **Primary agents** are user-facing and appear in the Switch Agent menu.
- **Subagents** are only invoked internally by other agents (e.g. the `task`
  agent used for search and context gathering).

## Built-in Agents

| ID     | Role     | Model | Description                                      |
|--------|----------|-------|--------------------------------------------------|
| coder  | primary  | large | Default coding agent with full tool access        |
| task   | subagent | large | Read-only search/context agent used by agent tool |

## Switching Agents

### UI

Press `/` to open the Commands menu, then select **Switch Agent**. A dialog
lists all available primary agents. Select one to hot-swap the system prompt
and tools without losing conversation history.

### CLI Flag

Start MegaCLI with a specific agent:

```bash
megacli --agent planner
megacli -a debug
```

### Programmatic

The `Coordinator` interface exposes:

```go
SwitchAgent(ctx context.Context, name string) error
CurrentAgent() string
AvailableAgents() []string
```

## Defining Custom Agents

There are two ways to define custom agents: **JSON configuration** and
**folder-based definitions**.

### Method 1: JSON Configuration

Add agents in `megacli.json` under the top-level `agents` key:

```json
{
  "agents": {
    "reviewer": {
      "name": "Reviewer",
      "description": "Code review specialist",
      "role": "primary",
      "model": "large",
      "prompt_file": ".megacli/prompts/reviewer.md.tpl",
      "allowed_tools": ["view", "grep", "glob", "ls"]
    },
    "summarizer": {
      "name": "Summarizer",
      "description": "Lightweight summarization subagent",
      "role": "subagent",
      "model": "small",
      "prompt_file": ".megacli/prompts/summarizer.md.tpl",
      "allowed_tools": ["view", "grep"]
    }
  }
}
```

### Agent Fields

| Field          | Type     | Default     | Description                                       |
|----------------|----------|-------------|---------------------------------------------------|
| `name`         | string   | agent ID    | Display name                                      |
| `description`  | string   | —           | Short description shown in UI and tool prompts     |
| `role`         | string   | `"primary"` | `"primary"` or `"subagent"`                       |
| `model`        | string   | `"large"`   | `"large"` or `"small"` model tier                 |
| `prompt`       | string   | agent ID    | Built-in template name (e.g. `"coder"`, `"task"`) |
| `prompt_file`  | string   | —           | Path to a custom `.md.tpl` template on disk        |
| `allowed_tools`| string[] | all tools   | Tool whitelist; empty means all tools              |
| `allowed_mcp`  | object   | all MCPs    | MCP server/tool whitelist                         |
| `context_paths`| string[] | global      | Override context file paths                        |
| `disabled`     | bool     | `false`     | Disable the agent                                 |

### Method 2: Folder-Based Definition

For agents that need a custom prompt template, agent-specific skills, or
bundled subagents, use a folder-based definition. Create a directory under
one of the discovery paths:

```
.megacli/agents/<id>/
├── AGENT.md.tpl              # Required: agent definition + prompt template
├── skills/                   # Optional: agent-specific skills
│   └── <skill-name>/
│       └── SKILL.md
└── subagents/                # Optional: agent-specific subagents
    └── <subagent-id>/
        └── AGENT.md.tpl
```

The `AGENT.md.tpl` file uses YAML frontmatter for metadata and the body as
the Go template prompt:

```yaml
---
name: Reviewer
description: Code review specialist
role: primary
model: large
allowed_tools:
  - view
  - grep
  - glob
  - ls
---

You are a code review specialist.

Working directory: {{ .WorkingDir }}
Platform: {{ .Platform }}
```

All frontmatter fields are optional and share the same defaults as the JSON
agent fields listed above.

**Discovery paths (project-level):**
- `.megacli/agents/`
- `.agents/`
- `.opencode/agents/` *(requires `compat: ["opencode"]`)*

**Discovery paths (global):**
- `~/.config/megacli/agents/`
- `~/.config/agents/`
- `~/.config/opencode/agents/` *(requires `compat: ["opencode"]`)*

**Priority (low to high):**
1. Folder-based agents (global directories)
2. Folder-based agents (project directories)
3. JSON `agents` block in `megacli.json` (highest)

The JSON configuration can override any field from a folder-based definition,
so you can use a folder for the prompt and skills while tweaking settings
via JSON.

### Overriding Built-in Agents

You can override any field on built-in agents. For example, to make the
`task` subagent use the small model:

```json
{
  "agents": {
    "task": { "model": "small" }
  }
}
```

## Writing Prompt Templates

Prompt templates use Go `text/template` syntax and are stored as `.md.tpl`
files. The template receives a `PromptDat` struct with these fields:

| Field           | Type   | Description                        |
|-----------------|--------|------------------------------------|
| `.Provider`     | string | Current provider ID                |
| `.Model`        | string | Current model name                 |
| `.Config`       | Config | Full configuration object          |
| `.WorkingDir`   | string | Current working directory          |
| `.IsGitRepo`    | bool   | Whether the directory is a git repo|
| `.Platform`     | string | Operating system (e.g. `linux`)    |
| `.Date`         | string | Today's date                       |
| `.GitStatus`    | string | Git branch, status, recent commits |
| `.ContextFiles` | []File | Loaded context file contents       |
| `.AvailSkillXML`| string | XML listing of available skills    |

### Resolution Order

1. `prompt_file` (disk path) — if set, loaded from disk.
2. Inline template from `AGENT.md.tpl` — if the agent was defined via a
   folder-based definition, the body of the file is used.
3. `prompt` (built-in name) — looked up as `templates/{name}.md.tpl` in the
   embedded filesystem.
4. Fallback — uses `coder.md.tpl`.

## Agent Tool and Subagent Dispatch

The `agent` tool allows the primary agent to delegate tasks to subagents.
The LLM can specify which subagent to use via the `target` parameter:

```json
{
  "prompt": "Find where the config is loaded",
  "target": "task"
}
```

If `target` is omitted, it defaults to `task`. The tool description
dynamically lists all available subagents with their descriptions.

### Agent Tool vs Delegate Tool

| Aspect       | agent tool               | delegate tool              |
|--------------|--------------------------|----------------------------|
| Targets      | `role: subagent` agents  | `role: primary` agents     |
| Weight       | Lightweight, same Coordinator | Heavyweight, cross-Coordinator |
| Parallelism  | Supports parallel calls  | Sequential                 |
| Session      | Sub-session under parent | Independent session        |
| Cost         | Accumulates to parent    | Separate tracking          |
