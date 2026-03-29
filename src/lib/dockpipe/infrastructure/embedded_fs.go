package infrastructure

// embeddedFSPackagesPrefix is the path prefix for bundled maintainer packages inside go:embed (repo-root embed.go).
// It must match the directory name in the checkout that is embedded. It is not used as a default for on-disk
// resolution: bundle/workflow roots come only from dockpipe.config.json compile.workflows / compile.bundles.
const embeddedFSPackagesPrefix = ".staging/packages"
