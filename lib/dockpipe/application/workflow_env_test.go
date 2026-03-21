package application

import "testing"

// TestMergeCommitEnvFromLines_AllowsKnownKeysOnly merges only documented DOCKPIPE_* / GIT_PAT keys from docker log lines.
func TestMergeCommitEnvFromLines_AllowsKnownKeysOnly(t *testing.T) {
	env := map[string]string{
		"DOCKPIPE_COMMIT_MESSAGE": "old",
		"IGNORED":                 "keep",
	}
	mergeCommitEnvFromLines(env, []string{
		"DOCKPIPE_COMMIT_MESSAGE=new msg",
		"DOCKPIPE_WORK_BRANCH=wb",
		"DOCKPIPE_BUNDLE_OUT=out.bundle",
		"DOCKPIPE_BUNDLE_ALL=1",
		"GIT_PAT=token",
		"IGNORED=changed",
		"no_equals_line",
	})
	if env["DOCKPIPE_COMMIT_MESSAGE"] != "new msg" || env["DOCKPIPE_WORK_BRANCH"] != "wb" || env["DOCKPIPE_BUNDLE_OUT"] != "out.bundle" || env["DOCKPIPE_BUNDLE_ALL"] != "1" || env["GIT_PAT"] != "token" {
		t.Fatalf("expected known keys to be merged, got %#v", env)
	}
	if env["IGNORED"] != "keep" {
		t.Fatalf("unexpected merge of unknown key: %#v", env)
	}
}

// TestApplyBranchPrefix sets DOCKPIPE_BRANCH_PREFIX from resolver, template mapping, or preserves preset.
func TestApplyBranchPrefix(t *testing.T) {
	env := map[string]string{}
	applyBranchPrefix(env, "codex", "")
	if env["DOCKPIPE_BRANCH_PREFIX"] != "codex" {
		t.Fatalf("resolver should win, got %q", env["DOCKPIPE_BRANCH_PREFIX"])
	}

	env = map[string]string{}
	applyBranchPrefix(env, "", "agent-dev")
	if env["DOCKPIPE_BRANCH_PREFIX"] != "claude" {
		t.Fatalf("template mapping should apply, got %q", env["DOCKPIPE_BRANCH_PREFIX"])
	}

	env = map[string]string{"DOCKPIPE_BRANCH_PREFIX": "preset"}
	applyBranchPrefix(env, "codex", "agent-dev")
	if env["DOCKPIPE_BRANCH_PREFIX"] != "preset" {
		t.Fatalf("preset value should be preserved, got %q", env["DOCKPIPE_BRANCH_PREFIX"])
	}
}

// TestAppendUniqueEnvAndFirstNonEmpty dedupes env slice keys and picks first non-empty string helper.
func TestAppendUniqueEnvAndFirstNonEmpty(t *testing.T) {
	s := []string{"A=1"}
	s = appendUniqueEnv(s, "A=2")
	if len(s) != 1 {
		t.Fatalf("expected duplicate key to be ignored, got %v", s)
	}
	s = appendUniqueEnv(s, "B=2")
	if len(s) != 2 {
		t.Fatalf("expected unique key to append, got %v", s)
	}

	if got := firstNonEmpty("", "  ", "x", "y"); got != "x" {
		t.Fatalf("firstNonEmpty mismatch: got %q", got)
	}
}
