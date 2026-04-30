package application

import "testing"

func TestCmdTestRunsPackageAndWorkflowLanes(t *testing.T) {
	prevPkg := runPackageTestFromFlagsFn
	prevWf := runWorkflowTestsFromFlagsFn
	defer func() {
		runPackageTestFromFlagsFn = prevPkg
		runWorkflowTestsFromFlagsFn = prevWf
	}()
	var pkgCalls, wfCalls int
	runPackageTestFromFlagsFn = func(workdir, only string) error {
		pkgCalls++
		return nil
	}
	runWorkflowTestsFromFlagsFn = func(workdir, only string) error {
		wfCalls++
		return nil
	}
	if err := cmdTest([]string{"--no-workflows"}); err != nil {
		t.Fatal(err)
	}
	if pkgCalls != 1 || wfCalls != 0 {
		t.Fatalf("unexpected calls after --no-workflows: pkg=%d wf=%d", pkgCalls, wfCalls)
	}
	if err := cmdTest([]string{"--no-packages"}); err != nil {
		t.Fatal(err)
	}
	if pkgCalls != 1 || wfCalls != 1 {
		t.Fatalf("unexpected calls after --no-packages: pkg=%d wf=%d", pkgCalls, wfCalls)
	}
}
