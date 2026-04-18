package fetchinstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseManifestCore(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"schema":1,"packages":{"core":{"version":"0.6.0","tarball":"templates-core-0.6.0.tar.gz","sha256":"abc"}}}`)
	u, sh, err := parseManifestCore(raw, "https://cdn.example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://cdn.example.com/pkg/templates-core-0.6.0.tar.gz"
	if u != want {
		t.Fatalf("url: got %q want %q", u, want)
	}
	if sh != "abc" {
		t.Fatalf("sha: got %q", sh)
	}
}

func TestParseManifestCoreAbsoluteTarball(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"schema":1,"packages":{"core":{"tarball":"https://other.example/t.tgz","sha256":"deadbeef"}}}`)
	u, sh, err := parseManifestCore(raw, "https://cdn.example.com/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://other.example/t.tgz" || sh != "deadbeef" {
		t.Fatalf("got %q %q", u, sh)
	}
}

func TestParseSHA256File(t *testing.T) {
	t.Parallel()
	s, err := parseSHA256File([]byte("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  foo.tar.gz\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 64 {
		t.Fatalf("len %d", len(s))
	}
}

func TestVerifyExtractedTreeMatchesTarGz(t *testing.T) {
	t.Parallel()
	gz := mustGzipTarCore(t, map[string]string{"core/a.txt": "content"})
	dir := t.TempDir()
	dest := filepath.Join(dir, "templates", "core")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzCore(gz, dest); err != nil {
		t.Fatal(err)
	}
	if err := verifyExtractedTreeMatchesTarGz(gz, dest); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyExtractedTreeMismatch(t *testing.T) {
	t.Parallel()
	gz := mustGzipTarCore(t, map[string]string{"core/a.txt": "content"})
	dir := t.TempDir()
	dest := filepath.Join(dir, "templates", "core")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzCore(gz, dest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "a.txt"), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyExtractedTreeMatchesTarGz(gz, dest); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestExtractTarGzCore(t *testing.T) {
	t.Parallel()
	gz := mustGzipTarCore(t, map[string]string{
		"core/hello.txt": "hi",
		"core/sub/x":     "y",
	})
	dir := t.TempDir()
	dest := filepath.Join(dir, "templates", "core")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzCore(gz, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil || string(b) != "hi" {
		t.Fatalf("hello: %v %q", err, string(b))
	}
	b, err = os.ReadFile(filepath.Join(dest, "sub", "x"))
	if err != nil || string(b) != "y" {
		t.Fatalf("sub: %v %q", err, string(b))
	}
}

func TestExtractTarGzCoreRejectsBadPrefix(t *testing.T) {
	t.Parallel()
	gz := mustGzipTarCore(t, map[string]string{"evil/hello.txt": "x"})
	dir := t.TempDir()
	dest := filepath.Join(dir, "core")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzCore(gz, dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestInstallTemplatesCoreEndToEnd(t *testing.T) {
	t.Parallel()
	payload := mustGzipTarCore(t, map[string]string{"core/a.txt": "content"})
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/install-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"schema":1,"packages":{"core":{"tarball":"templates-core-0.0.0-test.tar.gz","sha256":"%s"}}}`, digest)))
	})
	mux.HandleFunc("/templates-core-0.0.0-test.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(payload)
	})
	srv := newIPv4TestServer(t, mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	err := InstallTemplatesCore(context.Background(), CoreOptions{
		BaseURL:           srv.URL,
		Version:           "latest",
		Workdir:           dir,
		AllowInsecureHTTP: true,
		UserAgent:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "templates", "core", "a.txt"))
	if err != nil || string(b) != "content" {
		t.Fatalf("got %v %q", err, string(b))
	}
}

func mustGzipTarCore(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if strings.HasSuffix(name, "/") {
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
			hdr.Mode = 0o755
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadBytesRejectsHTTP(t *testing.T) {
	t.Parallel()
	_, err := downloadBytes(context.Background(), http.DefaultClient, "http://127.0.0.1/nope", "t", false)
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifySHA256(t *testing.T) {
	t.Parallel()
	if err := verifySHA256([]byte("x"), "", "u"); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("x"))
	h := hex.EncodeToString(sum[:])
	if err := verifySHA256([]byte("x"), h, "u"); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256([]byte("y"), h, "u"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func newIPv4TestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(h)
	srv.Listener = ln
	srv.Start()
	return srv
}
