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

func TestClosureWorkflowDependsOnUmbrellaPackageIncludesResolvers(t *testing.T) {
	repo := t.TempDir()
	workflowsRoot := filepath.Join(repo, "workflows")
	packagesRoot := filepath.Join(repo, "packages", "dorkpipe")
	if err := os.MkdirAll(filepath.Join(workflowsRoot, "brain.optimize"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(packagesRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowsRoot, "brain.optimize", "config.yml"), []byte(`name: brain.optimize
run: echo
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowsRoot, "brain.optimize", "package.yml"), []byte(`schema: 1
name: brain.optimize
version: 0.1.0
kind: workflow
depends: [dorkpipe]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packagesRoot, "package.yml"), []byte(`schema: 1
kind: package
name: dorkpipe
version: 0.6.0
includes_resolvers:
  - dorkpipe
  - dorkpipe-self-analysis
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.DockpipeProjectConfig{
		Schema: 1,
		Compile: domain.DockpipeCompileConfig{
			Workflows: func() *[]string {
				roots := []string{
					filepath.ToSlash(workflowsRoot),
					filepath.ToSlash(filepath.Join(repo, "packages")),
				}
				return &roots
			}(),
		},
	}
	start := filepath.Join(workflowsRoot, "brain.optimize")
	order, res, err := closureWorkflowOrderAndResolvers(repo, repo, start, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || filepath.Base(order[0]) != "brain.optimize" {
		t.Fatalf("unexpected workflow order: %v", order)
	}
	if !res["dorkpipe"] || !res["dorkpipe-self-analysis"] {
		t.Fatalf("expected includes_resolvers to be promoted, got %v", res)
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

func TestValidateCompileOutputsScopedAllowsDependsFromConfiguredStoreSource(t *testing.T) {
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "packages": {
    "sources": [
      {
        "kind": "store",
        "path": "`+filepath.ToSlash(external)+`"
      }
    ]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	localWorkflows := filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "workflows")
	if err := os.MkdirAll(localWorkflows, 0o755); err != nil {
		t.Fatal(err)
	}
	externalWorkflows := filepath.Join(external, "workflows")
	if err := os.MkdirAll(externalWorkflows, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorkflowTar := func(root, name, version, pkgName string, depends []string) {
		t.Helper()
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("name: "+name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		pm := "schema: 1\nname: " + pkgName + "\nversion: " + version + "\nkind: workflow\n"
		if len(depends) > 0 {
			pm += "depends:\n"
			for _, dep := range depends {
				pm += "  - " + dep + "\n"
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "package.yml"), []byte(pm), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(root, "dockpipe-workflow-"+name+"-"+version+".tar.gz")
		if _, err := packagebuild.WriteDirTarGzWithPrefix(dir, out, "workflows/"+name); err != nil {
			t.Fatal(err)
		}
	}

	writeWorkflowTar(localWorkflows, "brain.optimize", "0.1.0", "brain.optimize", []string{"dorkpipe"})
	writeWorkflowTar(externalWorkflows, "docs.orchestrate", "0.6.0", "dorkpipe", nil)

	if err := validateCompileOutputsScoped(repo, false, map[string]bool{"brain.optimize": true}, nil); err != nil {
		t.Fatalf("expected configured external store to satisfy depends, got %v", err)
	}
}

func TestValidateCompileOutputsScopedAllowsDependsFromGlobalStore(t *testing.T) {
	repo := t.TempDir()
	globalRoot := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", globalRoot)
	localWorkflows := filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "workflows")
	if err := os.MkdirAll(localWorkflows, 0o755); err != nil {
		t.Fatal(err)
	}
	globalWorkflows := filepath.Join(globalRoot, "packages", "workflows")
	if err := os.MkdirAll(globalWorkflows, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorkflowTar := func(root, name, version, pkgName string, depends []string) {
		t.Helper()
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("name: "+name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		pm := "schema: 1\nname: " + pkgName + "\nversion: " + version + "\nkind: workflow\n"
		if len(depends) > 0 {
			pm += "depends:\n"
			for _, dep := range depends {
				pm += "  - " + dep + "\n"
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "package.yml"), []byte(pm), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(root, "dockpipe-workflow-"+name+"-"+version+".tar.gz")
		if _, err := packagebuild.WriteDirTarGzWithPrefix(dir, out, "workflows/"+name); err != nil {
			t.Fatal(err)
		}
	}

	writeWorkflowTar(localWorkflows, "brain.optimize", "0.1.0", "brain.optimize", []string{"dorkpipe"})
	writeWorkflowTar(globalWorkflows, "docs.orchestrate", "0.6.0", "dorkpipe", nil)

	if err := validateCompileOutputsScoped(repo, false, map[string]bool{"brain.optimize": true}, nil); err != nil {
		t.Fatalf("expected global store to satisfy depends, got %v", err)
	}
}

func TestValidateCompileOutputsScopedAllowsDependsFromCompileRootPackageManifest(t *testing.T) {
	repo := t.TempDir()
	workflowsRoot := filepath.Join(repo, "workflows")
	localWorkflows := filepath.Join(repo, "bin", ".dockpipe", "internal", "packages", "workflows")
	umbrellaRoot := filepath.Join(repo, "packages", "cloud", "storage")
	if err := os.MkdirAll(workflowsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localWorkflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(umbrellaRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, domain.DockpipeProjectConfigFileName), []byte(`{
  "schema": 1,
  "compile": {
    "workflows": [
      "workflows",
      "packages"
    ]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(umbrellaRoot, "package.yml"), []byte(`schema: 1
kind: package
name: cloud.storage
version: 0.6.0
`), 0o644); err != nil {
		t.Fatal(err)
	}

	writeWorkflowTar := func(root, name, version, pkgName string, depends []string) {
		t.Helper()
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("name: "+name+"\nsteps: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		pm := "schema: 1\nname: " + pkgName + "\nversion: " + version + "\nkind: workflow\n"
		if len(depends) > 0 {
			pm += "depends:\n"
			for _, dep := range depends {
				pm += "  - " + dep + "\n"
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "package.yml"), []byte(pm), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(root, "dockpipe-workflow-"+name+"-"+version+".tar.gz")
		if _, err := packagebuild.WriteDirTarGzWithPrefix(dir, out, "workflows/"+name); err != nil {
			t.Fatal(err)
		}
	}

	writeWorkflowTar(localWorkflows, "secretstore-r2-publish-test", "0.6.0", "secretstore-r2-publish-test", []string{"cloud.storage"})

	if err := validateCompileOutputsScoped(repo, false, map[string]bool{"secretstore-r2-publish-test": true}, nil); err != nil {
		t.Fatalf("expected compile root package manifest to satisfy depends, got %v", err)
	}
}
