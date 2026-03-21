package domain

import "testing"

func TestFromResolverMapRuntimeKeysPrecedence(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RUNTIME_IMAGE_TEMPLATE": "from-runtime",
		"DOCKPIPE_RESOLVER_TEMPLATE":      "from-resolver",
	})
	if ra.Template != "from-runtime" {
		t.Fatalf("Template: got %q", ra.Template)
	}
}

func TestFromResolverMapResolverFallback(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_TEMPLATE": "legacy",
	})
	if ra.Template != "legacy" {
		t.Fatalf("Template: got %q", ra.Template)
	}
}

func TestFromResolverMapRuntimeType(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RUNTIME_TYPE": "IDE",
	})
	if ra.RuntimeKind != RuntimeKindIDE {
		t.Fatalf("RuntimeKind: got %q want %q", ra.RuntimeKind, RuntimeKindIDE)
	}
}

func TestFromResolverMapRuntimeTypeUnknownPreserved(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RUNTIME_TYPE": "future-kind",
	})
	if ra.RuntimeKind != "future-kind" {
		t.Fatalf("RuntimeKind: got %q", ra.RuntimeKind)
	}
}

func TestFromResolverMapRuntimeTypeEmpty(t *testing.T) {
	ra := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_TEMPLATE": "x",
	})
	if ra.RuntimeKind != "" {
		t.Fatalf("RuntimeKind: got %q want empty", ra.RuntimeKind)
	}
}
