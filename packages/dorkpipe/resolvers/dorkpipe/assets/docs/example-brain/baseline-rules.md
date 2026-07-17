# Baseline Rules

These are the deterministic rules to seed first when producing example brain guidance.

## Voice and framing

- Write from the consumer repo's point of view.
- Prefer repo-native names, paths, and terminology.
- Do not mention DockPipe, DorkPipe, mounts, `/work`, `/DesignNotes`, artifact roots, worker lanes,
  or orchestration internals unless the consumer repo itself treats them as product concepts.
- If an external corpus exists, describe it using the consumer repo's own naming and real local or
  organizational reference path, not the runtime mount label.

## Source precedence

- Prefer direct code and repo docs for current implementation claims.
- Prefer architecture notes, ADRs, planning docs, and explicit design references for intended
  direction.
- Preserve disagreements between current state and target state instead of blending them into one
  status claim.
- Narrow direct evidence beats broad summary prose when asserting that something exists now.

## Safe durable output

- Use exact path citations when a durable rule or claim depends on a file.
- Separate "what exists now" from "what is intended next".
- Call out open gaps explicitly when repo wiring, implementation, and design intent are not yet in
  parity.
- Source packets may retain stable guest display paths so evidence stays attributable during a run.
- Durable consumer docs must use repo-relative references. Rewrite a runtime or machine path only
  when one explicit source mapping proves the exact repo-relative target.
- Reject runtime labels, machine host paths, ambiguous mappings, and orchestration-only terminology
  rather than guessing or applying broad string replacement.

## What not to do

- Do not describe a runtime mount point as the source of truth.
- Do not let workflow metadata or orchestration scaffolding become product-architecture guidance for
  the consumer repo.
- Do not treat mockups, planning notes, or generated summaries as proof of implementation.
- Do not produce durable docs that read like an internal artifact handoff.
