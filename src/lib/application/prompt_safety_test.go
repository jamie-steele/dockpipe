package application

import "testing"

func TestApplyPromptSafetyCLIFromOpts(t *testing.T) {
	out := applyPromptSafetyCLIFromOpts(&CliOpts{ApproveSystemChanges: true})
	if out["DOCKPIPE_APPROVE_PROMPTS"] != "1" {
		t.Fatalf("unexpected env map: %#v", out)
	}
	if got := applyPromptSafetyCLIFromOpts(&CliOpts{}); got != nil {
		t.Fatalf("expected nil env map when flag is unset, got %#v", got)
	}
}
