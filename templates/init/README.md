# Init template (normal template layout)

This folder is a **normal template**: `config.yml` + `resolvers/` (README only in repo). The init code in `bin/dockpipe` creates the user's workspace (scripts/, images/, templates/) and generates a top-level README at their destination. When they run `dockpipe init <name>`, it copies this template to their `templates/<name>/` and pulls example run/act scripts and the example Docker image from the repo root (`scripts/example-run.sh`, `scripts/example-act.sh`, `images/example/`), plus resolvers from `templates/llm-worktree/resolvers/`.
