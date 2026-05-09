---
name: create-agent
description: Use when the user wants to create a new agent or subagent — defining its role, model, tools, and prompt template. Triggers include "create agent", "new agent", "add agent", "new subagent", "创建 agent", "新建 agent".
---

# Create Agent Skill

Help the user define a new agent or subagent for MegaCLI. Follow these steps
strictly.

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

## Step 2: Generate Configuration

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

## Step 3: Create Prompt Template (if requested)

If the user wants a custom prompt, create `.megacli/prompts/<id>.md.tpl`.

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

1. Read the existing `megacli.json` (or `.megacli.json`) file.
2. Merge the new agent config into the `agents` block.
3. Write the updated config file.
4. If a prompt template was created, confirm the file path.

## Step 5: Verify

Tell the user:
- How to switch to the agent: `/` menu → Switch Agent → select `<name>`
- Or via CLI: `megacli --agent <id>`
- If it's a subagent: it will be available in the agent tool's target list
  automatically.

## Reference

Full documentation: `docs/agents.md`

### Available Built-in Tools

`bash`, `edit`, `multiedit`, `write`, `view`, `glob`, `grep`, `ls`,
`sourcegraph`, `fetch`, `agent`, `diagnostics`, `references`, `lsp_restart`,
`todos`
