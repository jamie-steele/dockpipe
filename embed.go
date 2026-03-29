package dockpipe

import "embed"

// BundledFS holds embedded src/core/ (category dirs + workflows/ for bundled examples), assets/entrypoint.sh, VERSION,
// workflows/*, src/lib/dorkpipe/workflows/* (DorkPipe integration), .staging/packages/* (maintainer dockpipe packages: ide/agent/secrets resolvers, storage, bundles).
// On unpack, src/core/<category>/* → shipyard/core/<category>/*; src/core/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// src/lib/dorkpipe/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// .staging/packages/dockpipe/bundles/* → shipyard/core/bundles/*; other .staging/packages/** → shipyard/workflows/* (nested workflows and resolver profiles).
//
//go:embed src/core assets/entrypoint.sh VERSION workflows src/lib/dorkpipe/workflows .staging/packages
var BundledFS embed.FS
