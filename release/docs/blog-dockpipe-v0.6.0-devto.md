# DockPipe 0.6.0: the CLI that runs its own tests in Docker

```bash
dockpipe --workflow test --runtime docker
```

That’s a real multi-step workflow—alpine containers, outputs handed to the next step, no extra setup. In **v0.6.0** we treat this as the bar: **DockPipe dogfoods itself in CI**, so what you run locally is what we ship.

---

## What DockPipe does

**Run your command in a disposable container.** Your repo is at `/work`; when the process exits, the container is gone. Optional host scripts can run before and after—no bespoke “runner” stack.

**Workflows** (`--workflow`) add structure when you want them: steps, env handoff, named **runtimes** and **resolvers** when you care *where* and *which tool*—without turning the core into a heavy orchestrator.

---

## What’s new in 0.6.0

- **Stable story** — workflow = what happens; runtime = where; resolver = which tool; strategies and assets stay in their lanes.
- **Bundled layout you can reason about** — `shipyard/core/` and `shipyard/workflows/` when the binary unpacks; authoring trees still use `templates/` if you’re in the repo.
- **`dockpipe init`** — the obvious way to add DockPipe to a project.
- **Bundled examples** — including the `test` workflow above; the dockpipe project also ships extra workflows under `shipyard/workflows/` and runs them in CI (same binary you install — see **AGENTS.md**).
- **Windows in the test matrix** — same Go CLI, same tests, fewer surprises.

---

## Try it

1. Grab a **release binary** (or `make dev-install` from a clone): [github.com/jamie-steele/dockpipe/releases](https://github.com/jamie-steele/dockpipe/releases)
2. You need **Docker** and **bash** on the host.
3. Run:

```bash
dockpipe init
dockpipe --workflow test --runtime docker
```

**`dockpipe doctor`** checks your toolchain if something’s off.

No MSI in this release—install from `.deb`, tarball, zip, or the Windows script; details are in the repo’s **README** and **install** docs.

---

*Apache-2.0. Feedback and issues welcome on GitHub.*
