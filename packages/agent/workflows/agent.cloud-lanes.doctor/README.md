# agent.cloud-lanes.doctor

Validates the Codex and Claude cloud-lane resolver containers before a larger DorkPipe orchestration run.

It checks:

- host auth directory discovery
- auth mount visibility inside `/home/node`
- skills directory visibility
- CLI availability and version output
- optional tiny live prompt response

Run:

```bash
dockpipe --package agent --workflow agent.cloud-lanes.doctor --
```

Run only one provider while debugging auth:

```bash
DORKPIPE_AGENT_DOCTOR_PROVIDERS=claude dockpipe --workflow agent.cloud-lanes.doctor --
```

The workflow runs from the repo root because it shells back into the DockPipe CLI with the
current checkout as `--workdir`. Its `scopes` bind source reads to the repo and generated
output to this workflow's artifact root.

Each provider check writes `stdout.txt`, `stderr.txt`, and `result.json` under
`providers/<name>/`.

Set `DORKPIPE_AGENT_DOCTOR_LIVE=false` to skip live model calls and only validate container/auth/skills plumbing.

If the live Claude check reports `Not logged in`, fix the host login first:

```bash
claude login
DORKPIPE_AGENT_DOCTOR_PROVIDERS=claude dockpipe --workflow agent.cloud-lanes.doctor --
```
