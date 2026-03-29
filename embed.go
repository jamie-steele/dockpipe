package dockpipe

import "embed"

// BundledFS holds embedded src/core/ (category dirs + workflows/ for bundled examples), assets/entrypoint.sh, VERSION,
// workflows/*, src/lib/dorkpipe/workflows/* (DorkPipe integration), .staging/packages/* (maintainer packages).
// On unpack, src/core/<category>/* → shipyard/core/<category>/*; src/core/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// src/lib/dorkpipe/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// .staging/packages/** → shipyard/workflows/* (nested workflows, resolvers, domain script trees with config.yml).
//
//go:embed src/core assets/entrypoint.sh VERSION workflows src/lib/dorkpipe/workflows .staging/packages
var BundledFS embed.FS
