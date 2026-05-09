---
name: megacli-agent
description: Use when the user wants to create a new agent or subagent — defining its role, model, tools, and prompt template. Triggers include "create agent", "new agent", "add agent", "new subagent", "创建 agent", "新建 agent".
---

# Create Agent Skill

Help the user define a new agent or subagent for MegaCLI. There are two ways
to define an agent: **JSON configuration** (simple, quick) and **folder-based
definition** (rich, supports custom prompt + agent-specific skills/subagents).

## Step 1: Gather Requirements

Ask the user the following questions (skip any they already answered):

1. **Name** (ID): A short lowercase identifier (e.g. `plan`, `reviewer`,
   `summarizer`). Must be alphanumeric with hyphens.
2. **Display name**: Human-readable name (e.g. "Plan Agent", "Code Reviewer").
3. **Description**: One sentence describing what this agent does.
4. **Role**: `primary` (user can switch to it) or `subagent` (internal only,
   invoked by the agent tool).
5. **Model**: `large` (powerful, expensive) or `small` (fast, cheap).
6. **Tools**: One of:
   - "all" — full tool access (default for primary agents)
   - "read-only" — only `glob`, `grep`, `ls`, `view`, `sourcegraph`
   - Custom list — user specifies which tools to include
7. **Custom prompt?**: Whether they want a custom system prompt template.
   If not, the agent will use the default `coder.md.tpl` template.
8. **Agent-specific skills?**: Whether they need skills scoped only to this
   agent (folder-based definition required).

Based on complexity, recommend the appropriate definition method:
- **JSON**: Simple agents, no custom prompt, no agent-specific skills.
- **Folder-based**: Custom prompt template, agent-specific skills, or
  subagents bundled with the agent.

## Path Rules

**NEVER use absolute paths or hardcoded machine-specific information** in
agent definitions, prompt templates, or configuration. Instead:

- Use **relative paths** from the project root (e.g. `.megacli/prompts/reviewer.md.tpl`)
- Use **environment variables** (e.g. `$HOME`, `$MEGACLI_SKILLS_DIR`)
- Use **Go template variables** in prompt templates (e.g. `{{ .WorkingDir }}`,
  `{{ .HomeDir }}`, `{{ .Platform }}`)

This ensures agents are portable across machines and users.

## Step 2a: JSON Configuration (simple)

Produce a JSON snippet for `megacli.json` under the `agents` key:

```json
{
  "agents": {
    "<id>": {
      "name": "<Display Name>",
      "description": "<description>",
      "role": "<primary|subagent>",
      "model": "<large|small>",
      "prompt_file": ".megacli/prompts/<id>.md.tpl",
      "allowed_tools": ["<tool1>", "<tool2>"]
    }
  }
}
```

Rules:
- Omit `prompt_file` if using default prompt.
- Omit `allowed_tools` if using all tools.
- Omit `model` if using `large` (the default).

If the user chose JSON mode, skip to Step 3.

## Step 2b: Folder-Based Definition (rich)

Create a directory structure under `.megacli/agents/<id>/`:

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

### AGENT.md.tpl Format

Uses YAML frontmatter for metadata and the body as the Go template prompt:

```yaml
---
name: <Display Name>
description: <description>
role: <primary|subagent>
model: <large|small>
allowed_tools:
  - <tool1>
  - <tool2>
---

<Go template prompt body here>
```

Frontmatter fields (all optional):

| Field          | Default     | Description                                      |
|----------------|-------------|--------------------------------------------------|
| `name`         | folder name | Display name                                     |
| `description`  | ""          | Short description shown in UI and tool prompts   |
| `role`         | `primary`   | `primary` or `subagent`                          |
| `model`        | `large`     | `large` or `small` model tier                    |
| `allowed_tools`| all tools   | Tool whitelist                                   |
| `allowed_mcp`  | all MCPs    | MCP server/tool whitelist                        |
| `context_paths`| global      | Override context file paths                      |
| `disabled`     | `false`     | Disable the agent                                |

Subagents in the `subagents/` directory automatically get `role: subagent`
regardless of what their frontmatter says.

Skills in the `skills/` directory follow the standard SKILL.md format and
are only available to this agent.

### Discovery Paths

Folder-based agents are discovered from these directories:

**Project-level:**
- `.megacli/agents/`
- `.agents/`
- `.opencode/agents/`

**Global:**
- `~/.config/megacli/agents/`
- `~/.config/agents/`
- `~/.config/opencode/agents/`

### Priority (low to high)

1. Folder-based agents from global directories
2. Folder-based agents from project directories
3. `megacli.json` `agents` block (highest — can override any folder field)

## Step 3: Create Prompt Template (if needed)

For **JSON mode**: create `.megacli/prompts/<id>.md.tpl`.
For **folder mode**: the prompt is the body of `AGENT.md.tpl` itself.

The template uses Go `text/template` syntax. Available variables:

- `{{ .Provider }}` — current provider ID
- `{{ .Model }}` — current model name
- `{{ .WorkingDir }}` — working directory
- `{{ .Platform }}` — OS name
- `{{ .Date }}` — today's date
- `{{ .IsGitRepo }}` — boolean
- `{{ .GitStatus }}` — branch + status summary
- `{{ .ContextFiles }}` — loaded context files (iterate with `range`)
- `{{ .AvailSkillXML }}` — available skills XML

Start from this skeleton and customize based on the agent's purpose:

```
You are a specialized AI assistant for <purpose>.

<env>
Working directory: {{.WorkingDir}}
Platform: {{.Platform}}
Today's date: {{.Date}}
</env>

<rules>
1. <Rule specific to this agent's role>
2. <Another rule>
</rules>
```

For read-only agents, add a rule: "You MUST NOT modify any files. Only read,
analyze, and report."

## Step 4: Apply Configuration

**JSON mode:**
1. Read the existing `megacli.json` (or `.megacli.json`) file.
2. Merge the new agent config into the `agents` block.
3. Write the updated config file.
4. If a prompt template was created, confirm the file path.

**Folder mode:**
1. Create the directory structure.
2. Write the `AGENT.md.tpl` file.
3. Optionally create `skills/` and `subagents/` subdirectories.

## Step 5: Verify

Tell the user:
- How to switch to the agent: `/` menu → Switch Agent → select `<name>`
- Or via CLI: `megacli --agent <id>`
- If it's a subagent: it will be available in the agent tool's target list
  automatically.

## Reference

Full documentation: `docs/agents.md`

### Available Built-in Tools

**Read-only tools** (safe for search/analysis agents):
- `view` — Read file contents, supports line range selection
- `glob` — Find files by glob pattern matching
- `grep` — Search file contents with regex
- `ls` — List directory contents with depth control
- `sourcegraph` — Code search across repositories

**File modification tools:**
- `edit` — Replace a specific text segment in a file
- `multiedit` — Apply multiple edits to the same file atomically
- `write` — Create a new file or overwrite an existing file entirely

**Shell and execution:**
- `bash` — Execute shell commands (supports background jobs)
- `job_output` — View output from background jobs
- `job_kill` — Terminate a background job

**Web and network:**
- `web_fetch` — Fetch and read web page content as markdown
- `web_search` — Search the web for information
- `fetch` — Make HTTP requests (legacy, prefer `web_fetch`)
- `download` — Download a file from a URL to local disk

**Agent orchestration:**
- `agent` — Delegate a task to a subagent
- `switch_agent` — Switch the active primary agent

**Code intelligence (LSP):**
- `lsp_diagnostics` — Get errors/warnings from language servers
- `lsp_references` — Find all references to a symbol
- `lsp_restart` — Restart a language server

**MCP (Model Context Protocol):**
- `list_mcp_resources` — List available MCP resources
- `read_mcp_resource` — Read a specific MCP resource

**Utility:**
- `todos` — Manage a structured task/todo list
- `ask_user` — Ask the user a question and wait for input
- `megacli_info` — Query MegaCLI configuration and environment info
- `megacli_logs` — View MegaCLI diagnostic logs
