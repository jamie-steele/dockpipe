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

func TestInvokePrivateMethodDenied(t *testing.T) {
	_, err := Invoke([]byte(sampleProgram), "DefaultDeployConfig", "IsTiny", nil)
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("expected private method invoke failure, got %v", err)
	}
}

func TestCheckCompoundTypes(t *testing.T) {
	src := `
Interface IImageResource { string Path; }
Interface IImagePicker { List<string> Labels; List<IImageResource> Images; }
Class ImagePicker : IImagePicker {
  public List<string> Labels;
  public List<IImageResource> Images;
}`
	prog, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Check(prog); err != nil {
		t.Fatalf("check compound types: %v", err)
	}
}

func TestCheckReservedIComparablePlumbing(t *testing.T) {
	src := `Class ImageResource : IComparable { public string Path = "a"; }`
	prog, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Check(prog); err != nil {
		t.Fatalf("reserved IComparable should be accepted for now: %v", err)
	}
}
