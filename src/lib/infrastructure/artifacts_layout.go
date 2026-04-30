package infrastructure

// DefaultRepoArtifactsDir is the repo-relative directory for local publish outputs
// (templates-core tarball, checksums, install-manifest, CI release binaries when built in-tree).
// Tracked content stays under release/docs, release/packaging, etc.; this subdirectory is gitignored.
const DefaultRepoArtifactsDir = "release/artifacts"
