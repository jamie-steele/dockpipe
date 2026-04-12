# DorkPipe Request Contract

First-pass request/stream contract for `dorkpipe request`.

## Request

Natural-language execution:

```bash
dorkpipe request --execute --workdir /repo --message "update the README intro"
```

Optional hints:

```bash
--active-file docs/README.md
--selection-text "selected text from the editor"
```

## Routes

- `chat`
- `inspect`
- `edit`

## Stream Events

Each line on stdout is JSON:

```json
{
  "contract_version": "v1",
  "request_id": "req_123",
  "type": "routed",
  "display_text": "Route: edit",
  "progress": 0.22,
  "metadata": {
    "route": "edit",
    "action": "",
    "arg": "",
    "reason": "edit-oriented request with code/workspace cues"
  }
}
```

## Event Types

- `received`
- `routed`
- `context_gathering`
- `model_stream`
- `validating`
- `ready_to_apply`
- `applying`
- `done`
- `error`

## Safe Metadata

Allowed metadata examples:

- `route`
- `action`
- `arg`
- `reason`
- `model`
- `context_path`
- `active_file`
- `workflow`
- `target`
- `files_touched`
- `validation_status`
- `artifact_dir`
- `patch_path`
- `ready_to_apply`

Not allowed in normal UI mode:

- raw patch bodies
- shell commands
- validator internals
- arbitrary execution arguments

## Edit Artifact Directory

Prepared edit requests write a directory under:

```text
bin/.dockpipe/packages/dorkpipe/edit/<request-id>/
```

Typical contents:

- `request.json`
- `prompt.md`
- `model-response.txt`
- `artifact.json`
- `patch.diff`
- `verify-patch.log`
- `apply.log`
- `post-apply-validation.log`

## Confirmation Flow

1. `dorkpipe request --execute` routes to `edit`
2. DorkPipe prepares and validates a patch
3. DorkPipe emits `ready_to_apply` with safe metadata only
4. Pipeon asks the user for confirmation
5. Pipeon calls:

```bash
dorkpipe apply-edit --workdir /repo --artifact-dir bin/.dockpipe/packages/dorkpipe/edit/<request-id>
```

6. DorkPipe applies and validates the prepared artifact
