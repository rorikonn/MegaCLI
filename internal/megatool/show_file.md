Display a file's content to the user in the terminal UI without consuming LLM context tokens. Use this when the user needs to see file content but the LLM does not need to analyze it.

<usage>
- Shows file content directly in the user's terminal display panel
- The LLM only receives confirmation that the file was displayed
- Supports offset and limit for large files
- Use `view` instead if the LLM needs to read and analyze the content
</usage>

<important>
- This tool is for USER visibility only — use `view` when the LLM needs the content
- Maximum file size: 512KB
</important>
