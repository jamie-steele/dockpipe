package dockpipe

import "embed"

// BundledFS holds embedded templates/ (authoring layout), lib/entrypoint.sh, VERSION,
// and dockpipe/workflows/dogfood-* (Codex dogfood presets live only under dockpipe/workflows/ in-tree).
// On unpack, templates/core/* → dockpipe/core/*, templates/<wf>/* → dockpipe/workflows/<wf>/* (see bundled_extract.go).
//
//go:embed templates lib/entrypoint.sh VERSION dockpipe/workflows/dogfood-codex-pav dockpipe/workflows/dogfood-codex-security
var BundledFS embed.FS
