package application

import (
	"os"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
)

func runHostCommit(workHost, message, bundleOut string, bundleAll bool) error {
	commitIDs := map[string]string{
		"workspace": workHost,
	}
	if value := strings.TrimSpace(bundleOut); value != "" {
		commitIDs["bundle_out"] = value
	}
	if bundleAll {
		commitIDs["bundle_scope"] = "all"
	}
	return infrastructure.RunOperationWithOptions(os.Stderr, "run.host_commit", "Recording host checkpoint…", commitIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		result, err := commitOnHostAppFn(workHost, message, bundleOut, bundleAll)
		if err != nil {
			return err
		}
		if value := strings.TrimSpace(result.Result); value != "" {
			commitIDs["result"] = value
		}
		if value := strings.TrimSpace(result.SkipReason); value != "" {
			commitIDs["skip_reason"] = value
		}
		if value := strings.TrimSpace(result.Branch); value != "" {
			commitIDs["branch"] = value
		}
		if value := strings.TrimSpace(result.BundleOut); value != "" {
			commitIDs["bundle_out"] = value
		}
		if value := strings.TrimSpace(result.BundleScope); value != "" {
			commitIDs["bundle_scope"] = value
		}
		return nil
	})
}
