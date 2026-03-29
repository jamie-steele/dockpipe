package application

import (
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const terraformUsageText = `dockpipe terraform — Terraform pipeline helpers (terraform-pipeline.sh / DOCKPIPE_TF_*)

Preferred on the main CLI (same env, one invocation with your workflow):

  dockpipe --workflow <name> --tf plan
  dockpipe --workflow <name> --tf apply --tf-no-auto-approve
  dockpipe --workflow <name> --tf-dry-run --tf plan

Usage (standalone helper — same effect as --tf on dockpipe run):
  dockpipe terraform pipeline-path
  dockpipe terraform run <commands> [flags]
  dockpipe terraform <commands> [flags]    (shorthand for "run")

  commands   Comma-separated or space-separated: init, plan, apply, validate, fmt, import
             Examples: plan | apply | init,plan | init plan

Flags (for run / shorthand):
  --workflow <name>   Workflow that runs Terraform (default: dockpipe.cloudflare.r2infra)
  --workdir <path>    Project root (default: current directory)
  --dry-run           Set DOCKPIPE_TF_DRY_RUN=1 (print pipeline steps only)
  --no-auto-approve   Set DOCKPIPE_TF_APPLY_AUTO_APPROVE=0 for apply (interactive confirm)

pipeline-path prints the absolute path to terraform-pipeline.sh (dockpipe.terraform.core package).

See src/core/assets/scripts/README.md (DOCKPIPE_TF_* reference),
packages/terraform/resolvers/terraform-core/README.md (workflow dockpipe.terraform.core), and
packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/README.md.

`

func cmdTerraform(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(terraformUsageText)
		return nil
	}
	switch args[0] {
	case "pipeline-path":
		if len(args) > 1 {
			return fmt.Errorf("unexpected argument %q", args[1])
		}
		rr, err := infrastructure.RepoRoot()
		if err != nil {
			return err
		}
		p, err := infrastructure.ResolveCoreNamespacedScriptPath(rr, "assets.scripts.terraform-pipeline.sh")
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	case "run":
		return cmdTerraformRun(args[1:])
	default:
		return cmdTerraformRun(args)
	}
}

func cmdTerraformRun(args []string) error {
	wf := "dockpipe.cloudflare.r2infra"
	wd := ""
	dryRun := false
	noAutoApprove := false

	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "--workflow":
			if i+1 >= len(args) {
				return fmt.Errorf("--workflow requires a workflow name")
			}
			wf = strings.TrimSpace(args[i+1])
			if wf == "" {
				return fmt.Errorf("--workflow requires a non-empty name")
			}
			i += 2
		case "--workdir":
			if i+1 >= len(args) {
				return fmt.Errorf("--workdir requires a path")
			}
			wd = strings.TrimSpace(args[i+1])
			i += 2
		case "--dry-run":
			dryRun = true
			i++
		case "--no-auto-approve":
			noAutoApprove = true
			i++
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown flag %q (try: dockpipe terraform --help)", a)
			}
			goto positional
		}
	}
positional:
	if i >= len(args) {
		return fmt.Errorf("terraform run: expected commands (e.g. plan, apply, init,plan) — try: dockpipe terraform plan")
	}
	cmdList := joinTerraformCommands(args[i:])
	if err := validateTerraformCommands(cmdList); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[dockpipe] terraform: DOCKPIPE_TF_COMMANDS=%s — running --workflow %s\n", cmdList, wf)
	if wd != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] terraform: --workdir %s\n", wd)
	}

	return withTerraformRunEnv(cmdList, dryRun, noAutoApprove, func() error {
		argv := []string{"--workflow", wf}
		if wd != "" {
			argv = append(argv, "--workdir", wd)
		}
		argv = append(argv, "--")
		return Run(argv, os.Environ())
	})
}

// normalizeTerraformCommandList turns user input into a comma-separated list. The shell pipeline
// strips all whitespace from DOCKPIPE_TF_COMMANDS, so "init plan" would become "initplan" — we
// normalize spaces and commas into comma-separated tokens only.
func normalizeTerraformCommandList(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", " ")
	return strings.Join(strings.Fields(s), ",")
}

func joinTerraformCommands(args []string) string {
	return normalizeTerraformCommandList(strings.Join(args, " "))
}

func validateTerraformCommands(s string) error {
	norm := normalizeTerraformCommandList(s)
	if norm == "" {
		return fmt.Errorf("terraform commands must be non-empty")
	}
	for _, p := range strings.Split(norm, ",") {
		if err := validateOneTerraformCommand(p); err != nil {
			return err
		}
	}
	return nil
}

func validateOneTerraformCommand(p string) error {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "init", "plan", "apply", "validate", "fmt", "import":
		return nil
	default:
		return fmt.Errorf("unknown terraform command %q (use init, plan, apply, validate, fmt, import)", p)
	}
}

// applyTerraformCLIFromOpts builds DOCKPIPE_TF_* overrides from CliOpts (shared by --tf flags and
// dockpipe terraform run).
func applyTerraformCLIFromOpts(opts *CliOpts) (map[string]string, error) {
	if opts == nil {
		return nil, nil
	}
	out := make(map[string]string)
	if strings.TrimSpace(opts.TfCommands) != "" {
		norm := normalizeTerraformCommandList(opts.TfCommands)
		if err := validateTerraformCommands(norm); err != nil {
			return nil, err
		}
		out["DOCKPIPE_TF_COMMANDS"] = norm
	}
	if opts.TfDryRun {
		out["DOCKPIPE_TF_DRY_RUN"] = "1"
	}
	if opts.TfNoAutoApprove {
		out["DOCKPIPE_TF_APPLY_AUTO_APPROVE"] = "0"
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// mergeTerraformCLIIntoEnv merges DOCKPIPE_TF_* from --tf / --tf-dry-run / --tf-no-auto-approve into the workflow env map.
func mergeTerraformCLIIntoEnv(env map[string]string, opts *CliOpts) error {
	if opts == nil {
		return nil
	}
	o, err := applyTerraformCLIFromOpts(opts)
	if err != nil || o == nil {
		return err
	}
	for k, v := range o {
		env[k] = v
	}
	if cmd := o["DOCKPIPE_TF_COMMANDS"]; cmd != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] --tf: DOCKPIPE_TF_COMMANDS=%s\n", cmd)
	}
	return nil
}

func withTerraformRunEnv(commands string, dryRun, noAutoApprove bool, fn func() error) error {
	opts := &CliOpts{TfCommands: commands, TfDryRun: dryRun, TfNoAutoApprove: noAutoApprove}
	o, err := applyTerraformCLIFromOpts(opts)
	if err != nil {
		return err
	}
	if o == nil {
		return fn()
	}
	old := make(map[string]struct {
		val string
		had bool
	})
	for k, v := range o {
		prev, had := os.LookupEnv(k)
		old[k] = struct {
			val string
			had bool
		}{prev, had}
		os.Setenv(k, v)
	}
	defer func() {
		for k, s := range old {
			if s.had {
				os.Setenv(k, s.val)
			} else {
				os.Unsetenv(k)
			}
		}
	}()
	if cmd := o["DOCKPIPE_TF_COMMANDS"]; cmd != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] terraform: DOCKPIPE_TF_COMMANDS=%s\n", cmd)
	}
	return fn()
}
