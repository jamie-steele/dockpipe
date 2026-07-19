# TASK-016 Guided Game-Development Package And Unreal Integration

## Goal

Define a first-party DockPipe package that guides people from a small game idea toward a playable,
testable, and eventually releasable game while preserving human creative authority and DockPipe's
governed execution boundaries.

This task is a product and package backlog contract. It does not authorize implementation.

## Product Hypothesis

- DockPipe should act as a governed game-development guide, not a one-shot "generate my entire
  game" tool.
- The human owns the creative direction and final decisions. The system handles technical setup,
  project scaffolding, milestone planning, implementation agents, builds, verification, repair,
  playtest iteration, and release preparation within explicit boundaries.
- Start with one opinionated Unreal Engine path. Do not initially support every engine, workflow,
  or genre.
- Treat the package as a vertical application of existing DockPipe and DorkPipe capabilities, not
  as a reason to put game-specific behavior in DockPipe core.
- Keep package-specific behavior in the eventual package tree. Add or change a core primitive only
  after repository evidence demonstrates a generic capability gap.

## Target Users And Validation

Use two complementary validation users:

- An experienced developer dogfoods ambitious technical behavior through a seeded procedural
  tavern: first-person movement, opening doors, picking up a weapon, and swinging it.
- A consenting non-programmer creative tester validates whether the experience is understandable,
  enjoyable, recoverable after failures, and usable without engine or orchestration knowledge.

The developer path stresses technical composition. The creative-tester path is the primary proof
that the guide can make game development approachable without requiring an expert takeover.

## Golden Path

1. A non-programmer describes a tiny game idea in ordinary language.
2. The guide narrows it to one achievable playable interaction.
3. It creates or configures the Unreal project correctly.
4. It proposes one small milestone.
5. A governed coding agent implements the milestone.
6. The project builds and launches.
7. The user plays it and gives creative feedback.
8. The workflow completes one revision without requiring an expert to take over.

## Package And Core Boundary

The eventual first-party package should own its workflows, assets, templates, resolvers, skills,
policies, tests, Unreal-specific catalogs, and integration adapters. Follow the existing
[package-authoring boundary](../packages/package-authoring.md) and
[core/package model](../core/core-package-model.md).

`src/lib` and `src/cmd` must remain generic. A missing capability discovered while building the
package is not by itself permission for a core change; the task must first show that the primitive
is reusable outside game development and cannot be composed from existing package/runtime
contracts.

## Existing Capabilities To Reuse

Prefer composition over new infrastructure:

- package-owned workflows, assets, templates, resolvers, skills, policies, and tests
- DorkPipe planning, task graphs, verification, approval, and targeted follow-up reruns
- Ollama as a local or cheap attempt lane, without treating it as automatic authority
- governed Codex or Claude escalation lanes with explicit budgets
- scoped filesystem access plus separate apply and publish boundaries
- operation-result events, logs, artifacts, and run inspection
- skills rendering and durable project guidance
- PipeDeck's planned YAML-backed UI and approval surface from [TASK-008](agentic-app-ui.md)
- the shipped generic software-development workflow and task-pack contract from
  [TASK-007](closed/generic-software-dev-workflow.md), where applicable

Apply the [AI workflow value bar](../workflows/ai-workflow-value-bar.md): orchestration must beat one
strong direct worker on safety, cost, validation, rerunability, traceability, breadth, or user
experience. Do not add agents or handoffs that do not improve the result.

## Unreal Integration

- Unreal remains responsible for rendering, physics, animation, navigation, audio, assets, and the
  gameplay environment.
- Unreal's Procedural Content Generation framework and code-driven procedural techniques are
  first-class paths, not afterthoughts.
- Unreal MCP is a promising local bridge for governed Editor operations.
- A future custom Unreal Editor plugin may provide a thin dockable **Game Guide** panel.
- The Editor plugin must remain a presentation and context adapter. It must not become a second
  workflow system or own prompts, credentials, budgets, approvals, or durable task state.
- DockPipe YAML, package-owned catalogs, and run artifacts remain the durable source of truth.
- Keep Unreal MCP local-only by default. Do not expose an unauthenticated MCP endpoint remotely.

The first proof should use chat or CLI over the governed workflow and Unreal MCP bridge. A native
Editor surface is justified only after that workflow is useful and its durable contracts are clear.

## Proposed First Slices

1. Define the product contract, nontechnical UX principles, and package boundary.
2. Audit which existing DockPipe and DorkPipe capabilities can be composed unchanged.
3. Add an Unreal resolver and doctor that detect and validate the engine, compiler, project,
   plugins, and build tooling.
4. Add an opinionated Unreal starter and the first guided milestone workflow.
5. Prove a chat- or CLI-driven loop using Unreal MCP before building a custom Editor panel.
6. Add build, launch, log capture, failure classification, and targeted repair.
7. Add playtest-feedback capture and one-revision continuation.
8. Only after the workflow proves useful, add the thin native Unreal Editor surface.
9. Treat Steam preparation and delivery as a later approval-gated workflow.

Each slice should remain independently reviewable and preserve the package/core boundary.

## Safety And Approval Boundaries

- Keep the human in control of creative direction, milestone selection, asset choices, apply, and
  release decisions.
- Use scoped filesystem access and explicit approval for dependency installation, project mutation,
  plugin changes, apply, publish, network access, and other privileged operations.
- Keep prompts, credentials, budgets, approval policy, and durable task state out of the Unreal
  Editor adapter.
- Track asset licences and AI-content provenance from the beginning.
- Live-generated AI features such as LLM-driven NPCs require explicit content, safety, cost, and
  disclosure design before use.
- Follow the existing [safety guardrails](../runtime/safety-guardrails.md); secrets remain references
  and local model attempts do not gain automatic authority.

## Steam Release Boundary

A later workflow may prepare builds, SteamPipe configuration, private-branch uploads, store
checklists, and release-readiness artifacts. Human approval is mandatory for identity, banking,
taxes, agreements, fees, pricing, store claims, submissions, and final publication.

The package must not automatically accept licences or agreements or publish a public release. This
task is not legal, tax, licensing, or commercial advice.

## Success Criteria

Measure:

- time from initial idea to the first playable build
- number of technical interventions required from an expert
- whether the tester understands the current milestone and next decision
- whether build failures are explained and recoverable
- whether the same workflow completes one feedback and revision cycle
- whether artifacts clearly show what changed, why, verification status, cost, and required
  approvals
- whether orchestration demonstrably beats one direct worker on safety, rerunability, validation,
  traceability, or user experience

The golden-path proof succeeds only when both validation users can complete their intended path and
the non-programmer creative tester can complete the feedback/revision loop without an expert taking
over.

## Related Tasks

- [TASK-007 Generic Software Dev Workflow](closed/generic-software-dev-workflow.md) is a shipped
  foundation for governed planning, implementation, verification, approval, apply, and targeted
  follow-up work. Reuse it rather than duplicating its contract.
- [TASK-008 PipeDeck Agentic App UI](agentic-app-ui.md) owns the general YAML-backed UI, approvals,
  artifacts, logs, and run-inspection surface. The Unreal panel must stay a thin domain adapter.
- [TASK-009 Sandbox Toolchain Determinism](sandbox-toolchain-determinism.md) owns the broader
  resolver/preflight problem for reliable host tooling.
- [TASK-010 Declarative Dependency Install UX](declarative-dependency-install-ux.md) owns generic
  dependency declarations, install policy, and approval behavior.
- [TASK-001 Operation Results Contract Rollout](closed/operation-results-contract.md) shipped the
  event contract that new package operations should emit or reuse.

## Explicitly Out Of Scope

- supporting every game engine
- fully autonomous game generation
- automatic asset purchases
- automatic acceptance of licences or agreements
- public Steam publication
- remote exposure of Unreal MCP
- a large custom Unreal UI before the workflow is proven
- game-specific changes in `src/lib` or `src/cmd`
- implementing the package, Unreal plugin, workflows, resolvers, UI, or core changes in this
  backlog-only task

## Still Open

- Choose the eventual package name, package-tree location, and durable workflow identifiers. The
  descriptive **Guided game-development package** name is provisional.
- Define the smallest supported Unreal version, host platforms, compiler/toolchain matrix, and
  starter-project contract.
- Evaluate Unreal MCP's exact capability, authentication, lifecycle, and failure-recovery surface
  before adopting it as the local bridge.
- Decide the first playable interaction for the non-programmer golden-path proof while keeping the
  procedural tavern as the developer dogfood scenario.
- Define asset-source, licence, provenance, and AI-content metadata before any asset acquisition or
  generation workflow is implemented.
- Define the measurements and direct-worker baseline used to prove the workflow clears the value
  bar.
