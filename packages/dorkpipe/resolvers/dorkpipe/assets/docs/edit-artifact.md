# DorkPipe Edit Artifact

First-pass structured output for the Pipeon edit lane.

```json
{
  "summary": "Short user-facing summary of the intended edit.",
  "target_files": ["path/to/file.ext"],
  "patch": "unified diff patch text",
  "validations": ["optional short validation note"]
}
```

Rules:

- `summary` is safe to show in the chat UI.
- `target_files` are repo-relative paths only.
- `patch` must be valid unified diff text suitable for `git apply`.
- `validations` is advisory metadata only.

Current flow:

1. Pipeon collects user request + workspace hints.
2. DorkPipe gathers candidate files and context.
3. Ollama returns an edit artifact.
4. DorkPipe validates the artifact and patch applicability.
5. The patch may then be applied by a bounded primitive.
