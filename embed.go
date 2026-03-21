package dockpipe

import "embed"

// BundledFS holds templates (including templates/core/assets: scripts, images, compose), entrypoint, VERSION.
// The CLI does not require a separate on-disk install tree; assets unpack to the user cache on first use.
//
//go:embed templates lib/entrypoint.sh VERSION
var BundledFS embed.FS
