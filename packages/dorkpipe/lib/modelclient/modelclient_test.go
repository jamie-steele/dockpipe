package modelclient

import (
	"strings"
	"testing"
)

func TestOptionsOllamaPayload(t *testing.T) {
	temp := 0.2
	topP := 0.9
	opts := Options{
		NumCtx:      8192,
		Temperature: &temp,
		NumPredict:  256,
		TopP:        &topP,
		Extra: map[string]any{
			"seed": 42,
		},
	}
	got := opts.OllamaPayload()
	checks := map[string]any{
		"num_ctx":     8192,
		"temperature": temp,
		"num_predict": 256,
		"top_p":       topP,
		"seed":        42,
	}
	for key, want := range checks {
		if got[key] != want {
			t.Fatalf("payload[%q] = %#v want %#v", key, got[key], want)
		}
	}
}

func TestDefaultProviderFallback(t *testing.T) {
	t.Setenv("DORKPIPE_MODEL_PROVIDER", "")
	if got := DefaultProvider(); got != "ollama" {
		t.Fatalf("DefaultProvider() = %q want ollama", got)
	}
	t.Setenv("DORKPIPE_MODEL_PROVIDER", "custom")
	if got := DefaultProvider(); !strings.EqualFold(got, "custom") {
		t.Fatalf("DefaultProvider() = %q want custom", got)
	}
}
