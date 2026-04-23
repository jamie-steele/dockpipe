package application

import (
	"testing"

	"dockpipe/src/lib/domain"
)

func TestEffectiveRuntimeProfileNameCLI(t *testing.T) {
	opts := &CliOpts{Runtime: "docker-node"}
	wf := &domain.Workflow{Runtime: "wf"}
	if got := EffectiveRuntimeProfileName(opts, wf, false); got != "docker-node" {
		t.Fatalf("got %q", got)
	}
}

func TestEffectiveResolverProfileNameWorkflow(t *testing.T) {
	opts := &CliOpts{}
	wf := &domain.Workflow{Resolver: "claude"}
	if got := EffectiveResolverProfileName(opts, wf, false); got != "claude" {
		t.Fatalf("got %q", got)
	}
}

func TestEffectiveResolverProfileNameStepsPrefersTopLevel(t *testing.T) {
	wf := &domain.Workflow{Resolver: "top", Runtime: "rt"}
	if got := EffectiveResolverProfileName(nil, wf, true); got != "top" {
		t.Fatalf("got %q", got)
	}
}

func TestProfileLabelForEnv(t *testing.T) {
	if got := ProfileLabelForEnv("r", "s"); got != "s" {
		t.Fatalf("got %q", got)
	}
	if got := ProfileLabelForEnv("r", ""); got != "r" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateRuntimeAllowlistNoRestriction(t *testing.T) {
	wf := &domain.Workflow{}
	if err := ValidateRuntimeAllowlist(wf, "anything"); err != nil {
		t.Fatal(err)
	}
}
