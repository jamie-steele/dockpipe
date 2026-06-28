package application

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String()), runErr
}

func TestCmdGetStateFields(t *testing.T) {
	wd := t.TempDir()
	pkgRoot := filepath.Join(wd, "packages", "Pipeon Dev")
	t.Setenv("DOCKPIPE_WORKDIR", wd)
	t.Setenv("DOCKPIPE_PACKAGE_ROOT", pkgRoot)
	t.Setenv("DOCKPIPE_PACKAGE_ID", "")
	t.Setenv("DOCKPIPE_PACKAGE_STATE_DIR", "")
	t.Setenv("DOCKPIPE_STATE_DIR", "")

	gotState, err := captureStdout(t, func() error {
		return cmdGet([]string{"state_dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe"); gotState != want {
		t.Fatalf("state_dir = %q want %q", gotState, want)
	}

	gotID, err := captureStdout(t, func() error {
		return cmdGet([]string{"package_id"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "pipeon-dev" {
		t.Fatalf("package_id = %q want pipeon-dev", gotID)
	}

	gotPackageState, err := captureStdout(t, func() error {
		return cmdGet([]string{"package_state_dir"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wd, "bin", ".dockpipe", "packages", "pipeon-dev"); gotPackageState != want {
		t.Fatalf("package_state_dir = %q want %q", gotPackageState, want)
	}
}
