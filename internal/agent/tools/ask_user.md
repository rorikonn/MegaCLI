Ask the user one or more questions interactively.

ALWAYS use this tool when you need to ask the user anything — never ask
questions in plain prose. This includes yes/no questions, multiple-choice
questions, and open-ended questions.

Guidelines:
- Batch multiple related questions into a single call whenever possible.
- Each question may have 0-10 options. Provide options when there are likely
  answers to choose from. Include a catch-all like "Other" if the user might
  have an unlisted answer.
- For open-ended questions (e.g. "What name would you like?"), omit options
  entirely — the user will type a free-form answer.
- Options should be concise (a few words each).

The user sees questions one at a time with a progress indicator. They can
select an option via shortcut or type a custom answer. All answers are
returned together once the sequence is complete.
