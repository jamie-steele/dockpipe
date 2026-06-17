package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestClosureWorkflowOrderInject(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "workflows")
	if err := os.MkdirAll(filepath.Join(wfRoot, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wfRoot, "top"), 0o755); err != nil {
		t.Fatal(err)
	}
	baseY := `name: base
run: echo
steps: []
`
	topY := `name: top
run: echo
inject:
  - base
steps: []
`
	if err := os.WriteFile(filepath.Join(wfRoot, "base", "config.yml"), []byte(baseY), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, "top", "config.yml"), []byte(topY), 0o644); err != nil {
		t.Fatal(err)
	}
	start := filepath.Join(wfRoot, "top")
	order, res, err := closureWorkflowOrderAndResolvers(repo, repo, start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("order len: %d %v", len(order), order)
	}
	if filepath.Base(order[0]) != "base" || filepath.Base(order[1]) != "top" {
		t.Fatalf("expected base then top, got %v", order)
	}
	if len(res) != 0 {
		t.Fatalf("resolvers: %v", res)
	}
}

func TestClosureWorkflowInjectResolver(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "workflows")
	if err := os.MkdirAll(filepath.Join(wfRoot, "solo"), 0o755); err != nil {
		t.Fatal(err)
	}
	y := `name: solo
run: echo
inject:
  - resolver: my-resolver
steps: []
`
	if err := os.WriteFile(filepath.Join(wfRoot, "solo", "config.yml"), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	start := filepath.Join(wfRoot, "solo")
	order, res, err := closureWorkflowOrderAndResolvers(repo, repo, start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 {
		t.Fatalf("order: %v", order)
	}
	if !res["my-resolver"] {
		t.Fatalf("expected resolver my-resolver in %v", res)
	}
}

func TestClosureWorkflowImportsMergeInject(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "workflows")
	if err := os.MkdirAll(filepath.Join(wfRoot, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wfRoot, "top"), 0o755); err != nil {
		t.Fatal(err)
	}
	frag := `inject:
  - base
`
	topY := `name: top
run: echo
imports:
  - frag.yml
steps: []
`
	if err := os.WriteFile(filepath.Join(wfRoot, "base", "config.yml"), []byte(`name: base
run: echo
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, "top", "frag.yml"), []byte(frag), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfRoot, "top", "config.yml"), []byte(topY), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := domain.ParseWorkflowFromDisk(
		[]byte(topY),
		filepath.Join(wfRoot, "top"),
		func(p string) ([]byte, error) { return os.ReadFile(p) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Inject) != 1 || w.Inject[0].Workflow != "base" {
		t.Fatalf("merged inject: %+v", w.Inject)
	}
}

func TestValidateCompileOutputsScopedIgnoresUnrelatedResolverTarballs(t *testing.T) {
	repo := t.TempDir()
	resRoot := filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "resolvers")
	if err := os.MkdirAll(resRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeResolver := func(name, ns string) {
		t.Helper()
		dir := filepath.Join(repo, "tmp-"+name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "profile"), []byte("DOCKPIPE_RESOLVER_WORKFLOW=x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		pm := "schema: 1\nname: " + name + "\nversion: 0.0.0\nkind: resolver\n"
		if strings.TrimSpace(ns) != "" {
			pm += "namespace: " + ns + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "package.yml"), []byte(pm), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(resRoot, "dockpipe-resolver-"+name+"-0.0.0.tar.gz")
		if _, err := packagebuild.WriteDirTarGzWithPrefix(dir, out, "resolvers/"+name); err != nil {
			t.Fatal(err)
		}
	}
	writeResolver("qemu", "dockpipe-vm")
	writeResolver("onepassword", "")

	if err := validateCompileOutputsScoped(repo, false, nil, map[string]bool{"qemu": true}); err != nil {
		t.Fatalf("expected scoped validation to ignore unrelated resolver tarballs, got %v", err)
	}
	if err := validateCompileOutputsScoped(repo, false, nil, nil); err == nil || !strings.Contains(err.Error(), "dockpipe-resolver-onepassword-0.0.0.tar.gz") {
		t.Fatalf("expected full validation to catch unrelated missing namespace, got %v", err)
	}
}
