# Minimal project for chained-non-ai example.
# Run: dockpipe --workdir "$(pwd)" -- make lint|test|build

SHELL := /bin/bash
SRC := src/script.sh
BUILD_DIR := build
OUT := $(BUILD_DIR)/out.txt

.PHONY: lint test build

lint:
	@echo "[lint] Checking $(SRC)..."
	@test -f "$(SRC)" || (echo "Missing $(SRC)"; exit 1)
	@if command -v shellcheck >/dev/null 2>&1; then shellcheck "$(SRC)" || true; else echo "[lint] shellcheck not installed, skip"; fi
	@echo "[lint] OK"

test:
	@echo "[test] Running checks..."
	@test -f "$(SRC)" || (echo "Missing $(SRC)"; exit 1)
	@# After build, artifact should exist
	@if [ -f "$(OUT)" ]; then echo "[test] Build artifact present"; fi
	@echo "[test] OK"

build:
	@echo "[build] Building..."
	@mkdir -p "$(BUILD_DIR)"
	@echo "Built at $$(date -Iseconds)" > "$(OUT)"
	@echo "[build] Wrote $(OUT)"
