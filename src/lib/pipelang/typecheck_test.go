package pipelang

import (
	"strings"
	"testing"
)

func TestCheckValid(t *testing.T) {
	prog, err := Parse([]byte(sampleProgram))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Check(prog); err != nil {
		t.Fatalf("check: %v", err)
	}
}

func TestCheckInterfaceMismatch(t *testing.T) {
	src := `Interface A { int X; string Label(); }
Class B : A { int X = 1; }`
	prog, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Check(prog)
	if err == nil || !strings.Contains(err.Error(), "missing interface method") {
		t.Fatalf("expected missing interface method error, got %v", err)
	}
}

func TestInvokeMethod(t *testing.T) {
	out, err := Invoke([]byte(sampleProgram), "DefaultDeployConfig", "FullImage", nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if out.Type != TypeString || out.Value.String != "nginx:latest" {
		t.Fatalf("unexpected invoke result: %#v", out)
	}
}
