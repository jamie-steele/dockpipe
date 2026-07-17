package dockpipe

import "embed"

// BundledFS holds embedded src/core/ (category dirs + workflows/ for bundled examples), assets/entrypoint.sh, VERSION,
// workflows/*, and packages/* (first-party maintainer packages).
// On unpack, src/core/<category>/* → bundle/core/<category>/*; src/core/workflows/<wf>/* → bundle/workflows/<wf>/*;
// packages/** → bundle/workflows/* (nested workflows, resolvers, domain script trees with config.yml).
//
//go:embed src/core assets/entrypoint.sh VERSION workflows packages
var BundledFS embed.FS
