package dockpipe

import "embed"

// BundledFS holds templates, scripts, images, entrypoint, and VERSION for runtime materialization.
// The CLI does not require a separate on-disk install tree; assets unpack to the user cache on first use.
//
//go:embed templates scripts images lib/entrypoint.sh VERSION
var BundledFS embed.FS
