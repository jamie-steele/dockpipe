package pipelang

import "testing"

const sampleProgram = `Interface DeployConfig
{
    string Image;
    int Replicas;
    bool Public;
    string FullImage();
}

Class DefaultDeployConfig : DeployConfig
{
    string Image = "nginx";
    int Replicas = 1;
    bool Public = false;

    string FullImage() => Image + ":latest";
    bool IsScaled() => Replicas > 1;
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
	if len(c.Methods) != 2 {
		t.Fatalf("methods=%d", len(c.Methods))
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse([]byte(`Class X { string Name = ; }`))
	if err == nil {
		t.Fatal("expected parse error")
	}
}
