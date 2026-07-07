package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func dockpipePath() string {
	if v := strings.TrimSpace(os.Getenv("DOCKPIPE_BIN")); v != "" {
		return v
	}
	p, err := exec.LookPath("dockpipe")
	if err != nil {
		return "dockpipe"
	}
	return p
}

func dorkpipePath() string {
	if v := strings.TrimSpace(os.Getenv("DORKPIPE_BIN")); v != "" {
		return v
	}
	p, err := exec.LookPath("dorkpipe")
	if err != nil {
		return "dorkpipe"
	}
	return p
}

func runDockpipe(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	return runDockpipeWithEnv(ctx, args, nil)
}

func runDockpipeWithEnv(ctx context.Context, args []string, extraEnv map[string]string) (stdout, stderr string, exitCode int, err error) {
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, dockpipePath(), args...)
	if len(extraEnv) > 0 {
		cmd.Env = envWithOverrides(os.Environ(), extraEnv)
	}
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

func envWithOverrides(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]bool{}
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			out = append(out, entry)
			continue
		}
		replaced := false
		for overrideKey, overrideValue := range overrides {
			if strings.EqualFold(key, overrideKey) {
				out = append(out, overrideKey+"="+overrideValue)
				seen[overrideKey] = true
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, entry)
		}
	}
	for key, value := range overrides {
		if !seen[key] {
			out = append(out, key+"="+value)
		}
	}
	return out
}

func runDorkpipe(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, dorkpipePath(), args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

type hostChatSummary struct {
	Provider         string         `json:"provider"`
	Model            string         `json:"model,omitempty"`
	Text             string         `json:"text"`
	Status           string         `json:"status"`
	ExitCode         int            `json:"exit_code"`
	Stdout           string         `json:"stdout,omitempty"`
	Stderr           string         `json:"stderr,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	ReadyToApply     map[string]any `json:"ready_to_apply,omitempty"`
	RequiresApproval bool           `json:"requires_approval,omitempty"`
}

type hostAuthSummary struct {
	Provider string              `json:"provider"`
	Text     string              `json:"text"`
	Status   string              `json:"status"`
	Metadata map[string]any      `json:"metadata,omitempty"`
	Command  string              `json:"command,omitempty"`
	Auth     *providerAuthStatus `json:"auth,omitempty"`
}

type providerAuthStatus struct {
	Provider      string         `json:"provider"`
	Installed     bool           `json:"installed"`
	Authenticated bool           `json:"authenticated"`
	AuthDir       string         `json:"auth_dir,omitempty"`
	ConfigFile    string         `json:"config_file,omitempty"`
	CLIPath       string         `json:"cli_path,omitempty"`
	EnvKeys       []string       `json:"env_keys,omitempty"`
	Issues        []string       `json:"issues,omitempty"`
	NextActions   []string       `json:"next_actions,omitempty"`
	RepairTool    string         `json:"repair_tool,omitempty"`
	LoginCommand  string         `json:"login_command,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type codexSessionBinding struct {
	CodexSessionID string `json:"codex_session_id"`
	Workdir        string `json:"workdir"`
	Model          string `json:"model,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

type codexSessionState struct {
	Sessions map[string]codexSessionBinding `json:"sessions"`
}

func runFixedHostCommand(ctx context.Context, workdir, command string, args []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if strings.TrimSpace(command) == "" {
		return "", "", -1, fmt.Errorf("empty command")
	}
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, command, args...)
	cmd.Dir = workdir
	cmd.Env = os.Environ()
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runCtx.Err() == context.DeadlineExceeded {
		return stdout, stderr, -1, fmt.Errorf("%s timed out after %s", command, timeout)
	}
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

func codexSessionStatePath(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "packages", "dorkpipe", "host-bridge", "codex-sessions.json")
}

func loadCodexSessionState(workdir string) codexSessionState {
	state := codexSessionState{Sessions: map[string]codexSessionBinding{}}
	data, err := os.ReadFile(codexSessionStatePath(workdir))
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	if state.Sessions == nil {
		state.Sessions = map[string]codexSessionBinding{}
	}
	return state
}

func saveCodexSessionState(workdir string, state codexSessionState) error {
	path := codexSessionStatePath(workdir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func matchingWorkdir(a, b string) bool {
	ac, errA := filepath.Abs(filepath.Clean(a))
	bc, errB := filepath.Abs(filepath.Clean(b))
	if errA == nil {
		a = ac
	}
	if errB == nil {
		b = bc
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func latestCodexSessionID(workdir string, since time.Time) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	root := filepath.Join(home, ".codex", "sessions")
	type candidate struct {
		id      string
		modTime time.Time
	}
	var best candidate
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(since) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		firstLine, _, _ := strings.Cut(string(data), "\n")
		var meta struct {
			Type    string `json:"type"`
			Payload struct {
				SessionID string `json:"session_id"`
				ID        string `json:"id"`
				CWD       string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(firstLine), &meta) != nil || meta.Type != "session_meta" {
			return nil
		}
		if !matchingWorkdir(meta.Payload.CWD, workdir) {
			return nil
		}
		id := strings.TrimSpace(meta.Payload.SessionID)
		if id == "" {
			id = strings.TrimSpace(meta.Payload.ID)
		}
		if id == "" || (!best.modTime.IsZero() && !info.ModTime().After(best.modTime)) {
			return nil
		}
		best = candidate{id: id, modTime: info.ModTime()}
		return nil
	})
	return best.id
}

func shouldDiscoverCodexSessionID(resumed bool, knownSessionID string) bool {
	return !resumed || strings.TrimSpace(knownSessionID) == ""
}

func tempLastMessagePath(workdir string) (string, error) {
	dir := filepath.Join(workdir, "bin", ".dockpipe", "packages", "dorkpipe", "host-bridge", "codex-last-message")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "last-*.md")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	return path, nil
}

func hostChatPrompt(message, activeFile, selectionText string, openFiles []string) string {
	var parts []string
	parts = append(parts,
		"You are running as a direct Pipeon chat worker under a DorkPipe host bridge.",
		"Keep the answer concise and practical. Do not bypass host permissions.",
	)
	if active := strings.TrimSpace(activeFile); active != "" {
		parts = append(parts, "Active file: "+active)
	}
	if len(openFiles) > 0 {
		parts = append(parts, "Open files:\n- "+strings.Join(openFiles, "\n- "))
	}
	if selection := strings.TrimSpace(selectionText); selection != "" {
		parts = append(parts, "Selected text:\n"+selection)
	}
	parts = append(parts, "User request:\n"+strings.TrimSpace(message))
	return strings.Join(parts, "\n\n")
}

func codexModelArg(model string) (string, bool) {
	model = strings.TrimSpace(model)
	switch strings.ToLower(model) {
	case "", "config", "default", "auto", "cli-default", "account", "account-default", "gpt-5", "gpt-5-codex", "o4-mini":
		return "config", false
	default:
		return model, true
	}
}

func existingFile(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func existingDir(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return path, true
}

func codexCLIPathFromConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "CODEX_CLI_PATH") {
			continue
		}
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}

func resolveHostCodexPath() (string, error) {
	if path, ok := existingFile(os.Getenv("CODEX_CLI_PATH")); ok {
		return path, nil
	}
	if path, ok := existingFile(codexCLIPathFromConfig()); ok {
		return path, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		matches, _ := filepath.Glob(filepath.Join(home, ".vscode", "extensions", "openai.chatgpt-*", "bin", "windows-x86_64", "codex.exe"))
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		for _, match := range matches {
			if path, ok := existingFile(match); ok {
				return path, nil
			}
		}
	}
	return exec.LookPath("codex")
}

func resolveHostClaudePath() (string, error) {
	if path, ok := existingFile(os.Getenv("CLAUDE_CLI_PATH")); ok {
		return path, nil
	}
	return exec.LookPath("claude")
}

func hostConfigHome() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	if v := strings.TrimSpace(os.Getenv("USERPROFILE")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("HOME"))
}

func providerAuthStatusFor(provider string) *providerAuthStatus {
	provider = strings.ToLower(strings.TrimSpace(provider))
	status := &providerAuthStatus{
		Provider: provider,
		Metadata: map[string]any{
			"checked_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	switch provider {
	case "codex":
		status.LoginCommand = "codex login"
		if path, err := resolveHostCodexPath(); err == nil && strings.TrimSpace(path) != "" {
			status.Installed = true
			status.CLIPath = path
		} else {
			status.Issues = append(status.Issues, "Codex CLI is not installed or not discoverable by the host bridge.")
			status.NextActions = append(status.NextActions, "Install or expose the Codex CLI on the host PATH.")
		}
		home := hostConfigHome()
		if home != "" {
			if file, ok := existingFile(filepath.Join(home, ".codex", "auth.json")); ok {
				status.ConfigFile = file
				status.Authenticated = true
			}
		}
		if !status.Authenticated {
			status.Issues = append(status.Issues, "Codex host auth was not found.")
			status.NextActions = append(status.NextActions, "Run `codex login` on the host.")
		}
	case "claude":
		status.LoginCommand = "claude auth login"
		status.RepairTool = "dorkpipe.provider_auth_repair"
		if path, err := resolveHostClaudePath(); err == nil && strings.TrimSpace(path) != "" {
			status.Installed = true
			status.CLIPath = path
		}
		for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"} {
			if strings.TrimSpace(os.Getenv(key)) != "" {
				status.EnvKeys = append(status.EnvKeys, key)
			}
		}
		if len(status.EnvKeys) > 0 {
			status.Authenticated = true
		}
		var authDirs []string
		if v := strings.TrimSpace(os.Getenv("CLAUDE_HOME")); v != "" {
			authDirs = append(authDirs, v)
		}
		if home := hostConfigHome(); home != "" {
			authDirs = append(authDirs, filepath.Join(home, ".claude"))
			if file, ok := existingFile(filepath.Join(home, ".claude.json")); ok {
				status.ConfigFile = file
				status.Authenticated = true
			}
		}
		for _, dir := range authDirs {
			if found, ok := existingDir(dir); ok && status.AuthDir == "" {
				status.AuthDir = found
			}
			if _, ok := existingFile(filepath.Join(dir, ".credentials.json")); ok {
				status.AuthDir = dir
				status.Authenticated = true
			}
			matches, _ := filepath.Glob(filepath.Join(dir, "backups", ".claude.json.backup.*"))
			if len(matches) > 0 {
				status.AuthDir = dir
				status.Authenticated = true
			}
		}
		if !status.Authenticated {
			if !status.Installed {
				status.Issues = append(status.Issues, "Claude CLI is not installed or not discoverable by the host bridge.")
				status.NextActions = append(status.NextActions, "Install Claude Code or expose the Claude CLI on the host PATH.")
			}
			status.Issues = append(status.Issues, "Claude host auth was not found in environment, ~/.claude, or ~/.claude.json.")
			status.NextActions = append(status.NextActions, "Run `claude auth login` on the host, or provide ANTHROPIC_API_KEY/CLAUDE_API_KEY through a governed secret/env path.")
		}
	default:
		status.Issues = append(status.Issues, "Unsupported provider.")
	}
	return status
}

func providerAuthStatusText(status *providerAuthStatus) string {
	if status == nil {
		return "Provider auth status unavailable."
	}
	if status.Authenticated {
		return fmt.Sprintf("%s auth is available on the host.", providerDisplayName(status.Provider))
	}
	lines := []string{fmt.Sprintf("%s auth is not ready on the host.", providerDisplayName(status.Provider))}
	if len(status.Issues) > 0 {
		lines = append(lines, "", "Issues:")
		for _, issue := range status.Issues {
			lines = append(lines, "- "+issue)
		}
	}
	if len(status.NextActions) > 0 {
		lines = append(lines, "", "Next actions:")
		for _, action := range status.NextActions {
			lines = append(lines, "- "+action)
		}
	}
	return strings.Join(lines, "\n")
}

func providerDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		if provider == "" {
			return "Provider"
		}
		return provider
	}
}

func runHostCodexChat(ctx context.Context, workdir, message, model, pipeonSessionID, activeFile, selectionText string, openFiles []string) (*hostChatSummary, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("message required")
	}
	wd, err := resolveExecWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	codexPath, err := resolveHostCodexPath()
	if err != nil {
		return &hostChatSummary{
			Provider: "codex",
			Model:    strings.TrimSpace(model),
			Text:     "Codex CLI was not found by the DorkPipe host bridge.",
			Status:   "Provider: Codex | Codex CLI missing on host bridge",
			ExitCode: -1,
			Metadata: map[string]any{"route": "chat", "provider_preset": "codex", "validation_status": "missing_cli"},
		}, nil
	}
	chosenModel, passModel := codexModelArg(model)
	state := loadCodexSessionState(wd)
	pipeonSessionID = strings.TrimSpace(pipeonSessionID)
	binding, hasBinding := state.Sessions[pipeonSessionID]
	lastMessagePath, err := tempLastMessagePath(wd)
	if err != nil {
		return nil, err
	}
	defer os.Remove(lastMessagePath)
	prompt := hostChatPrompt(message, activeFile, selectionText, openFiles)
	args := []string{"exec"}
	resumed := false
	if pipeonSessionID != "" && hasBinding && strings.TrimSpace(binding.CodexSessionID) != "" {
		args = append(args, "resume")
		resumed = true
	} else {
		args = append(args, "-C", wd, "--sandbox", "workspace-write")
	}
	if passModel {
		args = append(args, "--model", chosenModel)
	}
	args = append(args, "--output-last-message", lastMessagePath)
	if resumed {
		args = append(args, binding.CodexSessionID)
	}
	args = append(args, prompt)
	commandStartedAt := time.Now()
	startedAt := commandStartedAt.Add(-2 * time.Second)
	stdout, stderr, code, err := runFixedHostCommand(ctx, wd, codexPath, args, 30*time.Minute)
	if err != nil {
		return nil, err
	}
	commandDurationMS := time.Since(commandStartedAt).Milliseconds()
	text := ""
	if data, readErr := os.ReadFile(lastMessagePath); readErr == nil {
		text = strings.TrimSpace(string(data))
	}
	if text == "" {
		text = strings.TrimSpace(stdout)
	}
	if text == "" && strings.TrimSpace(stderr) != "" {
		text = "Codex exited without stdout.\n\n```text\n" + strings.TrimSpace(stderr) + "\n```"
	}
	if text == "" {
		text = "(Codex returned no output.)"
	}
	codexSessionID := strings.TrimSpace(binding.CodexSessionID)
	sessionDiscoverySkipped := !shouldDiscoverCodexSessionID(resumed, codexSessionID)
	if !sessionDiscoverySkipped {
		if discovered := latestCodexSessionID(wd, startedAt); discovered != "" {
			codexSessionID = discovered
		}
	}
	if pipeonSessionID != "" && codexSessionID != "" {
		state.Sessions[pipeonSessionID] = codexSessionBinding{
			CodexSessionID: codexSessionID,
			Workdir:        wd,
			Model:          chosenModel,
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		}
		_ = saveCodexSessionState(wd, state)
	}
	status := fmt.Sprintf("Provider: Codex | Model: %s | Session: %s", chosenModel, map[bool]string{true: "resumed", false: "new"}[resumed])
	return &hostChatSummary{
		Provider:         "codex",
		Model:            chosenModel,
		Text:             text,
		Status:           status,
		ExitCode:         code,
		Stdout:           stdout,
		Stderr:           stderr,
		RequiresApproval: false,
		Metadata: map[string]any{
			"route":                     "chat",
			"provider_preset":           "codex",
			"model":                     chosenModel,
			"model_source":              map[bool]string{true: "explicit", false: "codex_config"}[passModel],
			"codex_path":                codexPath,
			"codex_session_id":          codexSessionID,
			"command_duration_ms":       commandDurationMS,
			"pipeon_session_id":         pipeonSessionID,
			"session_resumed":           resumed,
			"session_discovery_skipped": sessionDiscoverySkipped,
			"sandbox":                   "workspace-write",
			"approval_policy":           "exec_noninteractive",
			"validation_status":         "not_applicable",
			"exit_code":                 code,
		},
	}, nil
}

func runHostClaudeChat(ctx context.Context, workdir, message, model, pipeonSessionID, activeFile, selectionText string, openFiles []string) (*hostChatSummary, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("message required")
	}
	wd, err := resolveExecWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	auth := providerAuthStatusFor("claude")
	if !auth.Authenticated {
		metadata := map[string]any{
			"route":             "chat",
			"provider_preset":   "claude",
			"model":             strings.TrimSpace(model),
			"pipeon_session_id": pipeonSessionID,
			"validation_status": "auth_required",
			"exit_code":         -1,
			"auth_required":     true,
			"auth_provider":     "claude",
			"auth_action":       "provider_auth_repair",
			"auth":              auth,
		}
		return &hostChatSummary{
			Provider: "claude",
			Model:    strings.TrimSpace(model),
			Text:     providerAuthStatusText(auth),
			Status:   "Provider: Claude | Auth required on host",
			ExitCode: -1,
			Metadata: metadata,
		}, nil
	}
	prompt := hostChatPrompt(message, activeFile, selectionText, openFiles)
	chosenModel := strings.TrimSpace(model)
	claudeArgs := []string{"claude", "--dangerously-skip-permissions"}
	if chosenModel != "" {
		claudeArgs = append(claudeArgs, "--model", chosenModel)
	}
	claudeArgs = append(claudeArgs, "-p", prompt)
	args := claudeDockpipeArgs(wd)
	args = append(args, claudeArgs...)
	repoRoot := wd
	if rr, rrErr := effectiveRepoRoot(); rrErr == nil && strings.TrimSpace(rr) != "" {
		repoRoot = rr
	}
	commandStartedAt := time.Now()
	stdout, stderr, code, err := runDockpipeWithEnv(ctx, args, map[string]string{
		"DOCKPIPE_OP_INJECT": "0",
		"DOCKPIPE_REPO_ROOT": repoRoot,
	})
	if err != nil {
		return nil, err
	}
	commandDurationMS := time.Since(commandStartedAt).Milliseconds()
	text := cleanHostProviderStdout(stdout)
	if text == "" && strings.TrimSpace(stderr) != "" {
		text = "Claude workflow exited without stdout.\n\n```text\n" + strings.TrimSpace(stderr) + "\n```"
	}
	if text == "" {
		text = "Claude workflow returned no provider output. The DockPipe run only emitted infrastructure output."
	}
	if chosenModel == "" {
		chosenModel = "claude"
	}
	authRequired := claudeAuthRequired(stdout, stderr, text, code)
	if authRequired {
		text = "Claude needs authentication before this provider can answer.\n\nUse the **Authenticate Claude** action below to open a host terminal and run `claude auth login`, then retry Claude chat."
	}
	metadata := map[string]any{
		"route":               "chat",
		"provider_preset":     "claude",
		"model":               chosenModel,
		"pipeon_session_id":   pipeonSessionID,
		"session_context":     "pipeon_recent_history",
		"validation_status":   "dockpipe_workflow_boundary",
		"command_duration_ms": commandDurationMS,
		"exit_code":           code,
	}
	if authRequired {
		metadata["auth_required"] = true
		metadata["auth_provider"] = "claude"
		metadata["auth_action"] = "provider_auth_repair"
	}
	return &hostChatSummary{
		Provider: "claude",
		Model:    chosenModel,
		Text:     text,
		Status:   "Provider: Claude | Guard: DockPipe workflow boundary",
		ExitCode: code,
		Stdout:   stdout,
		Stderr:   stderr,
		Metadata: metadata,
	}, nil
}

func claudeDockpipeArgs(workdir string) []string {
	return []string{"--package", "agent", "--workflow", "claude", "--resolver", "claude", "--workdir", workdir, "--no-op-inject", "--"}
}

func claudeAuthRequired(stdout, stderr, text string, code int) bool {
	if code == 0 {
		return false
	}
	combined := strings.ToLower(stdout + "\n" + stderr + "\n" + text)
	authMarkers := []string{
		"not authenticated",
		"not logged in",
		"login required",
		"requires authentication",
		"authentication required",
		"please run claude login",
		"run `claude login`",
		"run claude login",
		"invalid api key",
		"anthropic_api_key",
		"claude_api_key",
	}
	for _, marker := range authMarkers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

func runHostClaudeAuth(ctx context.Context, workdir string) (*hostAuthSummary, error) {
	wd, err := resolveExecWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	before := providerAuthStatusFor("claude")
	command := claudeAuthCommand(wd)
	if !before.Installed {
		return &hostAuthSummary{
			Provider: "claude",
			Text:     providerAuthStatusText(before),
			Status:   "Claude auth repair blocked: CLI missing",
			Command:  command,
			Auth:     before,
			Metadata: map[string]any{"auth_provider": "claude", "auth_started": false, "auth_method": "host_cli_direct"},
		}, nil
	}
	switch runtime.GOOS {
	case "windows":
		script := "$ErrorActionPreference='Stop'; " +
			"Set-Location -LiteralPath " + psQuote(wd) + "; " +
			"& " + psQuote(before.CLIPath) + " auth login; " +
			"Write-Host ''; Write-Host 'Claude login finished. You can close this terminal.'"
		cmd := exec.CommandContext(ctx, "cmd.exe", "/C", "start", "DockPipe Claude Login", "powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-Command", script)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("launch Claude auth terminal: %w", err)
		}
		return &hostAuthSummary{
			Provider: "claude",
			Text:     "Opened a host terminal for Claude authentication. Complete `claude auth login` there, then retry Claude chat.",
			Status:   "Claude auth terminal opened",
			Command:  command,
			Auth:     before,
			Metadata: map[string]any{"auth_provider": "claude", "auth_started": true, "auth_method": "host_cli_direct"},
		}, nil
	case "darwin":
		script := "cd " + shQuote(wd) + " && " + shQuote(before.CLIPath) + " auth login; echo; read -r -p 'Claude login finished. Press enter to close.'"
		cmd := exec.CommandContext(ctx, "osascript", "-e", `tell application "Terminal" to do script `+strconvQuote(script))
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("launch Claude auth terminal: %w", err)
		}
		return &hostAuthSummary{
			Provider: "claude",
			Text:     "Opened a host terminal for Claude authentication. Complete `claude auth login` there, then retry Claude chat.",
			Status:   "Claude auth terminal opened",
			Command:  command,
			Auth:     before,
			Metadata: map[string]any{"auth_provider": "claude", "auth_started": true, "auth_method": "host_cli_direct"},
		}, nil
	default:
		terminal, _ := exec.LookPath("x-terminal-emulator")
		if terminal == "" {
			return &hostAuthSummary{
				Provider: "claude",
				Text:     "No host terminal launcher was found. Run the command below in a host terminal, then retry Claude chat.",
				Status:   "Claude auth command required",
				Command:  command,
				Auth:     before,
				Metadata: map[string]any{"auth_provider": "claude", "auth_started": false, "auth_method": "manual_command"},
			}, nil
		}
		script := "cd " + shQuote(wd) + " && " + shQuote(before.CLIPath) + " auth login; echo; read -r -p 'Claude login finished. Press enter to close.'"
		cmd := exec.CommandContext(ctx, terminal, "-e", "bash", "-lc", script)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("launch Claude auth terminal: %w", err)
		}
		return &hostAuthSummary{
			Provider: "claude",
			Text:     "Opened a host terminal for Claude authentication. Complete `claude auth login` there, then retry Claude chat.",
			Status:   "Claude auth terminal opened",
			Command:  command,
			Auth:     before,
			Metadata: map[string]any{"auth_provider": "claude", "auth_started": true, "auth_method": "host_cli_direct"},
		}, nil
	}
}

func claudeAuthCommand(workdir string) string {
	if runtime.GOOS == "windows" {
		return "Set-Location -LiteralPath " + psQuote(workdir) + "; claude auth login"
	}
	return "cd " + shQuote(workdir) + " && claude auth login"
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func strconvQuote(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func cleanHostProviderStdout(stdout string) string {
	lines := make([]string, 0, len(strings.Split(stdout, "\n")))
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || isDockerDigestOnlyLine(line) {
			continue
		}
		lines = append(lines, raw)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isDockerDigestOnlyLine(line string) bool {
	if !strings.HasPrefix(line, "sha256:") || len(line) != len("sha256:")+64 {
		return false
	}
	for _, r := range line[len("sha256:"):] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// absWorkdir ensures workdir is absolute; if relative, resolves from cwd.
func absWorkdir(wd string) (string, error) {
	wd = strings.TrimSpace(wd)
	if wd == "" {
		return os.Getwd()
	}
	mappedWorkdir, err := normalizeContainerWorkPath(wd)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(mappedWorkdir) {
		return filepath.Clean(mappedWorkdir), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(cwd, mappedWorkdir))
}

func mcpEnvOptOut(name string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return v == "0" || v == "false" || v == "no" || v == "off"
}

// restrictExecWorkdirToRepo is ON by default so dockpipe.run / dorkpipe.run_spec workdirs
// stay under the resolved repo root. Set DOCKPIPE_MCP_RESTRICT_WORKDIR=0 to disable.
func restrictExecWorkdirToRepo() bool {
	return !mcpEnvOptOut("DOCKPIPE_MCP_RESTRICT_WORKDIR")
}

// resolveExecWorkdir is used for MCP exec tools when restriction is on (default):
// empty workdir defaults to repo root; any other path must lie under that root.
func resolveExecWorkdir(inWorkdir string) (string, error) {
	if !restrictExecWorkdirToRepo() {
		return absWorkdir(inWorkdir)
	}
	rr, err := effectiveRepoRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Clean(rr)
	if strings.TrimSpace(inWorkdir) == "" {
		return root, nil
	}
	awd, err := absWorkdir(inWorkdir)
	if err != nil {
		return "", err
	}
	if err := CheckAbsolutePathUnderRepoRoot(awd); err != nil {
		return "", fmt.Errorf("workdir must stay under repo root (default; set DOCKPIPE_MCP_RESTRICT_WORKDIR=0 to allow any path): %w", err)
	}
	return awd, nil
}

// CheckMCPBinPathsAreAbsolute is ON by default: DOCKPIPE_BIN / DORKPIPE_BIN must be absolute.
// Set DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=0 to allow PATH lookup to a non-absolute name (weaker).
func CheckMCPBinPathsAreAbsolute() error {
	if mcpEnvOptOut("DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN") {
		return nil
	}
	dp := dockpipePath()
	if !filepath.IsAbs(dp) {
		return fmt.Errorf("DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=1: dockpipe path must be absolute (got %q); set DOCKPIPE_BIN=/path/to/dockpipe", dp)
	}
	dr := dorkpipePath()
	if !filepath.IsAbs(dr) {
		return fmt.Errorf("dorkpipe path must be absolute (default; set DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=0 to relax): got %q — set DORKPIPE_BIN=/path/to/dorkpipe", dr)
	}
	return nil
}

func toolResultJSON(text string, isError bool) []byte {
	type contentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type callResult struct {
		Content []contentBlock `json:"content"`
		IsError bool           `json:"isError"`
	}
	out, _ := json.Marshal(callResult{
		Content: []contentBlock{{Type: "text", Text: text}},
		IsError: isError,
	})
	return out
}

type dorkpipeExecSummary struct {
	ExitCode     int            `json:"exit_code"`
	FinalEvent   map[string]any `json:"final_event,omitempty"`
	ReadyToApply map[string]any `json:"ready_to_apply,omitempty"`
	StreamedText string         `json:"streamed_text,omitempty"`
	Events       []string       `json:"events,omitempty"`
	Stderr       []string       `json:"stderr,omitempty"`
}

func runDorkpipeEventStream(ctx context.Context, args []string) (*dorkpipeExecSummary, error) {
	stdout, stderr, exitCode, err := runDorkpipe(ctx, args)
	if err != nil {
		return nil, err
	}
	summary := &dorkpipeExecSummary{
		ExitCode: exitCode,
		Stderr:   compactNonEmptyLines(stderr),
	}
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			summary.Events = append(summary.Events, line)
			continue
		}
		if display, _ := event["display_text"].(string); strings.TrimSpace(display) != "" {
			summary.Events = append(summary.Events, strings.TrimSpace(display))
		}
		eventType, _ := event["type"].(string)
		switch strings.TrimSpace(eventType) {
		case "model_stream":
			if metadata, _ := event["metadata"].(map[string]any); metadata != nil {
				if piece, _ := metadata["text"].(string); piece != "" {
					summary.StreamedText += piece
				}
			}
		case "ready_to_apply":
			if metadata, _ := event["metadata"].(map[string]any); metadata != nil {
				summary.ReadyToApply = metadata
			}
		case "done", "error":
			summary.FinalEvent = event
		}
	}
	return summary, nil
}

func compactNonEmptyLines(text string) []string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func normalizeRepoHintPath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", nil
	}
	absPath, err := ResolvePathUnderRepoRoot(userPath)
	if err != nil {
		return "", err
	}
	repoRoot, err := effectiveRepoRoot()
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(filepath.Clean(repoRoot), filepath.Clean(absPath))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func normalizeRepoHintPaths(paths []string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range paths {
		rel, err := normalizeRepoHintPath(raw)
		if err != nil {
			return nil, err
		}
		if rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	return out, nil
}

func normalizeAttachmentPaths(paths []string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range paths {
		target := strings.TrimSpace(raw)
		if target == "" {
			continue
		}
		absPath, err := resolveRepoBoundAbsolutePath(target)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}
		out = append(out, absPath)
	}
	return out, nil
}

func resolveRepoBoundAbsolutePath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(userPath) {
		cleaned := filepath.Clean(userPath)
		if err := CheckAbsolutePathUnderRepoRoot(cleaned); err != nil {
			return "", err
		}
		return cleaned, nil
	}
	return ResolvePathUnderRepoRoot(userPath)
}
