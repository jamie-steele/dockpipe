#!/usr/bin/env bash
# Tests for CLI argument parsing and template resolution.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI="${REPO_ROOT}/bin/dockpipe"

test_help() {
  "$CLI" --help | grep -q "dockpipe"
  "$CLI" -h | grep -q "dockpipe"
  echo "test_help OK"
}

test_no_dash() {
  local out
  out=$("$CLI" --image alpine 2>&1) || true
  if echo "$out" | grep -q "expected -- before command"; then
    echo "test_no_dash OK"
  else
    echo "test_no_dash FAIL: expected error when -- is missing (got: $out)"
    return 1
  fi
}

test_no_command() {
  local out
  out=$("$CLI" -- 2>&1) || true
  if echo "$out" | grep -q "no command"; then
    echo "test_no_command OK"
  else
    echo "test_no_command FAIL: expected error when no command after -- (got: $out)"
    return 1
  fi
}

test_unknown_template() {
  local out
  out=$("$CLI" --template no-such-template -- true 2>&1) || true
  if echo "$out" | grep -q "unknown template"; then
    echo "test_unknown_template OK"
  else
    echo "test_unknown_template FAIL: expected error for unknown template (got: $out)"
    return 1
  fi
}

test_template_resolution() {
  # Just ensure valid templates don't error out before docker (we might not have docker)
  # So we check that --template base-dev -- true triggers build or run; we can't easily
  # test without docker. Instead test that parsing works: --template base-dev -- true
  # should not say "unknown template"
  out=$("$CLI" --template base-dev -- true 2>&1) || true
  if echo "$out" | grep -q "unknown template"; then
    echo "test_template_resolution FAIL: base-dev should be valid"
    return 1
  fi
  echo "test_template_resolution OK"
}

test_action_path_resolution() {
  # With a missing action path we should still parse; docker may fail later
  out=$("$CLI" --action no/such/action -- true 2>&1) || true
  if echo "$out" | grep -q "unknown option"; then
    echo "test_action_path_resolution FAIL: --action should be accepted"
    return 1
  fi
  echo "test_action_path_resolution OK"
}

test_image_option() {
  # --image alpine -- true should not fail on parse (docker run may fail if no alpine)
  out=$("$CLI" --image alpine -- true 2>&1) || true
  if echo "$out" | grep -q "unknown option"; then
    echo "test_image_option FAIL"
    return 1
  fi
  echo "test_image_option OK"
}

run_tests() {
  test_help
  test_no_dash
  test_no_command
  test_unknown_template
  test_template_resolution
  test_action_path_resolution
  test_image_option
}

run_tests
