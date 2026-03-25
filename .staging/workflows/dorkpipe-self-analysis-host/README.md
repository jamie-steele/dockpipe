# dorkpipe-self-analysis-host

Same behavior as **`dorkpipe-self-analysis`**, but **`skip_container: true`** — runs **`scripts/dorkpipe/run-self-analysis.sh`** on the **host**.

Use when **Docker** is not available or you want the fastest path without building **`bin/dorkpipe`** inside a container.

Prefer **`dorkpipe-self-analysis`** for isolation aligned with DockPipe’s product model.
