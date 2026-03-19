# Future updates / ideas

Backlog and brainstorm. No commitment to implement in any order.

---

## Done / mostly there (kept for history)

These landed in spirit; see **`docs/cli-reference.md`**, **`docs/architecture.md`**, and **`docs/chaining.md`** for the current model.

- **Project / template YAML** — `templates/<name>/config.yml` with **`vars:`**, **`run` / `isolate` / `act`**, and **`--workflow <name>`**. Precedence: CLI > config > environment (and `.env` / `--env-file` where documented).
- **Named workflows** — Same as above; not a separate `dockpipe run fix` task table yet, but workflows are first-class via `--workflow`.
- **Docs** — CLI reference, WSL/Windows notes, chaining as doc-only patterns.

Still nice-to-have on top of that: optional **repo-root** `dockpipe.yml` (same shape as template config), **`dockpipe init` scaffolds config**, schema validation, sharing partial configs.

---

## GUI apps in container

Run any GUI app (IDE, editor, browser, etc.) inside a dockpipe container so the full experience is isolated. Same "run → isolate → act" story: work in the container, close when done, commit from host or via actions.

**Possible approaches:** X11 forwarding (`DISPLAY` + `/tmp/.X11-unix`), Wayland socket, or VNC/noVNC for a full desktop in the container. Mount the same worktree layout; when you're done, close the container and apply changes via existing workflows.

---

## Terraform as a step / action kind

**Goal:** Treat **Terraform** as a first-class kind of thing dockpipe can run in a workflow — not only arbitrary shell in a container.

**Sketch:** A step could be typed as `terraform` (or `kind: terraform`) with fields like working directory, `plan` / `apply`, var-files, backend config, and whether it runs on the **host** or in an **image** that ships the Terraform CLI. Same lifecycle hooks as today: optional pre/post scripts, act phase, workdir mount.

**Why:** IaC fits the same "isolated run + deterministic teardown / apply" story; teams often chain clone → plan/apply → notify without writing glue for every project.

**Open questions:** State handling (remote backend vs local), secret injection (env vs `-var`), approval gates for apply, and how this composes with **multi-step pipelines** below.

---

## Multi-step pipelines: step outputs → next-step variables

**Goal:** When we wire up **ordered steps** (each step = script on host and/or container image + command), **every step exposes `outputs`** that **feed the next step** by **setting or overriding variables** for that step.

**Model (intent):**

- Step *N* runs with a merged var set (shared `vars`, step *N* `vars`, CLI).
- Step *N* declares (or emits) **outputs** — e.g. named keys from a small manifest file, `stdout` parsing, or explicit `outputs:` in YAML after we define the format.
- Before step *N+1*, those outputs are applied as **environment / template variables** for step *N+1*, with clear precedence: e.g. **step *N* outputs** override inherited defaults but **CLI / explicit step *N+1* vars** can still override outputs where we want that.

**Mental model:** **input ⇒ output** along the chain — each step’s output namespace becomes part of the **input** context for the next step, so you don’t rely only on files in the workdir for structured handoff (though files remain valid for large artifacts).

**Compatibility:** Today’s single `run → isolate → act` invocation maps to a **one-step** pipeline with no cross-step outputs; chaining today is separate processes + workdir conventions (see **`docs/chaining.md`**).

---

## Host Actions (Built-in + User-Defined PowerShell via WSL2)

### Goal

Enable Dockpipe workflows running in WSL2 to invoke Windows host actions seamlessly using PowerShell — supporting both **first-class built-in actions** and **user-defined scripts**.

This removes manual steps and allows full end-to-end automation from a single workflow execution.

### Summary

The WSL2 runtime acts as the orchestration layer and can invoke Windows-side actions using:

```bash
powershell.exe -Command "<command>"
```

or

```bash
powershell.exe -File <script.ps1>
```

Two types of host actions are supported:

1. **Built-in actions (first-class, bundled)**
2. **User-defined actions (custom PowerShell)**

### Action Types

**1. Built-in Host Actions**

Dockpipe provides a set of predefined, tested actions that are: safe, consistent, cross-platform adaptable (future), zero-config for users.

Examples: `open-url`, `copy-text`, `open-path`, `launch-app`, `fetch-worktree`.

Implementation: PowerShell scripts bundled with Dockpipe, invoked from WSL2, e.g. `powershell.exe -File dockpipe/actions/open-url.ps1 "https://example.com"`.

**2. User-Defined Host Actions**

Users can define custom PowerShell scripts or inline commands to be executed from WSL2 (e.g. `powershell.exe -File custom-script.ps1 "arg1"` or `powershell.exe -Command "Start-Process notepad.exe"`). Use cases: custom integrations, enterprise workflows, project-specific tooling.

### Example Workflow

1. Run Dockpipe isolated task
2. Task completes
3. WSL2 runtime triggers host actions: `powershell.exe -File FetchFromWsl2.ps1 "$WORKTREE"`; `powershell.exe -Command "Start-Process code"`
4. User continues in Windows environment

### Path Handling

Convert WSL paths when passing to Windows: `wslpath -w /mnt/c/Users/you/project`.

### Design Principles

- Container never interacts with Windows directly
- WSL2 is the only layer invoking host actions
- Built-in actions preferred for common workflows; user-defined for flexibility
- Default behavior safe and predictable

### Safety Model

Built-in actions trusted and controlled; user-defined actions explicitly configured; no implicit or hidden host execution; all host actions visible in workflow definitions.

### Future Expansion

Template-level `host-actions` section; cross-platform abstraction (macOS/Linux equivalents); action registry / plugin system; optional allowlist or permission model for enterprise use.

### Key Insight

Workflows should not stop at isolation. They should: **Run → Isolate → Act → Integrate (Host)**. This feature completes the loop and removes the need for manual glue scripts.

Current WSL-oriented usage is documented in **`docs/wsl-windows.md`**.

---

*Add new ideas below.*
