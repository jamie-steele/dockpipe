package composegen

import (
	"strings"
	"testing"
)

func TestRenderContainsServices(t *testing.T) {
	s := Render(DefaultOptions())
	if !strings.Contains(s, "pgvector/pgvector") || !strings.Contains(s, "ollama/ollama") {
		t.Fatalf("unexpected compose:\n%s", s)
	}
}

func TestRenderNoOllama(t *testing.T) {
	o := DefaultOptions()
	o.IncludeOllama = false
	s := Render(o)
	if strings.Contains(s, "ollama/ollama") {
		t.Fatal("expected ollama omitted")
	}
}
