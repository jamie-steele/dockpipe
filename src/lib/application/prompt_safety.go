package application

import (
	"fmt"
	"os"
)

func applyPromptSafetyCLIFromOpts(opts *CliOpts) map[string]string {
	if opts == nil || !opts.ApproveSystemChanges {
		return nil
	}
	return map[string]string{
		"DOCKPIPE_APPROVE_PROMPTS": "1",
	}
}

func mergePromptSafetyCLIIntoEnv(env map[string]string, opts *CliOpts) {
	if env == nil {
		return
	}
	o := applyPromptSafetyCLIFromOpts(opts)
	if o == nil {
		return
	}
	for k, v := range o {
		env[k] = v
	}
	fmt.Fprintln(os.Stderr, "[dockpipe] --yes: DOCKPIPE_APPROVE_PROMPTS=1")
}
