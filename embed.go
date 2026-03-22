package dockpipe

import "embed"

// BundledFS holds embedded templates/ (user-facing workflows + templates/core), lib/entrypoint.sh, VERSION,
// and dockpipe-experimental/workflows/* (repo-local experimental CI/dogfood workflows — not under templates/).
// On unpack, templates/core/* → dockpipe-experimental/core/*, templates/<wf>/* → dockpipe-experimental/workflows/<wf>/* (see bundled_extract.go);
// paths already under dockpipe-experimental/workflows/ are copied as-is.
//
//go:embed templates lib/entrypoint.sh VERSION dockpipe-experimental/workflows
var BundledFS embed.FS
