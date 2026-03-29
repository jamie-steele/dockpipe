package application

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// opLookPathFn is exec.LookPath for the secrets CLI binary (1Password’s `op`); overridden in tests.
var opLookPathFn = exec.LookPath

// runOpInjectFn runs `op inject -i <template> -o -` (vault-backed template → env); overridden in tests.
var runOpInjectFn = func(templatePath string) ([]byte, error) {
	cmd := exec.Command("op", "inject", "-i", templatePath, "-o", "-")
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("op inject: %w\n%s", err, msg)
		}
		return nil, fmt.Errorf("op inject: %w", err)
	}
	return out, nil
}

func opInjectWanted(opts *CliOpts) bool {
	if opts != nil && opts.NoOpInject {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DOCKPIPE_OP_INJECT")))
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func workflowVaultSkipInject(wf *domain.Workflow) bool {
	if wf == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(wf.Vault)) {
	case "none", "off", "false", "no", "0":
		return true
	default:
		return false
	}
}

func workflowVaultRequiresOp(wf *domain.Workflow) bool {
	if wf == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(wf.Vault)) {
	case "op", "1password":
		return true
	default:
		return false
	}
}

// mergeOpInjectFromProjectIfEnabled resolves vault references via `op inject` when project config
// sets secrets.vault_template or secrets.op_inject_template and the template file exists.
// Workflow YAML vault: op / 1password require a template; vault: none|off skips for this workflow.
// Resolved KEY=VAL pairs overwrite env (vault over workflow .env for those keys).
func mergeOpInjectFromProjectIfEnabled(env map[string]string, opts *CliOpts, wfRoot string, wf *domain.Workflow) error {
	if !opInjectWanted(opts) {
		return nil
	}
	if workflowVaultSkipInject(wf) {
		return nil
	}
	start := ""
	if opts != nil {
		start = strings.TrimSpace(opts.Workdir)
	}
	if start == "" {
		if wd, err := os.Getwd(); err == nil {
			start = wd
		} else {
			start = wfRoot
		}
	}
	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(start)
	if err != nil {
		return err
	}
	cfg, err := domain.LoadDockpipeProjectConfig(projectRoot)
	if err != nil {
		return err
	}
	if cfg == nil {
		if workflowVaultRequiresOp(wf) {
			return fmt.Errorf("workflow vault: op requires %s with secrets.vault_template or secrets.op_inject_template", domain.DockpipeProjectConfigFileName)
		}
		return nil
	}
	tmplPath, ok := domain.ResolveVaultTemplatePath(cfg, projectRoot)
	if !ok {
		if workflowVaultRequiresOp(wf) {
			return fmt.Errorf("workflow vault: op requires secrets.vault_template or secrets.op_inject_template in %s", domain.DockpipeProjectConfigFileName)
		}
		return nil
	}
	if _, err := os.Stat(tmplPath); err != nil {
		if workflowVaultRequiresOp(wf) {
			return fmt.Errorf("workflow vault: op requires vault template file at %s: %w", tmplPath, err)
		}
		return nil
	}
	if _, err := opLookPathFn("op"); err != nil {
		return fmt.Errorf("vault inject: `op` not in PATH (install 1Password CLI — it provides op inject) or set DOCKPIPE_OP_INJECT=0 to skip (template %s)", tmplPath)
	}
	out, err := runOpInjectFn(tmplPath)
	if err != nil {
		return err
	}
	m, err := infrastructure.ParseEnvBytes(out)
	if err != nil {
		return fmt.Errorf("parse vault inject output: %w", err)
	}
	for k, v := range m {
		env[k] = v
	}
	if len(m) > 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] vault: merged %d key(s) via op inject (%s)\n", len(m), tmplPath)
	}
	return nil
}
