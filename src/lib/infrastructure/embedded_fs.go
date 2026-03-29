package infrastructure

// embeddedPackageRootsPrefixes are repo-root paths inside go:embed (see embed.go): first-party maintainer
// packages, then optional local experiments under .staging/packages. Order matters only for flat
// name/config.yml probes; nested mapping walks all roots.
var embeddedPackageRootsPrefixes = []string{"packages", ".staging/packages"}
