package dockpipe

import "embed"

// BundledFS holds embedded templates/ (source authoring layout), lib/entrypoint.sh, VERSION.
// On unpack, templates/core/* is written to dockpipe/core/* and templates/<wf>/* to dockpipe/workflows/<wf>/* (see bundled_extract.go).
// The CLI does not require a separate on-disk install tree; assets unpack to the user cache on first use.
//
//go:embed templates lib/entrypoint.sh VERSION
var BundledFS embed.FS
