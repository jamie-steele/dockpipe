package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure/fetchinstall"
)

const (
	envInstallBaseURL           = "DOCKPIPE_INSTALL_BASE_URL"
	envInstallVersion           = "DOCKPIPE_INSTALL_VERSION"
	envInstallAllowInsecureHTTP = "DOCKPIPE_INSTALL_ALLOW_INSECURE_HTTP"
	envInstallManifest          = "DOCKPIPE_INSTALL_MANIFEST"
)

func cmdInstall(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printInstallUsage()
		return nil
	}
	switch args[0] {
	case "core":
		return cmdInstallCore(args[1:])
	default:
		return fmt.Errorf("unknown install target %q (try: dockpipe install core)", args[0])
	}
}

func cmdInstallCore(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printInstallCoreUsage()
		return nil
	}
	var (
		workdir      string
		baseURL      string
		tarballURL   string
		version      string
		wantSHA      string
		dryRun       bool
		manifestFile string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--base-url" && i+1 < len(args):
			baseURL = args[i+1]
			i++
		case args[i] == "--url" && i+1 < len(args):
			tarballURL = args[i+1]
			i++
		case args[i] == "--version" && i+1 < len(args):
			version = args[i+1]
			i++
		case args[i] == "--sha256" && i+1 < len(args):
			wantSHA = args[i+1]
			i++
		case args[i] == "--manifest" && i+1 < len(args):
			manifestFile = args[i+1]
			i++
		case args[i] == "--dry-run":
			dryRun = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe install core --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	} else {
		var err error
		workdir, err = filepath.Abs(workdir)
		if err != nil {
			return err
		}
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envInstallBaseURL))
	}
	if version == "" {
		version = strings.TrimSpace(os.Getenv(envInstallVersion))
	}
	if manifestFile == "" {
		manifestFile = strings.TrimSpace(os.Getenv(envInstallManifest))
	}
	allowInsecure := isTruthyEnv(envInstallAllowInsecureHTTP)

	opts := fetchinstall.CoreOptions{
		BaseURL:           strings.TrimSpace(baseURL),
		ExactTarballURL:   strings.TrimSpace(tarballURL),
		Version:           version,
		Workdir:           workdir,
		ExpectedSHA256:    strings.TrimSpace(wantSHA),
		ManifestFile:      manifestFile,
		DryRun:            dryRun,
		UserAgent:         "dockpipe/install",
		AllowInsecureHTTP: allowInsecure,
	}
	ctx := context.Background()
	return fetchinstall.InstallTemplatesCore(ctx, opts)
}

func isTruthyEnv(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func printInstallUsage() {
	fmt.Print(installUsageText)
}

func printInstallCoreUsage() {
	fmt.Print(installCoreUsageText)
}

const installUsageText = `dockpipe install

Download DockPipe template bundles from HTTPS (e.g. a public URL on Cloudflare R2
or a custom domain in front of R2). Does not run bash workflows.

Usage:
  dockpipe install core [options]

See: dockpipe install core --help
`

const installCoreUsageText = `dockpipe install core

Fetch templates/core as a gzip tar (archive paths must start with core/) and
extract to <workdir>/templates/core, replacing any existing templates/core.
After extract, the CLI re-reads the archive and checks every file on disk matches
(then prints the tarball sha256).

Resolution:
  • --url <URL>     Download this tarball (.tar.gz). Optional .sha256 sibling.
  • Otherwise       DOCKPIPE_INSTALL_BASE_URL or --base-url is required.
  • --version latest (default)  GET <base>/install-manifest.json (override name with
                    DOCKPIPE_INSTALL_MANIFEST or --manifest) and use packages.core.
  • --version X.Y.Z            GET <base>/templates-core-X.Y.Z.tar.gz (+ optional .sha256).

Environment:
  DOCKPIPE_INSTALL_BASE_URL     HTTPS origin for manifest and tarballs (no trailing slash).
  DOCKPIPE_INSTALL_VERSION      Default version selector if --version omitted (e.g. latest).
  DOCKPIPE_INSTALL_MANIFEST     Manifest filename (default install-manifest.json).
  DOCKPIPE_INSTALL_ALLOW_INSECURE_HTTP   Set to 1 for http:// (local tests only).

Options:
  --workdir <path>   Project root (default: current directory).
  --base-url <url>   Overrides DOCKPIPE_INSTALL_BASE_URL.
  --url <url>        Full tarball URL; skips manifest / version naming.
  --version <ver>    latest | semver (e.g. 0.6.0).
  --sha256 <hex>     Expected digest when --url is set and no .sha256 file.
  --manifest <file>  Manifest path under base URL (default install-manifest.json).
  --dry-run          Print what would be fetched; do not write files.

Manifest JSON (install-manifest.json):
  {"schema":1,"packages":{"core":{"tarball":"templates-core-0.6.0.tar.gz","sha256":"<hex>"}}}

Package the archive from a dockpipe checkout:
  bash scripts/dockpipe/package-templates-core.sh

Upload with dockpipe --workflow r2-publish or aws s3 cp to the same base URL.

`
