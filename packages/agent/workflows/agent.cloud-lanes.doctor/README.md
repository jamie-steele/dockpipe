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

Set `DORKPIPE_AGENT_DOCTOR_LIVE=false` to skip live model calls and only validate container/auth/skills plumbing.
