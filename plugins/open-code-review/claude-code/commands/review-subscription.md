---
description: Review code changes with OCR on your Claude subscription (Claude Code CLI as the LLM backend) and apply vetted fixes.
---

Review the current code changes with OpenCodeReview, using the local Claude
Code CLI as its LLM backend so no API key is needed.

1. Dispatch the `ocr-reviewer` subagent with the scope from the user's
   arguments: none means workspace changes; pass `-c <sha>` or
   `--from <ref> --to <ref>` through unchanged. Include `--background` when the
   user described the requirements.
2. When it returns, show the verified findings.
3. Fix High findings that have a clear, local fix. List Medium findings with
   the proposed fix and apply them only if they are safe and well-defined.
   Re-run the review only if the user asks.
