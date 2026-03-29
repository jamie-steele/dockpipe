package application

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// opLookPathFn is exec.LookPath for the secrets CLI binary (1Password’s `op`); overridden in tests.
var opLookPathFn = exec.LookPath

// runOpInjectFn runs `op inject -i <template>` (vault-backed template → env). Per `op inject --help`,
// `--out-file` / `-o` means "write to a file instead of stdout"; omitting `-o` sends the result to
// stdout. Do not pass `-o -` — that is a literal output path named "-" in the current directory, same
// as shell `> -`, not stdout.
var runOpInjectFn = func(templatePath string) ([]byte, error) {
	cmd := exec.Command("op", "inject", "-i", templatePath)
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

func vaultModeSkipInject(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "none", "off", "false", "no", "0":
		return true
	default:
		return false
	}
}

func vaultModeRequiresOp(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "op", "1password":
		return true
	default:
		return false
	}
}

// mergeOpInjectFromProjectIfEnabled resolves vault references via `op inject` when project config
// sets secrets.vault_template or secrets.op_inject_template and the template file exists.
// Effective vault mode: workflow YAML vault: wins when set; else secrets.vault in dockpipe.config.json;
// else best-effort inject when template exists. Strict op mode (workflow or project) requires template + file.
// Resolved KEY=VAL pairs overwrite env (vault over workflow .env for those keys).
func mergeOpInjectFromProjectIfEnabled(env map[string]string, opts *CliOpts, wfRoot string, wf *domain.Workflow) error {
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
	// Run before op-inject opt-out: DOCKPIPE_OP_INJECT=0 / --no-op-inject must still clean up
	// accidental `op inject … > -` (shell creates repo-root "-"); otherwise the file never goes away.
	maybeRemoveStrayDashInjectFile(projectRoot)
	if !opInjectWanted(opts) {
		return nil
	}
	cfg, err := domain.LoadDockpipeProjectConfig(projectRoot)
	if err != nil {
		return err
	}
	mode := domain.EffectiveVaultString(wf, cfg)
	if vaultModeSkipInject(mode) {
		return nil
	}
	if cfg == nil {
		if vaultModeRequiresOp(mode) {
			return fmt.Errorf("workflow vault: op requires %s with secrets.vault_template or secrets.op_inject_template", domain.DockpipeProjectConfigFileName)
		}
		return nil
	}
	tmplPath, ok := domain.ResolveVaultTemplatePath(cfg, projectRoot)
	if !ok {
		if vaultModeRequiresOp(mode) {
			return fmt.Errorf("workflow vault: op requires secrets.vault_template or secrets.op_inject_template in %s", domain.DockpipeProjectConfigFileName)
		}
		return nil
	}
	if _, err := os.Stat(tmplPath); err != nil {
		if vaultModeRequiresOp(mode) {
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

// maybeRemoveStrayDashInjectFile deletes repo-root "-" when it looks like accidental `op inject ... > -`
// output (shell creates a file named "-"). Set DOCKPIPE_KEEP_DASH_FILE=1 to skip removal.
func maybeRemoveStrayDashInjectFile(projectRoot string) {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DOCKPIPE_KEEP_DASH_FILE")))
	if v == "1" || v == "true" || v == "yes" {
		return
	}
	p := filepath.Join(projectRoot, "-")
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return
	}
	if st.Size() > 256*1024 {
		fmt.Fprintf(os.Stderr, "[dockpipe] warning: file %q exists — remove with: rm -- - (large file, not auto-removed)\n", p)
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if len(b) == 0 || bytes.Contains(b, []byte{0}) {
		return
	}
	if !bytes.Contains(b, []byte("=")) {
		return
	}
	ok := false
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if bytes.Contains(line, []byte("=")) {
			ok = true
			break
		}
	}
	if !ok {
		return
	}
	if err := os.Remove(p); err != nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] warning: stray file %q — remove with: rm -- - (%v)\n", p, err)
		return
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] removed stray file %q (from shell `op inject ... > -`); vault merge stays in memory only\n", p)
}
