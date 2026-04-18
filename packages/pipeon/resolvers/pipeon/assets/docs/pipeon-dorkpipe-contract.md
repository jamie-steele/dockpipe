# Pipeon ↔ DorkPipe Internal Contract

Versioned internal API contract between:

- **Pipeon** — chat client / user experience
- **DorkPipe** — routing, orchestration, validation
- **DockPipe** — isolated mutation boundary

## Guiding rule

Pipeon sends requests. DorkPipe decides, orchestrates, validates, and streams. DockPipe mutates the
filesystem. Model output is never executed directly.

## Boundary

### Pipeon

- normal conversational UX
- streams user-safe status
- may send advisory hints
- must not mutate the repo directly
- must not bypass DorkPipe routing
- may provide active-file, selection, and recent-turn hints, but route authority stays with DorkPipe

### DorkPipe

- server-authoritative route selection: `chat` / `inspect` / `edit`
- context gathering
- model invocation
- validation of model output into bounded artifacts
- optional confirmation gates
- hands only validated artifacts to DockPipe
- may return a routed non-mutating action for Pipeon to execute locally when that action is already a bounded primitive
- may stop edit flows at `ready_to_apply` so Pipeon can ask for confirmation before mutation

### DockPipe

- isolated execution
- workflow and script execution
- filesystem mutation
- validation/test execution under the chosen workflow

## Request (`v1`)

```json
{
  "contract_version": "v1",
  "request_id": "req_123",
  "session_id": "sess_abc",
  "workspace_root": "/workspace",
  "user_message": "Update the README intro",
  "attachments": [
    {
      "kind": "file",
      "path": "docs/architecture.md"
    }
  ],
  "client_capabilities": {
    "streaming": true,
    "rich_status": true,
    "debug": false
  },
  "client_hints": {
    "intent": "edit"
  },
  "cancel_previous": false
}
```

Rules:

- `workspace_root` is single-root only in `v1`
- `file` attachments are now wired for local workspace files in `v1`
- `image` and `pdf` attachment kinds are scaffolded but remain TODO
- hints are advisory only
- route authority stays with DorkPipe

CLI equivalent in the current local-first harness:

```bash
dorkpipe request --execute --workdir /workspace --message "fix the README intro"
```

## Stream events (`v1`)

```json
{
  "contract_version": "v1",
  "request_id": "req_123",
  "type": "context_gathering",
  "display_text": "Searching repository...",
  "progress": 0.4,
  "final_payload": {},
  "debug": {}
}
```

Event types:

- `received`
- `routed`
- `context_gathering`
- `model_stream`
- `validating`
- `ready_to_apply`
- `applying`
- `done`
- `error`

Rules:

- `display_text` must be user-safe
- `model_stream` is UI-only
- only validated artifacts may drive execution
- ordering must be consistent
- non-streaming backends may still emit synthetic progress and a buffered `model_stream`
- `ready_to_apply` must carry only safe metadata such as artifact location and touched-file counts, not raw patch bodies

## Final response

```json
{
  "contract_version": "v1",
  "request_id": "req_123",
  "status": "ok",
  "user_message": "Updated the README intro and validated the change.",
  "metadata": {
    "route": "edit",
    "files_touched": 1,
    "validation_status": "passed",
    "confirmation_used": true
  },
  "debug": {}
}
```

Default mode must not expose:

- patch bodies
- shell commands
- DAG internals
- validator traces

## Errors

```json
{
  "contract_version": "v1",
  "request_id": "req_123",
  "status": "error",
  "error": {
    "error_code": "MODEL_UNAVAILABLE",
    "user_message": "The local model is unavailable right now.",
    "retryable": true,
    "severity": "warning"
  }
}
```

Stable error codes:

- `INVALID_REQUEST`
- `UNSUPPORTED_CONTRACT_VERSION`
- `WORKSPACE_UNAVAILABLE`
- `CONTEXT_GATHER_FAILED`
- `MODEL_UNAVAILABLE`
- `MODEL_OUTPUT_INVALID`
- `VALIDATION_FAILED`
- `CONFIRMATION_REQUIRED`
- `APPLY_FAILED`
- `CANCELLED`
- `INTERNAL_ERROR`

## Mandatory denylist

The contract must never expose by default:

- raw shell commands
- arbitrary host execution
- DAG file paths
- node ids
- patch bodies
- validator internals
- direct mutation controls
- DockPipe execution arguments
- anything that allows Pipeon to bypass DorkPipe

## Debug mode

Debug mode changes visibility only, never privileges or execution behavior.

Allowed additions in debug mode:

- route reasoning summary
- safe artifact counts
- timing
- validation summary
- safe internal IDs

See also:

- **`packages/dorkpipe/resolvers/dorkpipe/assets/docs/request-contract.md`**
- **`packages/dorkpipe/resolvers/dorkpipe/assets/docs/edit-artifact.md`**

## Attachments

### `v1`

- `file` attachments resolve to local workspace paths
- text-like file contents become read-only context for DorkPipe
- binary-looking files are ignored safely
- image and pdf attachments are not executed and are not yet ingested

### Deferred

- `image` attachments
- `pdf` attachments
- converted-to-text enrichment before model use
- still treated as untrusted input
- must not bypass validation or directly drive execution

## Safety guarantees

- model output is data only
- streamed model text is UI only
- no execution of raw model text
- only validated artifacts cross to DockPipe
- all mutation happens outside DorkPipe
- Pipeon receives no direct mutation controls
