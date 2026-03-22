package dockpipe

import "embed"

// BundledFS holds embedded templates/ (user-facing workflows + templates/core), lib/entrypoint.sh, VERSION,
// and dockpipe/workflows/* (this repo’s internal CI/demo workflows — not under templates/).
// On unpack, templates/core/* → dockpipe/core/*, templates/<wf>/* → dockpipe/workflows/<wf>/* (see bundled_extract.go);
// paths already under dockpipe/workflows/ are copied as-is.
//
//go:embed templates lib/entrypoint.sh VERSION dockpipe/workflows
var BundledFS embed.FS
