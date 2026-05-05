Delegate a task to an agent on another running MegaCli instance. Use this for cross-project collaboration where work needs to happen in a different workspace.

<usage>
- Discovers other MegaCli instances running on the same machine
- Sends a task to a specific agent on the remote instance
- Blocks until the remote agent completes and returns the result
- The remote agent runs in its own workspace with its own tools and context
</usage>

<parameters>
- instance_pid: PID of the target MegaCli instance (required, use discover first)
- target_agent: Name of the agent on the remote instance (required)
- task: Clear task description (required)
- context: Optional extra context
</parameters>

<tips>
- First use megacli_info to see available remote instances and their agents
- The remote agent has no access to your local files — include all needed context in the task
- Consider timeout implications for long-running remote tasks
</tips>
