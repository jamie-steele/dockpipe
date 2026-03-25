# Dev container base: Cursor's remote server install script calls `base64 -D` (BSD decode flag).
# GNU coreutils `base64` only accepts `-d`, which breaks the installer in Linux containers.
# Shadow `base64` early on PATH and translate `-D` → `-d` before delegating to the real binary.
FROM mcr.microsoft.com/devcontainers/base:bookworm

RUN cat > /usr/local/bin/base64 <<'EOF'
#!/bin/sh
rb=/usr/bin/base64
[ -x "$rb" ] || rb=/bin/base64
set --
for arg
do
  case "$arg" in
    -D) set -- "$@" -d ;;
    *) set -- "$@" "$arg" ;;
  esac
done
exec "$rb" "$@"
EOF
RUN chmod +x /usr/local/bin/base64

ENV PATH=/usr/local/bin:${PATH}
