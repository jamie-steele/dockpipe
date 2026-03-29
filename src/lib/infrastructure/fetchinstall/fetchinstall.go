// Package fetchinstall downloads DockPipe template bundles over HTTPS and extracts them safely.
package fetchinstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultManifestFile is fetched from the install base URL when version is "latest".
const DefaultManifestFile = "install-manifest.json"

// CorePackageKey is the manifest entry name for templates/core archives (JSON field name in install-manifest.json).
const CorePackageKey = "core"

// TemplatesCoreTarballPrefix is the filename prefix for version-pinned tarballs: templates-core-<ver>.tar.gz
const TemplatesCoreTarballPrefix = "templates-core-"
const templatesCoreTarballSuffix = ".tar.gz"

// CoreOptions configures InstallTemplatesCore.
type CoreOptions struct {
	BaseURL string // e.g. https://cdn.example.com/dockpipe (no trailing slash)
	// ExactTarballURL, if set, skips manifest / version resolution and downloads this URL only.
	ExactTarballURL string
	// Version is "latest" (manifest) or e.g. "0.6.0" for templates-core-0.6.0.tar.gz.
	Version string
	Workdir string
	// ExpectedSHA256 is optional hex digest; when empty, a sibling .sha256 file is fetched when present.
	ExpectedSHA256 string
	ManifestFile   string // default DefaultManifestFile
	DryRun         bool
	UserAgent      string
	HTTPClient     *http.Client
	// AllowInsecureHTTP permits http:// URLs (tests only; set from env in the application layer).
	AllowInsecureHTTP bool
}

// InstallTemplatesCore downloads a gzip tar of templates/core (archive top directory "core/") and
// extracts it to workdir/templates/core, replacing any existing templates/core tree.
func InstallTemplatesCore(ctx context.Context, o CoreOptions) error {
	if strings.TrimSpace(o.Workdir) == "" {
		return fmt.Errorf("workdir is empty")
	}
	wd, err := filepath.Abs(o.Workdir)
	if err != nil {
		return err
	}
	manifestName := strings.TrimSpace(o.ManifestFile)
	if manifestName == "" {
		manifestName = DefaultManifestFile
	}
	ver := strings.TrimSpace(o.Version)
	if ver == "" {
		ver = "latest"
	}

	if o.DryRun {
		s, err := describeDryRunCoreInstall(o, wd, manifestName, ver)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, s)
		return nil
	}

	tarballURL, wantSHA, err := resolveCoreTarballURL(ctx, o, manifestName, ver)
	if err != nil {
		return err
	}
	if wantSHA == "" && o.ExpectedSHA256 != "" {
		wantSHA = strings.TrimSpace(strings.ToLower(o.ExpectedSHA256))
	}

	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	ua := strings.TrimSpace(o.UserAgent)
	if ua == "" {
		ua = "dockpipe/fetchinstall"
	}

	body, err := downloadBytes(ctx, client, tarballURL, ua, o.AllowInsecureHTTP)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := verifySHA256(body, wantSHA, tarballURL); err != nil {
		return err
	}

	destCore := filepath.Join(wd, "templates", "core")
	if err := os.RemoveAll(destCore); err != nil {
		return fmt.Errorf("remove existing templates/core: %w", err)
	}
	if err := os.MkdirAll(destCore, 0o755); err != nil {
		return fmt.Errorf("mkdir templates/core: %w", err)
	}
	if err := extractTarGzCore(body, destCore); err != nil {
		_ = os.RemoveAll(destCore)
		return fmt.Errorf("extract: %w", err)
	}
	if err := verifyExtractedTreeMatchesTarGz(body, destCore); err != nil {
		_ = os.RemoveAll(destCore)
		return fmt.Errorf("post-install checksum: %w", err)
	}
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	if wantSHA != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] install: checksum ok sha256=%s (remote digest + on-disk tree match)\n", hexSum)
	} else {
		fmt.Fprintf(os.Stderr, "[dockpipe] install: checksum ok sha256=%s (on-disk tree match; no remote digest file)\n", hexSum)
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] installed templates/core from %s\n", tarballURL)
	return nil
}

// describeDryRunCoreInstall returns stderr lines without using the network.
func describeDryRunCoreInstall(o CoreOptions, wd, manifestName, ver string) (string, error) {
	var b strings.Builder
	ex := strings.TrimSpace(o.ExactTarballURL)
	if ex != "" {
		if err := validateHTTPURL(ex, o.AllowInsecureHTTP); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "[dockpipe] install dry-run: would fetch %s\n", ex)
		if sh := strings.TrimSpace(strings.ToLower(o.ExpectedSHA256)); sh != "" {
			fmt.Fprintf(&b, "[dockpipe] install dry-run: expect sha256=%s\n", sh)
		} else {
			fmt.Fprintf(&b, "[dockpipe] install dry-run: optional digest: %s.sha256\n", ex)
		}
		fmt.Fprintf(&b, "[dockpipe] install dry-run: would extract to %s\n", filepath.Join(wd, "templates", "core"))
		return b.String(), nil
	}
	base := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if base == "" {
		return "", fmt.Errorf("install base URL is empty (set DOCKPIPE_INSTALL_BASE_URL or use --base-url)")
	}
	if err := validateHTTPURL(base+"/", o.AllowInsecureHTTP); err != nil {
		return "", fmt.Errorf("base url: %w", err)
	}
	if ver == "latest" {
		fmt.Fprintf(&b, "[dockpipe] install dry-run: would GET %s/%s\n", base, manifestName)
		fmt.Fprintf(&b, "[dockpipe] install dry-run: then fetch the core tarball from the manifest and verify sha256 when present\n")
	} else {
		tb := base + "/" + TemplatesCoreTarballPrefix + ver + templatesCoreTarballSuffix
		fmt.Fprintf(&b, "[dockpipe] install dry-run: would fetch %s\n", tb)
		fmt.Fprintf(&b, "[dockpipe] install dry-run: optional digest: %s.sha256\n", tb)
	}
	fmt.Fprintf(&b, "[dockpipe] install dry-run: would extract to %s\n", filepath.Join(wd, "templates", "core"))
	return b.String(), nil
}

func resolveCoreTarballURL(ctx context.Context, o CoreOptions, manifestName, ver string) (tarballURL string, sha string, err error) {
	ex := strings.TrimSpace(o.ExactTarballURL)
	if ex != "" {
		if err := validateHTTPURL(ex, o.AllowInsecureHTTP); err != nil {
			return "", "", err
		}
		sha = strings.TrimSpace(strings.ToLower(o.ExpectedSHA256))
		if sha == "" {
			// Optional sibling digest file.
			if s, e := sha256FromSiblingURL(ctx, o, ex); e == nil && s != "" {
				sha = s
			}
		}
		return ex, sha, nil
	}
	base := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if base == "" {
		return "", "", fmt.Errorf("install base URL is empty (set DOCKPIPE_INSTALL_BASE_URL or use --base-url)")
	}
	if err := validateHTTPURL(base+"/", o.AllowInsecureHTTP); err != nil {
		return "", "", fmt.Errorf("base url: %w", err)
	}

	if ver == "latest" {
		mURL := base + "/" + manifestName
		client := o.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: 2 * time.Minute}
		}
		ua := strings.TrimSpace(o.UserAgent)
		if ua == "" {
			ua = "dockpipe/fetchinstall"
		}
		raw, err := downloadBytes(ctx, client, mURL, ua, o.AllowInsecureHTTP)
		if err != nil {
			return "", "", fmt.Errorf("manifest %s: %w", mURL, err)
		}
		tb, sh, err := parseManifestCore(raw, base)
		if err != nil {
			return "", "", err
		}
		return tb, strings.TrimSpace(strings.ToLower(sh)), nil
	}

	// Pinned version: {base}/templates-core-{ver}.tar.gz
	if strings.Contains(ver, "/") || strings.Contains(ver, "\\") || strings.Contains(ver, "..") {
		return "", "", fmt.Errorf("invalid version %q", ver)
	}
	tb := base + "/" + TemplatesCoreTarballPrefix + ver + templatesCoreTarballSuffix
	if err := validateHTTPURL(tb, o.AllowInsecureHTTP); err != nil {
		return "", "", err
	}
	sha = ""
	if s, err := fetchSHA256File(ctx, o, tb+".sha256"); err == nil {
		sha = s
	}
	return tb, sha, nil
}

func sha256FromSiblingURL(ctx context.Context, o CoreOptions, tarballURL string) (string, error) {
	return fetchSHA256File(ctx, o, tarballURL+".sha256")
}

func fetchSHA256File(ctx context.Context, o CoreOptions, shaURL string) (string, error) {
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	ua := strings.TrimSpace(o.UserAgent)
	if ua == "" {
		ua = "dockpipe/fetchinstall"
	}
	b, err := downloadBytes(ctx, client, shaURL, ua, o.AllowInsecureHTTP)
	if err != nil {
		return "", err
	}
	return parseSHA256File(b)
}

// installManifestJSON is the expected shape of install-manifest.json.
type installManifestJSON struct {
	Schema   int                            `json:"schema"`
	Packages map[string]manifestPackageJSON `json:"packages"`
}

type manifestPackageJSON struct {
	Version string `json:"version"`
	Tarball string `json:"tarball"`
	SHA256  string `json:"sha256"`
}

func parseManifestCore(raw []byte, base string) (tarballURL, sha256hex string, err error) {
	var m installManifestJSON
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", "", fmt.Errorf("parse manifest: %w", err)
	}
	p, ok := m.Packages[CorePackageKey]
	if !ok || strings.TrimSpace(p.Tarball) == "" {
		return "", "", fmt.Errorf("manifest missing tarball for %q", CorePackageKey)
	}
	tb := strings.TrimSpace(p.Tarball)
	if u, err := url.Parse(tb); err == nil && u.Scheme != "" && u.Host != "" {
		return tb, p.SHA256, nil
	}
	if strings.Contains(tb, "..") {
		return "", "", fmt.Errorf("invalid tarball path in manifest")
	}
	rel := strings.Trim(strings.TrimSpace(tb), "/")
	base = strings.TrimRight(base, "/")
	return base + "/" + rel, p.SHA256, nil
}

func parseSHA256File(b []byte) (string, error) {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("empty sha256 file")
	}
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[0]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 64 {
		return "", fmt.Errorf("invalid sha256 length")
	}
	for _, c := range s {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return "", fmt.Errorf("invalid sha256 character")
	}
	return s, nil
}

func verifySHA256(body []byte, wantHex, src string) error {
	if wantHex == "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] warning: no sha256 for %s — verify trust in the URL and network\n", src)
		return nil
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if got != wantHex {
		return fmt.Errorf("sha256 mismatch for %s (expected %s, got %s)", src, wantHex, got)
	}
	return nil
}

func validateHTTPURL(raw string, allowInsecure bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if allowInsecure {
			return nil
		}
		return fmt.Errorf("only https URLs are allowed (set DOCKPIPE_INSTALL_ALLOW_INSECURE_HTTP=1 for http in dev)")
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
}

func downloadBytes(ctx context.Context, client *http.Client, rawURL, userAgent string, allowInsecure bool) ([]byte, error) {
	if err := validateHTTPURL(rawURL, allowInsecure); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("GET %s: %s — %s", rawURL, resp.Status, strings.TrimSpace(string(slurp)))
	}
	return io.ReadAll(resp.Body)
}

// extractTarGzCore expects gzip tar entries named "core/..." and writes files under destCore (templates/core).
func extractTarGzCore(gz []byte, destCore string) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	const prefix = "core/"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.ToSlash(hdr.Name)
		name = strings.TrimPrefix(name, "./")
		if name == "core" || strings.HasPrefix(name, prefix) {
			rel := strings.TrimPrefix(name, prefix)
			if name == "core" {
				rel = ""
			}
			if rel == "" && hdr.Typeflag == tar.TypeDir {
				continue
			}
			if strings.Contains(rel, "..") {
				return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
			}
			target := filepath.Join(destCore, filepath.FromSlash(rel))
			destAbs, _ := filepath.Abs(destCore)
			fullAbs, _ := filepath.Abs(target)
			if !strings.HasPrefix(fullAbs, destAbs+string(filepath.Separator)) && fullAbs != destAbs {
				return fmt.Errorf("path escapes output dir: %q", hdr.Name)
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			case tar.TypeReg, tar.TypeRegA:
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				mode := fsFileMode(hdr.Mode)
				f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
				if err != nil {
					return err
				}
				if _, err := io.Copy(f, tr); err != nil {
					_ = f.Close()
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			case tar.TypeSymlink:
				return fmt.Errorf("symlinks not allowed in template bundle: %q", hdr.Name)
			default:
				// Skip other types (hard links, etc.)
			}
			continue
		}
		return fmt.Errorf("unexpected tar entry %q (expected prefix %q)", hdr.Name, prefix)
	}
	return nil
}

// verifyExtractedTreeMatchesTarGz re-reads the gzip tar and checks every file and directory on disk
// matches the archive (guards partial writes and extract bugs). Must use the same path rules as extractTarGzCore.
func verifyExtractedTreeMatchesTarGz(gz []byte, destCore string) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	const prefix = "core/"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.ToSlash(hdr.Name)
		name = strings.TrimPrefix(name, "./")
		if name == "core" || strings.HasPrefix(name, prefix) {
			rel := strings.TrimPrefix(name, prefix)
			if name == "core" {
				rel = ""
			}
			if rel == "" && hdr.Typeflag == tar.TypeDir {
				continue
			}
			if strings.Contains(rel, "..") {
				return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
			}
			target := filepath.Join(destCore, filepath.FromSlash(rel))
			destAbs, _ := filepath.Abs(destCore)
			fullAbs, _ := filepath.Abs(target)
			if !strings.HasPrefix(fullAbs, destAbs+string(filepath.Separator)) && fullAbs != destAbs {
				return fmt.Errorf("path escapes output dir: %q", hdr.Name)
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				st, err := os.Stat(target)
				if err != nil {
					return fmt.Errorf("missing directory %s: %w", target, err)
				}
				if !st.IsDir() {
					return fmt.Errorf("not a directory %s", target)
				}
			case tar.TypeReg, tar.TypeRegA:
				if hdr.Size < 0 {
					return fmt.Errorf("invalid size for %q", hdr.Name)
				}
				expected := make([]byte, hdr.Size)
				if _, err := io.ReadFull(tr, expected); err != nil {
					return fmt.Errorf("read archive body %q: %w", hdr.Name, err)
				}
				got, err := os.ReadFile(target)
				if err != nil {
					return fmt.Errorf("read installed file %s: %w", target, err)
				}
				if !bytes.Equal(expected, got) {
					return fmt.Errorf("content mismatch after extract for %s (size tar=%d disk=%d)", target, len(expected), len(got))
				}
			case tar.TypeSymlink:
				return fmt.Errorf("symlinks not allowed in template bundle: %q", hdr.Name)
			default:
				return fmt.Errorf("unsupported tar entry %q (type %d)", hdr.Name, hdr.Typeflag)
			}
			continue
		}
		return fmt.Errorf("unexpected tar entry %q (expected prefix %q)", hdr.Name, prefix)
	}
	return nil
}

func fsFileMode(m int64) os.FileMode {
	if m < 0 {
		return 0o644
	}
	// Tar mode may include type bits; keep permission bits only (bounded for G115).
	perm := m & 0o777
	mode := os.FileMode(perm) & 0o777
	if mode == 0 {
		return 0o644
	}
	return mode
}
