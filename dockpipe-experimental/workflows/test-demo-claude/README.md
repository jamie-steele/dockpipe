# test-demo-claude

Same pipeline shape as **`test-demo`**, but the final step uses **Claude Code** (`claude --dangerously-skip-permissions -p`) in the **claude** image. Prompt: **`../test-demo/prompts/claude-chain-review.md`**.

```bash
export ANTHROPIC_API_KEY="…"   # or CLAUDE_API_KEY — see templates/core/resolvers/claude/profile
dockpipe --workflow test-demo-claude --resolver claude --runtime docker --workdir /path/to/repo \
  --mount "$(go env GOPATH)/pkg:/go/pkg:rw" --
```
