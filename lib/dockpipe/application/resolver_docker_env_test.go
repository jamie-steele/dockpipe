package application

import (
	"testing"
)

func TestMergeEnvHintKeys(t *testing.T) {
	t.Run("extra env wins over host", func(t *testing.T) {
		dst := map[string]string{"OPENAI_API_KEY": "from-cli"}
		src := map[string]string{"OPENAI_API_KEY": "from-host"}
		mergeEnvHintKeys(dst, src, "OPENAI_API_KEY")
		if dst["OPENAI_API_KEY"] != "from-cli" {
			t.Fatalf("expected cli value kept, got %#v", dst)
		}
	})
	t.Run("host fills when dst empty", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"OPENAI_API_KEY": "from-host"}
		mergeEnvHintKeys(dst, src, "OPENAI_API_KEY")
		if dst["OPENAI_API_KEY"] != "from-host" {
			t.Fatalf("expected host copy, got %#v", dst)
		}
	})
	t.Run("comma-separated hints", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"ANTHROPIC_API_KEY": "a", "CLAUDE_API_KEY": "c"}
		mergeEnvHintKeys(dst, src, "ANTHROPIC_API_KEY, CLAUDE_API_KEY")
		if dst["ANTHROPIC_API_KEY"] != "a" || dst["CLAUDE_API_KEY"] != "c" {
			t.Fatalf("expected both, got %#v", dst)
		}
	})
	t.Run("nil ra path is no-op in mergeResolverAuthEnvFromHost", func(t *testing.T) {
		dst := map[string]string{}
		src := map[string]string{"OPENAI_API_KEY": "x"}
		mergeResolverAuthEnvFromHost(dst, src, nil)
		if len(dst) != 0 {
			t.Fatalf("expected no keys, got %#v", dst)
		}
	})
}
