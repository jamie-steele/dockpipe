# Validation Commands

Read before final reporting or when choosing checks.

## Core

```bash
go test ./src/lib/...
go test ./src/cmd
make build
make ci
```

Use escalated execution when sandboxed Go cache or build cache blocks validation.

## Workflow

```bash
./src/bin/dockpipe workflow validate <config.yml>
./src/bin/dockpipe --workflow <name> --
```

## Package

```bash
./src/bin/dockpipe package test --workdir . --only <package>
./src/bin/dockpipe package compile workflows --workdir . --from packages/<package> --force
./src/bin/dockpipe package compile resolvers --workdir . --from packages/<package> --force
```

## DorkPipe Skills

```bash
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target codex --dry-run
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target claude --output /tmp/dorkpipe-claude-skills --dry-run
```

## Search

```bash
rg -n "<old-name>|<old-script>" packages docs src workflows .github -S
git status --short
git diff --stat
```

## Report

Always say which checks ran, which failed, and which were skipped.

## Dogfooding

This repo runs the same released binary and declarative workflows users get. Dogfooding is a quality
bar, not a special CLI primitive. After `make build`, run dogfood workflows from the repo root with:

```bash
./src/bin/dockpipe --workflow <name> --
```

Omit `--workdir .` when the current directory is already the project root.
