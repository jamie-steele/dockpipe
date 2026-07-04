package application

import (
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestRunHostCommitLogsStructuredNoopResult(t *testing.T) {
	withRunSeams(t)
	commitOnHostAppFn = func(workdir, message, bundleOut string, bundleAll bool) (infrastructure.HostCommitResult, error) {
		if workdir != "/tmp/wd" || message != "msg" || bundleOut != "" || bundleAll {
			t.Fatalf("unexpected commit args: %q %q %q bundleAll=%v", workdir, message, bundleOut, bundleAll)
		}
		return infrastructure.HostCommitResult{
			Result:     "noop",
			SkipReason: "no_changes",
		}, nil
	}
	stderr, err := captureResultStderr(t, func() error {
		return runHostCommit("/tmp/wd", "msg", "", false)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=run.host_commit",
		"status=start",
		"status=done",
		"result=noop",
		"skip_reason=no_changes",
		"workspace=/tmp/wd",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
