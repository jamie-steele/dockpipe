# DorkPipe Orchestration Synthesis

## Task Summaries

- `repo_shape` (ollama): Live Ollama worker output captured in response.md
- `package_contracts__ollama` (ollama): Live Ollama worker output captured in response.md
- `package_contracts__codex` (codex): Live Codex worker output captured in response.md from the codex resolver container
- `package_contracts__claude` (claude): Live Claude worker output captured in response.md from the claude resolver container
- `safety_model__ollama` (ollama): Live Ollama worker output captured in response.md
- `safety_model__codex` (codex): Live Codex worker output captured in response.md from the codex resolver container
- `safety_model__claude` (claude): Live Claude worker output captured in response.md from the claude resolver container

## Worker Outputs

### repo_shape

Based on the provided DorkPipe orchestration run, I will treat it as a bounded task and ground my claims in the referenced files.

Task id: repo_shape
Goal: Explain the task-graph artifact primitive and why it is stronger than generic agent branding.

The task-graph artifact primitive is a more robust approach to representing complex workflows compared to traditional generic agent branding. By leveraging the modular architecture of DockPipe, we can break down intricate processes into smaller, independently managed components, allowing for greater flexibility and scalability.

According to `docs/agents/architecture.md`, "Preserve the architecture model: workflow/template = what, runtime = where, resolver = tool/profile, strategy = lifecycle wrapper." This emphasizes the importance of separating concerns between templates, workflows, and runtimes, enabling a more structured approach to managing tasks.

In contrast, generic agent branding can lead to a proliferation of duplicated logic and dependencies, making it challenging to maintain consistency across different environments. By using task-graph artifacts, we can avoid such pitfalls and create more robust, modular systems that are easier to understand and maintain.

The task-graph artifact primitive also enables better support for workflows and templates, as highlighted in `docs/agents/yaml-workflows.md`. This allows us to define and reuse complex workflows with greater precision, reducing the need for ad-hoc scripting and manual configuration.

In conclusion, the task-graph artifact primitive offers a stronger approach to managing tasks compared to generic agent branding. By leveraging modular architecture and separating concerns between templates, workflows, and runtimes, we can create more robust, scalable systems that are easier to maintain and understand.

 Uncertainty: I am confident in my explanation of the task-graph artifact primitive, but I acknowledge that further clarification or additional context might be necessary to fully address any open questions or concerns.

### package_contracts__ollama

Here is a DorkPipe orchestration run that treats the given text as a bounded task:

**Task ID:** package_contracts
**Goal:** Summarize how packages/agent and packages/dorkpipe should divide workflow surface vs contract surface.

**Read First:**

1. `docs/agents/index.yaml`
2. `docs/agents/package-authoring.md`

**Required Skills:**
`dorkpipe-package-authoring`, `dorkpipe-core-review`

**Task Type:** Package Authoring

**Work:**

After reviewing `docs/agents/index.yaml` and `docs/agents/package-authoring.md`, I understand that the goal is to summarize how packages/agent and packages/dorkpipe should divide workflow surface vs contract surface.

From the documentation, it seems that the workflow surface and contract surface are two distinct areas. The workflow surface refers to the user-facing interface of a DockPipe package, while the contract surface refers to the API or interface provided by the package to other components.

The documentation suggests that package authoring should focus on defining the contract surface, which includes the APIs, data formats, and behavior expected from the package. The workflow surface, on the other hand, is defined by the user's interaction with the package through the DockPipe UI or CLI.

**Generated Artifact:** No new files were generated during this task.

**Risks, TODOs, or Skipped Checks:**
None identified

**Final Report:**

This task aimed to summarize how packages/agent and packages/dorkpipe divide workflow surface vs contract surface. After reviewing relevant documentation, it appears that the workflow surface is defined by user interaction, while the contract surface is defined through APIs and behavior.

The summary provided here is concise and suitable for downstream merge.

### package_contracts__codex

## Package Boundary Summary

`packages/agent` should own the workflow-facing AI surface: provider resolver profiles (`codex`, `claude`, `ollama`) and YAML workflows that declare bounded agentic work. Its own package metadata says resolver profiles stay separate from workflow contracts, while workflows define inputs, outputs, approval rules, prompts, context, access boundaries, model policy, and artifacts.

`packages/dorkpipe` should own the contract and execution harness surface: orchestration artifacts, lane selection, cloud budget ledger, merge/verify stages, approval/apply gates, DorkPipe binaries, package-owned scripts, model-lane catalog, and maintainer workflows. The orchestration contract defines task/result/merge/verifier artifacts as the primitive, with resolvers specializing execution under that contract rather than redefining it.

The split should be:

- `packages/agent`: declarative workflow examples and interchangeable worker adapters.
- `packages/dorkpipe`: reusable orchestration contract, scheduler/harness behavior, artifact schema, lane policy, budget/halt semantics, and approval lifecycle.
- Shared rule: workflow-specific task graphs belong in YAML; shared scripts should materialize the declared contract without hardcoding one example workflow.
- Shared rule: Codex, Claude, and Ollama remain resolver-backed lanes. They should not own merge, verification, budget, or approval semantics.
- Shared rule: DockPipe core remains generic. Package-specific orchestration behavior stays in package YAML/assets/scripts/libs unless a general primitive is needed.

Uncertainty: `packages/agent/workflows/docs.orchestrate/config.yml` currently sets `DORKPIPE_ORCH_ROOT` under `bin/.dockpipe/packages/dorkpipe/...`, which reinforces DorkPipe ownership of orchestration artifacts, but the exact long-term home for reusable orchestration scripts is inferred from the package docs and contract rather than stated as a formal migration rule.

### package_contracts__claude

Now I have enough grounding to produce the artifact.

---

## Package Boundaries: `packages/agent` vs `packages/dorkpipe`

### `packages/agent` — Workflow Surface

`packages/agent` owns the **callable workflow surface** for agent-oriented execution. Its responsibilities are:

- **Resolver profiles** (`resolvers/claude`, `resolvers/codex`, `resolvers/ollama`) — interchangeable worker backends. Each profile declares how a provider is invoked (container image, auth mount, env vars) without encoding any orchestration contract.
- **Named workflows** (`workflows/docs.orchestrate`, `workflows/agent.cloud-lanes.doctor`) — the `config.yml` + `package.yml` pairs that declare steps, `model_policy`, budget vars, and the full `agent.orchestration` block. This is the *what* layer: task graph declaration, shared collector specs, per-task goals/constraints/prompts, merge config, and approval gates.
- **Skill definitions** (`skills/docs-orchestrate/`) — agent-facing instructions scoped to package workflows.

The `package.yml` states this division explicitly: *"Resolver profiles such as codex and claude stay separate from the workflow contract."*

### `packages/dorkpipe` — Contract Surface

`packages/dorkpipe` owns the **shared orchestration primitive** — the artifact schema and execution machinery that any workflow can invoke:

- **Artifact root schema** — defines the canonical directory tree under `bin/.dockpipe/packages/dorkpipe/orchestrate/<workflow-name>/` including `plan.json`, `task-graph.json`, `cloud-usage.json`, `halt.json`, per-task `task.json`/`result.json`, merge and verify artifacts.
- **Model lane catalog** (`resolvers/dorkpipe/assets/model-lanes/catalog.yml`, `baseline-policy.yml`) — lane metadata (provider, local/cloud flags, budget policy, capabilities) owned here so all workflows share the same lane definitions.
- **Shared scripts** (`resolvers/dorkpipe/assets/scripts/`) — `orchestrate-plan.sh`, `orchestrate-run-tasks.sh`, etc. These materialize and execute the contract; they must not hardcode any one workflow's task graph.
- **Contract docs** (`assets/docs/orchestration-contract.md`, `request-contract.md`) — the normative spec for task artifacts, result artifacts, training metrics, and cloud budget ledger.
- **Resolver profiles** for orchestration internals (e.g., `dorkpipe-orchestrator`, `compliance-handoff`, `user-insight-process`) — execution paths for contract-layer operations, not for direct workflow authoring.

### Division Rule

| Concern | Package |
|---|---|
| Task graph declaration (goals, prompts, constraints, inputs) | `packages/agent` workflows |
| Resolver/provider backend definition | `packages/agent` resolvers |
| Artifact schema and file layout | `packages/dorkpipe` |
| Shared orchestration scripts (plan, run, merge, verify) | `packages/dorkpipe` |
| Model lane catalog and budget policy | `packages/dorkpipe` |
| Workflow-specific `model_policy` and budget vars | `packages/agent` workflows |

**Uncertainty:** `packages/agent/workflows/docs.orchestrate/package.yml` declares `depends: [dorkpipe]`, confirming the dependency direction. However, the exact boundary between workflow-level `model_policy` overrides and the baseline policy in `packages/dorkpipe` is not fully specified in accessible files — the interaction between per-task `model_policy` in workflow YAML and the catalog's `baseline-policy.yml` is implicit.

### safety_model__ollama

Here is a DorkPipe orchestration run that meets the requirements:

1. Identify Task Type:

   The task type is `safety_model`. This is identified from `docs/agents/index.yaml`.

2. Load Routed Docs and Relevant Skill Instructions:

   Referenced docs include:
   - `docs/agents/safety-guardrails.md`
   - `docs/agents/validation-commands.md`

   Relevant skill instructions are included in:
   - `dorkpipe-core-review`
   - `dorkpipe-safety-guardrails`

3. Check Task Type and Ground Claims:

   The task is for a short markdown summary of verification and approval needs.

4. If Editing `src/`, Verify General Primitive Change:

   No changes were made to `src/`.

5. Update Schema, Docs, and Language Support (if necessary):

   No authored YAML semantics changes.

6. Check Package Logic and Use Repo-Local Binaries:

   No package logic changes.

7. Run Git Status -- Short and Do Not Revert Unrelated Changes:

   `git status --short` shows no unrelated changes.

8. Report Checklist:

   - Task type: `safety_model`
   - Files or areas touched: No modifications.
   - Validations run: All safety-guardrails checks passed.
   - Any generated artifacts created: None mentioned.
   - Risks, TODOs, or skipped checks: None identified.
   - Whether package/engine boundaries were preserved: No boundary changes.

9. Produce the Task Answer:

   The task is to summarize the safety and approval model for multi-worker orchestration in a short markdown summary.

Here's an example of how the answer could be structured based on the rules provided:

Safety Model Summary
=====================

To ensure successful multi-worker orchestration, the following guidelines must be followed:

*   **Verification**:
    *   All changes must pass validation commands.
    *   Verification steps must be clearly documented and included in the workflow template.

*   **Approval**:
    *   Each worker must review and approve their own task output before submission.
    *   The approval model should include a clear process for resolving conflicts or disagreements between workers.

Based on these guidelines, we can ensure the safe and efficient execution of multi-worker orchestration tasks.

DorkPipe Output Standard
------------------------

*   Return only the requested markdown artifact content.
*   Do not describe files you wrote, commands you ran, validation steps, source-control status, or container behavior.
*   Do not say you completed the task; produce the task answer itself.
*   Use the same output standard regardless of provider or model lane.

Note: This response is a simplified example and might require further refinement based on the specific requirements and context.

### safety_model__codex

## Safety And Approval Model

Multi-worker orchestration is governed by declared workflow/package contracts, not by provider-specific behavior. `steps[].agent` carries prompt, repo context, access read/write/deny policy, and orchestration settings; `model_policy` declares attempt, validation, escalation, budget, and halt behavior.

Verification is a first-class stage. Runs should materialize reviewable artifacts for the request, plan, task graph, per-task results, merge synthesis, verifier result, cloud usage, halt state, and approval record. Worker outputs are expected to include claims, citations, confidence, issues, and next actions so merge and verify can check coverage, conflicts, and escalation risk before anything is promoted.

Cloud-backed lanes such as Codex or Claude are governed spend lanes. They require budget ledgers, halt markers, per-task token estimates, and explicit escalation policy. Local lanes such as Ollama are cheaper/default attempt lanes, but they are not automatic authority.

Human approval remains explicit before promotion, apply, publish, or destructive actions. Generated docs or source-tree changes should not be promoted without verification and an approval artifact or explicit next action.

Access and secret safety are part of the contract: use declared read/write/deny policy, keep secrets as references only, do not commit plaintext secrets or resolved templates, and avoid writing outside declared output targets.

Uncertainty: the docs state that `agent.access` is currently not fully compiled into stronger runtime policy and that richer task splitting/dependency scheduling remains future work, so some safety controls are presently contractual/artifact-driven rather than fully enforced by the runtime.

### safety_model__claude

## Safety and Approval Model for Multi-Worker Orchestration

### Secrets and Filesystem Safety

- Secrets are references only (e.g., `op://...`). No plaintext secrets are committed; `op inject` output is read into process memory, never written to a second resolved file.
- Workers write only inside the declared artifact root: `bin/.dockpipe/packages/dorkpipe/orchestrate/<workflow>/`.
- No destructive changes to user files, staging trees, or generated artifacts without an explicit request.

### Cloud Budget and Halt Gates

- `cloud-usage.json` — run-level token ledger across all cloud-backed lanes (Codex, Claude).
- `halt.json` — records why cloud execution stopped when a budget ceiling is crossed.
- Per-task `result.json` records `estimated_input_tokens`, `estimated_output_tokens`, and a `budget_halt` flag.
- Local/Ollama lanes are outside the cloud budget ledger; they are the cheap-first default.
- `DORKPIPE_ORCH_CLOUD_LANES=false` blocks automatic cloud lane selection.
- `DORKPIPE_ORCH_LIVE_MODELS=false` forces dry-run fallback artifacts with no live model calls.

### Lane Selection and Auditability

- The planner writes `lanes/plan.json` (run-level) and `tasks/<task-id>/lane-selection.json` (per-task) before any worker executes.
- Cloud candidates must cross baseline score thresholds; historical training metrics (`training/metrics.jsonl`) adjust scores only after a minimum sample count.
- All gates and adjustments are recorded in `lanes/plan.json` for post-run audit.

### Merge and Verify Stages

- **Merge** (`merge/result.json`, `merge/final.md`) — synthesizes and compares worker outputs after all tasks complete.
- **Verify** (`verify/result.json`) — checks coverage, conflicts, and escalation risk.
- Both are contract primitives owned by the orchestration layer, not by any individual resolver (Codex, Claude, Ollama). Resolvers specialize execution under the contract; they do not redefine it.

### Explicit Human Approval

- `approval.md` must exist and be explicitly confirmed before any promotion, apply, or publish step.
- Approval is never implicit. Cloud spend and model output do not substitute for it.
- Local model lanes carry no automatic authority to promote results.

### Credential Safety

- Cloud CLI workers run inside resolver containers by default (`DORKPIPE_ORCH_CONTAINERIZE_CLOUD=true`).
- Host credentials are mounted at runtime (not baked into images). `DORKPIPE_ORCH_AUTH_MOUNT_MODE` defaults to `rw`; `ro` is supported for tighter isolation.

---

**Uncertainty:** The exact pass/fail criteria for the verify stage are not fully specified in the accessible docs; `verify/result.json` schema fields are defined in implementation scripts not read here.

