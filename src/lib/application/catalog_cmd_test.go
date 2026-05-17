package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListCatalogWorkflowsIncludesTypedInputs(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "workflows", "demo")
	modelsDir := filepath.Join(wfDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(`name: demo
description: Demo workflow
category: app
types:
  - models/DemoVmWorkflowConfig.pipe
vars:
  DOCKPIPE_VM_BOOT_SOURCE: installer-iso
  DOCKPIPE_VM_NET_DEVICE: e1000e
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "DemoVmWorkflowConfig.pipe"), []byte(`/// <summary>Demo contract.</summary>
public Class DemoVmWorkflowConfig
{
    /// <summary>How the VM should boot.</summary>
    [DisplayName = "Boot Source"]
    [Group = "General"]
    public string BootSource = "installer-iso";

    /// <summary>Preferred NIC model.</summary>
    [DisplayName = "Network Adapter"]
    public string NetDevice = "e1000e";
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := listCatalogWorkflows(tmp, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(got))
	}
	if got[0].WorkflowID != "demo" {
		t.Fatalf("unexpected workflow id %q", got[0].WorkflowID)
	}
	if len(got[0].Inputs) != 2 {
		t.Fatalf("expected 2 typed inputs, got %#v", got[0].Inputs)
	}
	if got[0].Inputs[0].EnvName != "DOCKPIPE_VM_BOOT_SOURCE" {
		t.Fatalf("unexpected first env mapping %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[0].Description != "How the VM should boot." {
		t.Fatalf("unexpected first description %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[0].DefaultValue != "installer-iso" {
		t.Fatalf("unexpected first default %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[0].Attributes["displayname"] != "Boot Source" {
		t.Fatalf("unexpected first display attribute %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[0].Attributes["group"] != "General" {
		t.Fatalf("unexpected first group attribute %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[1].EnvName != "DOCKPIPE_VM_NET_DEVICE" {
		t.Fatalf("unexpected second env mapping %#v", got[0].Inputs[1])
	}
	if got[0].Inputs[1].Attributes["displayname"] != "Network Adapter" {
		t.Fatalf("unexpected second display attribute %#v", got[0].Inputs[1])
	}
	if got[0].Vars["DOCKPIPE_VM_NET_DEVICE"] != "e1000e" {
		t.Fatalf("expected vars map to include network default, got %#v", got[0].Vars)
	}
}

func TestListCatalogWorkflowsIncludesExternalResolverTypes(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "dockpipe.config.json"), []byte(`{
  "compile": {
    "workflows": ["packages"]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(tmp, "packages", "vm", "workflows", "windows-vm")
	resolverDir := filepath.Join(tmp, "packages", "vm", "resolvers", "qemu", "models")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resolverDir, 0o755); err != nil {
		t.Fatal(err)
	}
if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(`name: windows-vm
description: Demo workflow
category: app
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe
vars:
  DOCKPIPE_VM_NET_DEVICE: e1000e
  DOCKPIPE_VM_BOOT_SOURCE: installer-iso
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resolverDir, "QemuVmResolverConfig.pipe"), []byte(`public Class QemuVmResolverConfig
{
    [DisplayName = "Network Adapter"]
    public string NetDevice = "e1000e";
    [DisplayName = "Boot Source"]
    public string BootSource = "installer-iso";
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := listCatalogWorkflows(tmp, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(got))
	}
	if len(got[0].Inputs) != 2 {
		t.Fatalf("expected 2 typed inputs, got %#v", got[0].Inputs)
	}
	if got[0].Inputs[0].EnvName != "DOCKPIPE_VM_NET_DEVICE" {
		t.Fatalf("unexpected first env mapping %#v", got[0].Inputs[0])
	}
	if got[0].Inputs[1].EnvName != "DOCKPIPE_VM_BOOT_SOURCE" {
		t.Fatalf("unexpected second env mapping %#v", got[0].Inputs[1])
	}
}
