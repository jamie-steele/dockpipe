package dockpipe

import "embed"

// BundledFS holds embedded src/templates/ (user-facing workflows + core/), assets/entrypoint.sh, VERSION,
// and shipyard/workflows/* (repo-local maintainer CI/dogfood workflows — not under src/templates/).
// On unpack, src/templates/core/* → shipyard/core/*, src/templates/<wf>/* → shipyard/workflows/<wf>/* (see bundled_extract.go);
// paths already under shipyard/workflows/ are copied as-is.
//
//go:embed src/templates assets/entrypoint.sh VERSION shipyard/workflows
var BundledFS embed.FS
