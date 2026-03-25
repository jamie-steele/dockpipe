package dockpipe

import "embed"

// BundledFS holds embedded src/templates/ (user-facing workflows + core/), assets/entrypoint.sh, VERSION,
// and workflows/* (first-party repo CI) plus .staging/workflows/* (maintainer / packaging / experiments — merged into the same materialized workflows tree).
// On unpack, src/templates/core/* → shipyard/core/*, src/templates/<wf>/* → shipyard/workflows/<wf>/* (see bundled_extract.go);
// embedded workflows/* and .staging/workflows/* → shipyard/workflows/* on disk (cache layout name unchanged).
//
//go:embed src/templates assets/entrypoint.sh VERSION workflows .staging/workflows
var BundledFS embed.FS
