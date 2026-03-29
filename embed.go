package dockpipe

import "embed"

// BundledFS holds embedded src/core/ (category dirs + workflows/ for bundled examples), assets/entrypoint.sh, VERSION,
// workflows/*, packages/* (first-party maintainer packages), .staging/packages/* (optional local experiments).
// On unpack, src/core/<category>/* → shipyard/core/<category>/*; src/core/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// packages/** and .staging/packages/** → shipyard/workflows/* (nested workflows, resolvers, domain script trees with config.yml).
//
//go:embed src/core assets/entrypoint.sh VERSION workflows packages .staging/packages
var BundledFS embed.FS
