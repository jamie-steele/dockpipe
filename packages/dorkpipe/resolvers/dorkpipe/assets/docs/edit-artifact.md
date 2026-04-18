# DorkPipe Edit Artifact

Versioned structured output for the Pipeon edit lane.

## Artifact

```json
{
  "artifact_version": "v2",
  "summary": "Short user-facing summary of the intended edit.",
  "target_files": ["path/to/file.ext"],
  "patch": "unified diff patch text",
  "structured_edits": [
    {
      "id": "replace_range-src-index-ts-1",
      "op": "replace_range",
      "source": "patch-derived",
      "language": "typescript",
      "target_file": "src/index.ts",
      "description": "Update function `renderSettings` in src/index.ts.",
      "target": {
        "kind": "function",
        "symbol_name": "renderSettings",
        "symbol_kind": "function"
      },
      "range": {
        "start_line": 18,
        "old_line_count": 7,
        "new_line_count": 9
      },
      "old_text": "old text block\\n",
      "new_text": "new text block\\n",
      "preconditions": ["The expected pre-edit text is still present near the recorded line range."],
      "postconditions": ["The file matches the prepared post-edit text for this range."],
      "fallback_notes": ["Fall back to the prepared unified diff if direct structured apply is unsafe."]
    }
  ],
  "validations": ["optional short validation note"]
}
```

## Structured Edit Ops

- `replace_range`
  - Replaces a line-bounded range using recorded pre/post text.
  - May include JS/TS-specific target metadata such as `function`, `class`, or `import_block`.
- `create_file`
  - Creates a new repo-relative file with the recorded content.
- `delete_file`
  - Deletes an existing repo-relative file.

## Rules

- `artifact_version` is currently `v2`.
- `summary` is safe to show in the chat UI.
- `target_files` are repo-relative paths only.
- `patch` remains required and must be valid unified diff text suitable for `git apply`.
- `structured_edits` is the preferred machine-readable apply plan when present.
- `validations` is advisory metadata only.

## Trace

Prepared edit runs also write `trace.jsonl` beside the artifact files. Each line is a JSON record:

```json
{
  "contract_version": "v2",
  "artifact_version": "v2",
  "request_id": "req_123",
  "parent_request_id": "req_parent",
  "phase": "edit",
  "event_type": "planning",
  "label": "Planning edit strategy",
  "status": "",
  "progress": 0.24,
  "metadata": {
    "candidate_count": 4
  },
  "artifact_dir": "bin/.dockpipe/packages/dorkpipe/edit/req_123",
  "timestamp": "2026-04-12T18:00:00Z"
}
```

The trace is the durable source for run inspection and time-travel UX. Keep it structured and bounded; do not put raw patch bodies into event metadata.

## Flow

1. Pipeon collects user request + workspace hints.
2. DorkPipe gathers candidate files and context.
3. DorkPipe builds a plan and realizes it into `structured_edits` plus `patch.diff`.
4. DorkPipe validates the artifact and patch applicability.
5. `apply-edit` prefers structured edits when safe, then falls back to the prepared patch.
