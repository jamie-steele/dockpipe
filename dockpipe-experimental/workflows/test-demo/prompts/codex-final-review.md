# Final review (Codex)

You are in **`/work`** inside DockPipe’s **Docker** runtime. **Deterministic prep** already ran: tests, vet, file list, bounded greps — summarized in **`.dockpipe/review-context.md`** on disk.

## Do this

1. **Read** `/work/.dockpipe/review-context.md` first (and only open other files under `/work` if you need to verify a specific finding — prefer paths hinted there or in `.dockpipe/review-files.txt`). If `/work/.dockpipe/local-model-notes.txt` exists and is short, you may treat it as an optional hint from a local prep pass.
2. Produce **only**:
   - **`## Findings`** — at most **12** bullets; each bullet names a **file or package** and a **concrete** issue or improvement.
   - Optionally **`## Notes`** — one short paragraph if something blocked inspection (say what, once).

## Do not

- Restate this prompt or the full prep bundle.
- Narrate your process (“I will now…”, repeated shell attempts).
- Run broad discovery (`find` / `ls -R` spam) — prep already bounded that work.
- Fabricate file-level detail you did not derive from `/work`.

## Trust

Workflow flags in the prep bundle and prior steps are ground truth for test/vet pass/fail.
