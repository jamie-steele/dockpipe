package application

import (
	"strings"
	"testing"
)

func TestCmdInstallCoreDryRunRequiresBaseOrURL(t *testing.T) {
	t.Setenv(envInstallBaseURL, "")
	err := cmdInstallCore([]string{"--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "base URL") {
		t.Fatalf("expected base URL error, got %v", err)
	}
}

func TestCmdInstallCoreDryRunWithBaseURL(t *testing.T) {
	t.Setenv(envInstallBaseURL, "")
	err := cmdInstallCore([]string{"--dry-run", "--base-url", "https://cdn.example.com/dockpipe"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdInstallUnknownTarget(t *testing.T) {
	err := cmdInstall([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown install target") {
		t.Fatalf("got %v", err)
	}
}

func TestInstallHelp(t *testing.T) {
	if err := Run([]string{"install"}, nil); err != nil {
		t.Fatal(err)
	}
}
