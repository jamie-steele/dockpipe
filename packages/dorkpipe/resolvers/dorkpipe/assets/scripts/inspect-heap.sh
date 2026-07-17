#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:?repo root required}"
TARGET="${2:-}"

cd "$ROOT"

if [[ -n "$TARGET" && "$TARGET" =~ ^[0-9]+$ ]]; then
  echo "Process memory snapshot for PID $TARGET"
  echo
  ps -o pid,ppid,rss,%mem,etime,command -p "$TARGET" 2>/dev/null || {
    echo "Could not inspect PID $TARGET"
    exit 1
  }
  echo
  if command -v pmap >/dev/null 2>&1; then
    echo "pmap summary:"
    pmap -x "$TARGET" 2>/dev/null | tail -n 5 || true
  fi
  exit 0
fi

if [[ -n "$TARGET" && -f "$TARGET" ]]; then
  echo "Heap/profile artifact: $TARGET"
  echo
  ls -lh "$TARGET"
  echo
  case "$TARGET" in
    *.pprof|*heap.out|*heap.pprof)
      if command -v go >/dev/null 2>&1; then
        echo "go tool pprof -top:"
        go tool pprof -top "$TARGET" 2>/dev/null | sed -n '1,40p' || true
      else
        echo "Go toolchain not available to summarize this pprof file."
      fi
      ;;
    *.heapsnapshot)
      echo "Heap snapshot detected. File-level metadata:"
      head -c 512 "$TARGET" 2>/dev/null || true
      echo
      ;;
    *)
      echo "Unrecognized heap/profile file type; showing basic file stats only."
      ;;
  esac
  exit 0
fi

echo "No PID or heap profile path provided."
echo
echo "Largest current-user processes by RSS:"
ps -u "$(id -u)" -o pid,ppid,rss,%mem,command --sort=-rss 2>/dev/null | sed -n '1,15p' || true
echo
echo "Tips:"
echo "- /heap 12345"
echo "- /heap path/to/heap.pprof"
echo "- /heap path/to/Heap.heapsnapshot"
