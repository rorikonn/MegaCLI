Ask the user one or more questions interactively.

ALWAYS use this tool when you need to ask the user anything — never ask
questions in plain prose output. This includes yes/no questions,
multiple-choice questions, and open-ended questions. If you find yourself
about to write a question mark in your response text, stop and use this
tool instead.

Guidelines:
- Batch multiple related questions into a single call whenever possible.
- Each question may have 0-10 options. Provide options when there are likely
  answers to choose from. Include a catch-all like "Other" if the user might
  have an unlisted answer.
- Options should be concise (a few words each).

Formatting rules for question content:
- CRITICAL: When a question has NO options (open-ended), the content MUST be
  a single clear sentence or short paragraph. Do NOT use numbered lists
  (1. 2. 3.), bullet points, or enumerated items in the content field —
  these look like selectable options in the UI and confuse users.
- If you have multiple distinct things to ask, split them into SEPARATE
  Question objects in the questions array. Each question = one focused topic.
- Bad: {"content": "I need some details:\n1. How many files?\n2. What names?\n3. Which directory?", "options": []}
- Good: Three separate questions — {"content": "How many files do you want to create?"}, {"content": "What should they be named?"}, {"content": "Which directory should they go in?"}

The user sees questions one at a time with a progress indicator. They can
select an option via shortcut or type a custom answer. All answers are
returned together once the sequence is complete.
