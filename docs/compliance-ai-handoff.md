# Compliance & security posture — AI handoff

**Audience:** Humans and AI assistants when someone asks things like: *“Do we have compliance issues?”*, *“What’s our security posture?”*, *“Any governance gaps?”*

**Framework:** **DockPipe** is only execution (spawn → run → act). **DorkPipe** and **reasoning** sit on top. This document is **not** a new engine in `src/lib/dockpipe` — it is **contract + artifact map** so answers stay **grounded** and **honest**.

---

## What this is *not*

- **Not** SOC 2, ISO 27001, HIPAA, or any **certified** compliance verdict unless your organization has separate attestation.
- **Not** a substitute for legal review, penetration testing, or a formal risk assessment.

## What to do when asked

1. **Read** **`AGENTS.md`** (architecture, trust boundaries, what runs where).
2. **Load structured signals** if present (do **not** invent scan results):
   - **`.dockpipe/ci-analysis/findings.json`** — normalized **gosec** / **govulncheck** signals (see **`docs/dorkpipe-ci-signals.md`**).
   - **`.dockpipe/ci-analysis/SUMMARY.md`** — short counts / provenance.
   - **`.dorkpipe/self-analysis/`** — repo facts (signals, git, excerpts) from self-analysis.
   - **`.dorkpipe/run.json`** — orchestrator run metadata when relevant.
3. **State uncertainty:** If artifacts are **missing** or **stale** (vs `HEAD`), say so and recommend **`make ci`**, **`bash scripts/ci-local.sh`**, or downloading **CI artifacts** — do **not** claim “clean” without evidence.
4. **Classify** findings: severity, confidence, **file/workflow references** when grounded.
5. **Recommend** concrete next steps: fix issues, widen scans, document risk acceptance, pin tool versions — aligned with **`docs/dorkpipe-ci-signals.md`** and **`AGENTS.md`**.

## Quick local summary (maintainers)

From repo root after **`make build`**:

```bash
make compliance-handoff
# or: ./bin/dockpipe --workflow compliance-handoff --workdir . --
```

This runs **`scripts/dorkpipe/compliance-handoff.sh`** and prints artifact status + pointers.

## Bundled copy

Projects that run **`dockpipe init`** receive **`templates/core/bundles/dorkpipe/assets/docs/compliance-ai-handoff.md`** (same content as this file) with **`templates/core/`** — use it as the **user-shaped** copy of this contract.
