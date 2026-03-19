# Contributing

Primitive first: run in container, optionally act after. Keep the core minimal.

**Issues** for bugs/ideas (check [future-updates.md](docs/future-updates.md)). **PRs** for code/docs; see [AGENTS.md](AGENTS.md). **Tests:** `bash tests/run_tests.sh` (unit); integration: `bash tests/integration-tests/run.sh`.

**Go:** layout is `lib/dockpipe/{domain,application,infrastructure}` — see [lib/dockpipe/README.md](lib/dockpipe/README.md). Run `go test ./...` and `gofmt` before PRs.

**Workflow YAML (user contract):** when changing step/async/merge behavior, update **[docs/workflow-yaml.md](docs/workflow-yaml.md)** and keep [lib/dockpipe/README.md](lib/dockpipe/README.md) in sync for contributor-oriented detail.

**Resolver:** add a file under `templates/<template>/resolvers/<name>`. New named template → `images/<name>/Dockerfile` + a branch in `lib/dockpipe/infrastructure/template.go` (`TemplateBuild`). **Scripts:** add run/act scripts in `scripts/`; workflow configs use `run:` and `act:`.

**Template:** add `templates/<name>/` with config.yml (run, isolate, act pointing to scripts/), resolvers/, isolate/. No run script in the template; config points to the repo scripts folder. See `templates/llm-worktree/`.

**Action:** add `scripts/<name>.sh`; use `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`. Add to `action init --from` list in `bin/dockpipe` if copyable.
