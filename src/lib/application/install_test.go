package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdInstallCoreDryRunRequiresBaseOrURL(t *testing.T) {
	t.Setenv(envInstallBaseURL, "")
	err := cmdInstallCore([]string{"--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "base URL") {
		t.Fatalf("expected base URL error, got %v", err)
	}
}

func TestCmdInstallCoreDryRunWithBaseURL(t *testing.T) {
	t.Setenv(envInstallBaseURL, "")
	stderr, err := captureResultStderr(t, func() error {
		return cmdInstallCore([]string{"--dry-run", "--base-url", "https://cdn.example.com/dockpipe"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=install.core.dry_run",
		"status=start",
		"status=done",
		"mode=latest",
		"result=dry_run",
		"would GET https://cdn.example.com/dockpipe/install-manifest.json",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected dry-run stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdInstallCoreEmitsOperationResults(t *testing.T) {
	payload := mustGzipTarCoreInstallTest(t, map[string]string{"core/a.txt": "content"})
	mux := http.NewServeMux()
	mux.HandleFunc("/install-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema":1,"packages":{"core":{"tarball":"templates-core-0.0.0-test.tar.gz"}}}`))
	})
	mux.HandleFunc("/templates-core-0.0.0-test.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(payload)
	})
	srv := newIPv4InstallTestServer(t, mux)
	t.Cleanup(srv.Close)
	t.Setenv(envInstallAllowInsecureHTTP, "1")

	dir := t.TempDir()
	stderr, err := captureResultStderr(t, func() error {
		return cmdInstallCore([]string{"--workdir", dir, "--base-url", srv.URL})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=install.core",
		"unit=install.core.resolve",
		"unit=install.core.download",
		"unit=install.core.extract",
		"status=start",
		"status=done",
		"mode=latest",
		"checksum=missing",
		"result=installed",
		"templates/core",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected install stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "templates", "core", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "content" {
		t.Fatalf("installed file = %q want content", string(got))
	}
}

func TestCmdInstallUnknownTarget(t *testing.T) {
	err := cmdInstall([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown install target") {
		t.Fatalf("got %v", err)
	}
}

func TestInstallHelp(t *testing.T) {
	if err := Run([]string{"install"}, nil); err != nil {
		t.Fatal(err)
	}
}

func mustGzipTarCoreInstallTest(t *testing.T, files map[string]string) []byte {
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

func newIPv4InstallTestServer(t *testing.T, h http.Handler) *httptest.Server {
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
