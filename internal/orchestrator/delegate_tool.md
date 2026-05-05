Delegate a sub-task to another agent managed by the same MegaCli instance. The target agent runs the task independently and returns the result.

<usage>
- Use when a task is better handled by a specialized agent (e.g. code review, architecture analysis)
- The target agent has its own tools, model, and system prompt
- Delegation is synchronous — this tool blocks until the target agent completes
- Cannot delegate to yourself
</usage>

<parameters>
- target: Name of the agent to delegate to (required)
- task: Clear description of what the target agent should do (required)
- context: Optional extra context (code snippets, file paths, constraints)
</parameters>

<tips>
- Keep the task description self-contained — the target agent doesn't share your conversation history
- Use the agent names shown in the dashboard (e.g. "reviewer", "architect")
- For long-running tasks consider whether the user should be informed first
</tips>
