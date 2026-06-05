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

func TestListCatalogWorkflowsBuildsStructuredTypedInputs(t *testing.T) {
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
  DOCKPIPE_VM_IMAGE_PATH: C:\example\demo-image.qcow2
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "DemoVmWorkflowConfig.pipe"), []byte(`public Interface IImageResource
{
    /// <summary>Image path.</summary>
    public string Path;
    public string Format;
}

public Class ImageResource : IImageResource
{
    [EnvName = "DOCKPIPE_VM_IMAGE_PATH"]
    public string Path;
    [EnvName = "DOCKPIPE_VM_IMAGE_FORMAT"]
    public string Format = "qcow2";
}

public Class DemoVmWorkflowConfig
{
    /// <summary>VM image settings.</summary>
    [DisplayName = "VM Image"]
    public IImageResource Image;

    /// <summary>Optional tags.</summary>
    public List<string> Tags;
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
		t.Fatalf("expected 2 top-level typed inputs, got %#v", got[0].Inputs)
	}
	image := got[0].Inputs[0]
	if image.FieldName != "Image" || image.Type != "IImageResource" {
		t.Fatalf("unexpected image input %#v", image)
	}
	if len(image.Children) != 2 {
		t.Fatalf("expected nested image children, got %#v", image.Children)
	}
	if image.Children[0].EnvName != "DOCKPIPE_VM_IMAGE_PATH" {
		t.Fatalf("unexpected nested env mapping %#v", image.Children[0])
	}
	if image.Children[0].Description != "Image path." {
		t.Fatalf("unexpected nested doc %#v", image.Children[0])
	}
	if image.Children[1].EnvName != "DOCKPIPE_VM_IMAGE_FORMAT" {
		t.Fatalf("unexpected nested env mapping %#v", image.Children[1])
	}
	if image.Children[1].DefaultValue != "qcow2" {
		t.Fatalf("unexpected nested default %#v", image.Children[1])
	}
	tags := got[0].Inputs[1]
	if tags.Type != "List<string>" || tags.ElementType != "string" {
		t.Fatalf("unexpected tags input %#v", tags)
	}
	if tags.EnvName != "DOCKPIPE_VM_TAGS" {
		t.Fatalf("unexpected tags env mapping %#v", tags)
	}
}

func TestListCatalogWorkflowsIncludesWorkflowView(t *testing.T) {
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
view:
  entry:
    type: choice
    field: General.BootSource
    title: VM Source
    options:
      - value: image
        label: Boot existing image
        pages: [source]
      - value: installer-iso
        label: Install from ISO
        next: source
      - value: broken
        next: missing
  pages:
    - id: source
      title: Source
      sections:
        - id: media
          title: Media
          fields:
            - General.BootSource
            - Storage.Disk
            - Missing.Field
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "DemoVmWorkflowConfig.pipe"), []byte(`public Interface IGeneral
{
    [EnvName = "DOCKPIPE_VM_BOOT_SOURCE"]
    public string BootSource;
}

public Class General : IGeneral
{
    public string BootSource = "";
}

public Interface IStorage
{
    [EnvName = "DOCKPIPE_VM_DISK"]
    public string Disk;
}

public Class Storage : IStorage
{
    public string Disk = "";
}

public Class DemoVmWorkflowConfig
{
    public IGeneral General;
    public IStorage Storage;
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
	if got[0].View == nil || len(got[0].View.Pages) != 1 {
		t.Fatalf("expected workflow view pages, got %#v", got[0].View)
	}
	if got[0].View.Entry == nil {
		t.Fatalf("expected workflow entry routing, got %#v", got[0].View)
	}
	if got[0].View.Entry.Field != "General.BootSource" || got[0].View.Entry.Type != "choice" {
		t.Fatalf("unexpected workflow entry %#v", got[0].View.Entry)
	}
	if len(got[0].View.Entry.Options) != 3 {
		t.Fatalf("expected entry options, got %#v", got[0].View.Entry.Options)
	}
	if len(got[0].View.Entry.Options[0].Pages) != 1 || got[0].View.Entry.Options[0].Pages[0] != "source" {
		t.Fatalf("unexpected pages routing %#v", got[0].View.Entry.Options[0])
	}
	if got[0].View.Entry.Options[1].Next != "source" || len(got[0].View.Entry.Options[1].Pages) != 1 {
		t.Fatalf("expected next to infer pages %#v", got[0].View.Entry.Options[1])
	}
	if got[0].View.Entry.Options[2].Next != "" || len(got[0].View.Entry.Options[2].Pages) != 0 {
		t.Fatalf("expected invalid page routing to be pruned %#v", got[0].View.Entry.Options[2])
	}
	page := got[0].View.Pages[0]
	if page.ID != "source" || page.Title != "Source" {
		t.Fatalf("unexpected page %#v", page)
	}
	if len(page.Sections) != 1 {
		t.Fatalf("expected one section, got %#v", page.Sections)
	}
	if len(page.Sections[0].Fields) != 2 {
		t.Fatalf("expected invalid field refs to be pruned, got %#v", page.Sections[0].Fields)
	}
	if page.Sections[0].Fields[0] != "General.BootSource" || page.Sections[0].Fields[1] != "Storage.Disk" {
		t.Fatalf("unexpected field refs %#v", page.Sections[0].Fields)
	}
}
