# Future updates / ideas

Backlog and brainstorm. No commitment to implement in any order.

---

## Template init / clone (like actions)

**Easier win, high value.** A way for people to clone workflow examples (e.g. codex-worktree, claude-worktree) into their project and customize, same idea as `action init --from` but for full workflows.

**Already done:** `dockpipe action init [--from <bundled>] <filename>` — creates a new action script from scratch or by copying a bundled action (commit-worktree, export-patch, print-summary).

**Idea:** `dockpipe template init my-workflow --from codex-worktree` (or similar) → copy `examples/codex-worktree/` into the user's project so they can edit the clone/repo/branch/commit flow. Gives a runnable script + README in a folder. Makes it easy to fork the official worktree examples or ship org-specific templates without changing dockpipe core.

---

## GUI apps in container

Run any GUI app (IDE, editor, browser, etc.) inside a dockpipe container so the full experience is isolated. Same "run → isolate → act" story: work in the container, close when done, commit from host or via actions.

**Possible approaches:** X11 forwarding (`DISPLAY` + `/tmp/.X11-unix`), Wayland socket, or VNC/noVNC for a full desktop in the container. Mount the same worktree layout; when you're done, close the container and apply changes via existing workflows.

---

*Add new ideas below.*
