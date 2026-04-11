package infrastructure

// embeddedPackageRootsPrefixes are top-level directory names inside go:embed (see embed.go): maintainer
// workflow trees (see embed comment), then optional .staging experiments. Order matters only for flat
// name/config.yml probes; nested mapping walks all roots.
var embeddedPackageRootsPrefixes = []string{"packages", ".staging/packages"}
