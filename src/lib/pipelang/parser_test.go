package pipelang

import "testing"

const sampleProgram = `/// Deployment contract.
public Interface DeployConfig
{
    /// Base image name.
    public string Image;
    public int Replicas;
    public bool Public;
    public string FullImage();
}

/// Concrete deployment config.
public Class DefaultDeployConfig : DeployConfig
{
    public string Image = "nginx";
    public int Replicas = 1;
    public bool Public = false;
    private string InternalSuffix = ":latest";

    public string FullImage() => Image + InternalSuffix;
    public bool IsScaled() => Replicas > 1;
    private bool IsTiny() => Replicas < 1;
}
`

func TestParseProgram(t *testing.T) {
	prog, err := Parse([]byte(sampleProgram))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Interfaces) != 1 {
		t.Fatalf("interfaces=%d", len(prog.Interfaces))
	}
	if len(prog.Classes) != 1 {
		t.Fatalf("classes=%d", len(prog.Classes))
	}
	c := prog.Classes[0]
	if c.Name != "DefaultDeployConfig" {
		t.Fatalf("class=%q", c.Name)
	}
	if len(c.Methods) != 3 {
		t.Fatalf("methods=%d", len(c.Methods))
	}
	if c.Visibility != VisibilityPublic {
		t.Fatalf("class visibility=%q", c.Visibility)
	}
	if c.Fields[3].Visibility != VisibilityPrivate {
		t.Fatalf("expected private field")
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse([]byte(`Class X { string Name = ; }`))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseStruct(t *testing.T) {
	prog, err := Parse([]byte(`public Struct Values { public string Mode = "remote"; }`))
	if err != nil {
		t.Fatalf("parse struct: %v", err)
	}
	if len(prog.Classes) != 1 {
		t.Fatalf("classes=%d", len(prog.Classes))
	}
	if prog.Classes[0].Name != "Values" {
		t.Fatalf("class=%q", prog.Classes[0].Name)
	}
}
