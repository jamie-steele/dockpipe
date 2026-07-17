# High-Value Patterns

Example brain workflows should bias toward a few durable guidance shapes that keep later runs
smarter.

## Seed these first

- Source-of-truth rules: what wins for current implementation, intended direction, terminology, and
  conflict handling.
- Repo knowledge: bounded facts about the current implementation surface that are worth carrying
  forward.
- Design corpus usage: how to use external notes or planning material without treating them as
  implementation proof.
- Safe claim templates: short phrases that teach later workers how to express current-vs-target
  statements cleanly.

## Strong follow-ons

- Domain or subsystem indexes that point to the highest-value repo docs and source roots.
- Boundary rules that keep workflow, architecture, implementation, and backlog concerns from being
  collapsed together.
- Small "next build order" or "open gaps" pages when the repo clearly has a staged migration or
  known proof gaps.

## Patterns to avoid

- One large blended summary that mixes current code, architecture intent, backlog, and workflow
  mechanics.
- Durable docs that only describe how the last AI run behaved.
- Consumer guidance that depends on readers understanding the orchestration substrate.

## Practical heuristic

If a generated page will still be useful after the current run artifacts disappear, it is likely a
good candidate for seeding. If it mainly explains how the workflow executed, keep it in package or
artifact docs instead.
