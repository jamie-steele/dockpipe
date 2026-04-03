# Pipeon IDE — experience design

**Pipeon** is the **gateway to intelligence**: a **local-first app** the user **installs and boots**. It is **not** a shell where you paste one-off prompts as the main UX. Inside the app, the user **talks to Ollama (Llama) on the host** in a normal chat surface, while Pipeon **aggregates and surfaces** the intelligence this stack already produces (repo analysis, CI signals, user insights, workflow metadata)—**on startup** and **in conversation**—without making the user learn a “platform.”

### What “gateway” means

| Layer | Role |
|--------|------|
| **Host / Ollama** | Default **Llama** inference stays on the **host** (fast, private, no cloud required for core use). |
| **Docker (backend)** | Used for **isolation** and for **DockPipe-class work** (containers, reproducible steps)—not as the thing the user manually drives for every chat turn. The app orchestrates it; the user does not live in `docker run` for daily chat. |
| **Safe host mapping** | The user **opens directories** (projects, worktrees) **in the app**; Pipeon maps them into the isolated side **safely** (bounded mounts, clear labels, no silent exfiltration)—**like a native “add folder” workflow**, not manual volume flags. |
| **Intelligence** | Everything we added—**`.dockpipe/`**, **`.dorkpipe/`**, insights queue, CI normalization, workflows—is **ingested** so the model and UI can **hint** what’s available and **ground** answers. |

### Boot and conversation (product UX)

- **On boot:** short, **non-intrusive hints**—what bundles are present, whether scans look stale, that user insights are “signals not truth,” optional “refresh analysis” as a **suggestion** (never a forced action).
- **While chatting:** when the user asks about security, compliance, architecture, or “what do we know,” Pipeon **names the lanes** it used (facts vs scans vs user guidance) in **one line**, same as below—without requiring the user to have run terminal commands first.

**Cloud models** may arrive later as optional backends; they must not change the **single conversational surface** or disclosure patterns.

### What this repo ships *today* vs the product

The **shell scripts** (`src/bin/pipeon`, bundle + one-shot chat) are a **developer harness** to exercise the **same context bundle and Ollama contract** before the branded app exists. They are **not** the Pipeon user experience. The **editor** is a **fork of VS Code (Code OSS)** with Pipeon layered on—see **`pipeon-vscode-fork.md`**, **`pipeon-architecture.md`**, and **`../vscode-extension/`** (sibling of **`assets/`**). Harness: **`../assets/scripts/README.md`**.

---

## 1. Core UX principles

| Principle | What it means |
|-----------|----------------|
| **Conversational first** | One primary interaction: natural language with the assistant. No required dashboards, node editors, or workflow builders for core use. |
| **Grounded, not clever** | Answers cite or implicitly rely on **discovered context** (files, artifacts, insights). Uncertainty is stated plainly. |
| **User-initiated actions** | The system **never** runs destructive or repo-wide operations without clear user intent (see [Control](#3-control)). |
| **System voice** | Clear, direct, contextual—not a mascot or “personality.” |
| **Effortless adoption** | Install → boot → add folders → talk. No terminal prompt passing as the default path. Settings stay minimal; power users can tune model/paths later. |

---

## 2. Awareness — what the system loads (without being asked)

At session start and when the user’s question is **repo-scoped**, the runtime should **attempt** to discover and attach (in order of relevance):

1. **Repository facts** — e.g. layout, recent changes, key paths (from self-analysis or lightweight indexing if present).
2. **CI / scan signals** — e.g. normalized findings under `.dockpipe/ci-analysis/` when available.
3. **Structured user guidance** — e.g. `.dockpipe/analysis/insights.json` (accepted/pending items, scoped by path or topic).
4. **Workflow / orchestration metadata** — e.g. `.dorkpipe/run.json`, metrics tails, when relevant to “why did X happen” or “what ran last.”
5. **Handoff text** — optional short blocks (e.g. paste prompts) only when they add signal density, not as a second UI.

**Rule:** Discovery is **best-effort**. If something is missing, the system says so briefly and continues—no blocking modal.

---

## 3. Control — what the system never does silently

| Allowed without extra confirmation | Requires explicit user intent |
|-----------------------------------|-------------------------------|
| Read files and artifacts the user’s question plausibly needs | Delete branches, reset hard, force-push |
| Summarize, compare, explain | Install packages system-wide, change git remotes |
| Suggest commands or scripts (as text) | Run `docker`, `rm -rf`, payment/secret tooling |
| Offer “refresh analysis” as a **recommendation** | Merge PRs, open cloud issues, send network calls to third parties (unless user asked) |

**Pattern:** *Recommend → user confirms → optional one-shot script or documented command* (e.g. “Run `make self-analysis` when you’re ready.”).

---

## 4. Feedback — minimal, honest, non-intrusive

### 4.1 When analysis is **in use**

Short prefix or footnote (pick one style per product skin; keep under two lines):

- *“Using: repo summary (`.dorkpipe/self-analysis`), CI findings (`.dockpipe/ci-analysis`, 12 items), 3 user insights (accepted).”*
- *“Context: local analysis bundle + 2 insight records; CI bundle not present.”*

### 4.2 When analysis is **stale**

Define **stale** in product config (e.g. commit SHA in artifact ≠ `HEAD`, or age &gt; N days). Wording:

- *“Note: CI findings are from commit `abc1234` (behind current `HEAD`). Refresh recommended if you changed security-sensitive code.”*
- *“Self-analysis looks older than your latest merge; re-run analysis for best accuracy.”*

**Do not** nag every turn—surface once per session or when the user asks security/compliance questions.

### 4.3 When **refresh** is recommended

- *“I can still answer from what’s on disk. To update scans: `bash src/scripts/ci-local.sh` (or your CI artifact download).”*
- *“No `findings.json` here—I’m reasoning from source only. Generate signals when you want scan-backed answers.”*

### 4.4 When **relevant findings** exist

- *“CI reports 2 high-severity items in `lib/...`—I’ll factor those in. Details: …”*
- *“You have a pending user insight tagged compliance—treating it as guidance, not proof.”*

---

## 5. Tone and response patterns

### 5.1 Voice

- **Use:** short sentences, concrete nouns (paths, artifact names, counts), explicit limits (“I don’t have X”).
- **Avoid:** hype, filler, fake empathy, anthropomorphism (“As an AI model…” unless legally required).

### 5.2 Response skeleton (default)

1. **Direct answer** to the question.
2. **Context used** (one line)—what was loaded or not.
3. **Caveats**—staleness, missing artifacts, user insights as signals only.
4. **Optional next step**—one suggestion, phrased as optional.

### 5.3 Example patterns (templates)

| Situation | Pattern |
|-----------|---------|
| Good coverage | “Answer: … **Context:** findings + insights + recent diff. **Caveat:** none material.” |
| Partial coverage | “Answer: … **Context:** source only; no CI bundle. **If you need scan-backed:** run …” |
| User insight involved | “I’m applying 2 **accepted** user insights as constraints; they are not verified facts.” |
| Refusal / defer | “I won’t run that without your go-ahead. Here’s the exact command if you want to run it locally.” |

---

## 6. Example conversational flows

### Flow A — “Are we okay on compliance?”

**User:** Are we okay compliance-wise?

**System:**  
I can’t certify compliance—no tool can from a repo alone. **From disk:** I have CI-normalized findings (0 issues) at commit `7d807b5`, and 1 pending user insight about audit logging (guidance only). **Gap:** no formal policy attestation.  
If you want this aligned with `HEAD`, refresh CI analysis after your latest changes.

### Flow B — “What should I work on next?”

**User:** What should I work on next?

**System:**  
From **signals**: 3 TODOs in `signals_todo.txt`, and CI is clean. **User insights** prioritize API error handling in `src/lib/application/`.  
**Suggestion:** tackle the TODOs in that package unless you’re mid-feature elsewhere—your call.

### Flow C — Stale CI

**User:** Any security issues?

**System:**  
**Findings on disk** are from an older commit (`abc1234` vs current `HEAD`). I’ll summarize those 4 items, but treat them as **possibly stale**.  
**Optional:** regenerate `.dockpipe/ci-analysis/` locally, then ask again for a scan-aligned answer.

### Flow D — No analysis

**User:** Summarize the architecture.

**System:**  
No bundled analysis present; I’m working from the tree and `AGENTS.md`-style docs. **Optional:** run your repo’s self-analysis workflow to get a fact bundle for faster future sessions.

### Flow E — User tries to delegate a dangerous act

**User:** Delete all containers and prune volumes.

**System:**  
I won’t run destructive Docker commands for you. If that’s what you want, run locally: `docker …` (paste). Tell me if you want a safer subset (e.g. stop named stack only).

---

## 7. Minimal UX guidelines (effortless adoption)

1. **One chat surface** as default; optional side panel for “context sources” later—never required on day one.
2. **Show context in one line** when non-obvious; hide raw paths behind “details” if the UI grows.
3. **Local model by default** in settings; cloud = advanced, same UX contract.
4. **No scoreboards** for “AI quality”—usefulness beats gamification.
5. **Errors:** model offline → “Start Ollama / pick another local model” in one sentence + link to docs—not a stack trace in the main thread.
6. **Privacy copy** once: data stays local unless the user opts into cloud.

---

## 8. Alignment with this repository

This repo already separates:

- **Facts** (e.g. `.dorkpipe/self-analysis/`)
- **Scans** (e.g. `.dockpipe/ci-analysis/findings.json`)
- **User guidance** (e.g. `.dockpipe/analysis/insights.json`)

Pipeon should **treat those as distinct lanes** in prompts and in user-facing disclosure, matching the contracts in **`../../docs/compliance-ai-handoff.md`**, **`../../docs/dorkpipe-ci-signals.md`**, and **`../../docs/user-insight-queue.md`**.

---

## 9. Experience goal (one line)

**Install Pipeon, boot it, add your project folders, and talk in the app—it pulls in the intelligence we built (Ollama on the host, Docker where isolation matters) and explains what it knows, without feeling like another platform to learn.**

---

## 10. Implementation in this repository

### Product (Pipeon app — target UX)

The **shipping** Pipeon experience is a **desktop (or equivalent) application** that implements the **gateway** model in the opening section above: boot hints, in-app chat to **host Ollama**, **Docker-backed** isolation for engine work, **safe host directory mapping**, and unified use of DockPipe/DorkPipe artifacts. That UI is **not** fully implemented in this repository yet; this repo defines the **contracts**, **artifact layout**, and a **thin harness** for developers.

### Repository today (intelligence + dev harness)

| Piece | Purpose |
|-------|---------|
| **Artifact lanes** | **`.dockpipe/`**, **`.dorkpipe/`**, insights, CI bundle—documented across **`docs/`** |
| **`../scripts/bundle-context.sh`** | Builds **`pipeon-context.md`** — same **aggregate** the app should load (harness + future UI) |
| **`src/bin/pipeon`** / **`chat.sh`** | **Dev-only:** one-shot Ollama call to validate prompts + bundle (**not** the user-facing UX) |
| **`../scripts/lib/enable.sh`** | Feature gate for harness (**`DOCKPIPE_PIPEON`**, min version **0.6.5**, **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE`**) |
| **`.vscode/tasks.json`** | Optional tasks for **maintainers** testing the harness |

**Release plan:** keep harness gates until **`VERSION` ≥ 0.6.5** as planned; the **app** may ship on its own cadence but should consume the **same** artifact contracts.

**Architecture (gateway, Ollama, Docker, mounts):** **`pipeon-architecture.md`**. Harness details: **`../scripts/README.md`**.
