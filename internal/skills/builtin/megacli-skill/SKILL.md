---
name: megacli-skill
description: Use when the user wants to create a new Agent Skill for MegaCLI — writing SKILL.md files, choosing skill directories, and following the spec. Triggers include "create skill", "new skill", "add skill", "write skill", "创建 skill", "新建 skill".
---

# Create Skill

Help the user create a new Agent Skill for MegaCLI. Skills are self-contained
instruction sets that the AI agent can load on demand, following the open
[Agent Skills](https://agentskills.io) specification.

## Step 1: Gather Requirements

Ask the user the following questions (skip any they already answered):

1. **Name**: A short lowercase identifier (e.g. `code-reviewer`, `deploy`).
   Must be alphanumeric with hyphens, no leading/trailing/consecutive hyphens.
   Max 64 characters.
2. **Description**: One sentence describing when this skill should be used.
   Include trigger words/phrases. Max 1024 characters.
3. **Scope**: Where should this skill live?
   - **Project-level** — only available in this project
   - **Global** — available across all projects
   - **Agent-specific** — scoped to a single agent (folder-based agent required)
4. **Content**: What should the skill instruct the agent to do? Gather the
   steps, rules, reference material, and examples to include.

## Path Rules

**NEVER use absolute paths or hardcoded machine-specific information** in
skill files. Instead:

- Use **relative paths** from the project root (e.g. `src/config/settings.go`)
- Use **environment variables** when referencing global locations
  (e.g. `$HOME/.config/megacli/`)
- Use **generic placeholders** (e.g. `<project-root>`, `<working-dir>`)

This ensures skills are portable across machines and users.

## Path Convention

Unless the user explicitly specifies a different location:

- **Project-level**: place skill files under `.megacli/skills/` in the project root
- **User-level (global)**: place skill files under `~/.megacli/skills/` (all platforms)
- **Alternatives**: `.agents/skills/`, `.opencode/skills/`, `.cursor/skills/`,
  `.claude/skills/` are supported but should only be used when the user
  explicitly requests them

## Step 2: Choose Location

Based on scope, create the skill directory:

### Project-level skill

```
.megacli/skills/<skill-name>/SKILL.md
```

Other supported project directories (use only if user specifies):
- `.agents/skills/<skill-name>/SKILL.md`
- `.claude/skills/<skill-name>/SKILL.md`
- `.cursor/skills/<skill-name>/SKILL.md`
- `.opencode/skills/<skill-name>/SKILL.md`

### Global skill

```
~/.megacli/skills/<skill-name>/SKILL.md
```

This is `$HOME/.megacli/skills/` on all platforms. Legacy path
`~/.config/megacli/skills/` is still discovered but not recommended for
new skills.

Or override with the `MEGACLI_SKILLS_DIR` environment variable:
```
$MEGACLI_SKILLS_DIR/<skill-name>/SKILL.md
```

You can also add custom directories via `megacli.json`:
```json
{
  "options": {
    "skills_paths": ["~/my-skills", "/shared/team-skills"]
  }
}
```

### Agent-specific skill

Place inside a folder-based agent definition:
```
.megacli/agents/<agent-id>/skills/<skill-name>/SKILL.md
```

These skills are only visible to that agent.

## Step 3: Write SKILL.md

The file has two parts: **YAML frontmatter** (metadata) and **markdown body**
(instructions).

### Format

```markdown
---
name: <skill-name>
description: <when to use this skill, include trigger words>
---

# <Title>

<Instructions for the agent to follow>
```

### Frontmatter Fields

| Field           | Required | Max Length | Description                          |
|-----------------|----------|-----------|--------------------------------------|
| `name`          | Yes      | 64 chars  | Must match the directory name        |
| `description`   | Yes      | 1024 chars| When to trigger; include keywords    |
| `license`       | No       | —         | License identifier                   |
| `compatibility` | No       | 500 chars | Compatible tools/versions            |

### Validation Rules

- `name` **must match** the parent directory name (case-insensitive)
- `name` must be alphanumeric with hyphens (`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)
- Both `name` and `description` are required
- YAML frontmatter must be delimited by `---` on its own line

### Writing Good Instructions

- Be specific and actionable — tell the agent exactly what to do
- Structure with numbered steps for sequential workflows
- Use headings to organize sections (rules, steps, reference, examples)
- Include validation criteria so the agent knows when it's done
- Reference files with **relative paths only** — never absolute paths
- Add trigger phrases in the description so the agent knows when to load
  the skill (e.g. "Use when the user asks to deploy, release, or push to
  production")

### Example

```markdown
---
name: deploy-prod
description: Use when deploying to production. Triggers include "deploy",
  "release", "push to prod", "发布", "上线".
---

# Production Deployment

## Pre-flight Checks

1. Run the test suite: `npm test`
2. Check for uncommitted changes: `git status`
3. Verify the branch is `main`

## Deploy Steps

1. Build the production bundle: `npm run build`
2. Run the deploy script: `./scripts/deploy.sh`
3. Verify the deployment: `curl https://api.example.com/health`

## Rollback

If deployment fails, run: `./scripts/rollback.sh`
```

## Step 4: Verify

After creating the skill:

1. The skill is auto-discovered — no restart needed for project skills.
2. Confirm it appears: run MegaCLI and check the `/skills` command or ask
   the agent to use `megacli_info` to list active skills.
3. To disable a skill without deleting it, add to `megacli.json`:
   ```json
   { "options": { "disabled_skills": ["<skill-name>"] } }
   ```

### Deduplication

If a user skill has the same name as a builtin skill, the user skill wins
(last occurrence in discovery order takes precedence).

## Reference

### Discovery Order

Skills are discovered in this order (later overrides earlier):

1. **Builtin skills** — embedded in the MegaCLI binary
2. **Global skills** — `~/.megacli/skills/` (preferred), `~/.config/megacli/skills/` (legacy)
3. **Project skills** — `.megacli/skills/` (preferred), `.agents/skills/`, etc.
4. **Agent-specific skills** — `<agent-dir>/skills/`

### How Skills Are Used

Skills appear in the agent's system prompt as an XML list. When the agent
determines a skill is relevant to the user's request, it reads the full
SKILL.md file using the `view` tool and follows the instructions.

### Available Built-in Tools

When writing skill instructions, you can reference these tools that the
agent has access to:

**Read-only tools** (safe for search/analysis skills):
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
