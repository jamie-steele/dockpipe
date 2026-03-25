package dockpipe

import "embed"

// BundledFS holds embedded src/core/ (category dirs + workflows/ for bundled examples), assets/entrypoint.sh, VERSION,
// workflows/*, .staging/workflows/*, .staging/resolvers/*, .staging/bundles/* (maintainer trees merged into materialized shipyard/core or workflows).
// On unpack, src/core/<category>/* → shipyard/core/<category>/*; src/core/workflows/<wf>/* → shipyard/workflows/<wf>/*;
// .staging/resolvers|bundles/* → shipyard/core/resolvers|bundles/*; embedded workflows/* and .staging/workflows/* → shipyard/workflows/*.
//
//go:embed src/core assets/entrypoint.sh VERSION workflows .staging/workflows .staging/resolvers .staging/bundles
var BundledFS embed.FS
