package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dorkpipe.orchestrator/statepaths"
	"gopkg.in/yaml.v3"
)

type providerPoolCatalogDoc struct {
	Schema      int                         `yaml:"schema"`
	Description string                      `yaml:"description"`
	Defaults    providerPoolCatalogDefaults `yaml:"defaults"`
	Providers   []providerPoolProvider      `yaml:"providers"`
}

type providerPoolCatalogDefaults struct {
	Provider string `yaml:"provider"`
	Mode     string `yaml:"mode"`
}

type providerPoolProvider struct {
	ID           string                    `yaml:"id"`
	DisplayName  string                    `yaml:"display_name"`
	Description  string                    `yaml:"description"`
	EnabledEnv   []string                  `yaml:"enabled_env"`
	DefaultModel string                    `yaml:"default_model"`
	ModelEnv     []string                  `yaml:"model_env"`
	Models       []string                  `yaml:"models"`
	Pool         providerPoolProviderShape `yaml:"pool"`
}

type providerPoolProviderShape struct {
	MinReady        int    `yaml:"min_ready" json:"min_ready"`
	MaxActive       int    `yaml:"max_active" json:"max_active"`
	IdleTTLSeconds  int    `yaml:"idle_ttl_seconds" json:"idle_ttl_seconds"`
	Role            string `yaml:"role" json:"role"`
	SessionAffinity bool   `yaml:"session_affinity" json:"session_affinity"`
	WarmMode        string `yaml:"warm_mode" json:"warm_mode"`
	RequiresAuth    bool   `yaml:"requires_auth" json:"requires_auth"`
	WarmSource      string `yaml:"warm_source" json:"warm_source"`
}

type providerPoolCatalogResponse struct {
	ContractVersion string             `json:"contract_version"`
	DefaultProvider string             `json:"default_provider"`
	Providers       []providerPoolView `json:"providers"`
	GeneratedAt     string             `json:"generated_at"`
}

type providerPoolView struct {
	ID           string                    `json:"id"`
	DisplayName  string                    `json:"display_name"`
	Description  string                    `json:"description"`
	DefaultModel string                    `json:"default_model"`
	Models       []string                  `json:"models"`
	Pool         providerPoolProviderShape `json:"pool"`
	Status       providerPoolStatus        `json:"status"`
}

type providerPoolStatus struct {
	Provider        string         `json:"provider"`
	DisplayName     string         `json:"display_name,omitempty"`
	State           string         `json:"state"`
	Status          string         `json:"status"`
	Enabled         bool           `json:"enabled"`
	ReadyWorkers    int            `json:"ready_workers"`
	ActiveWorkers   int            `json:"active_workers"`
	MaxActive       int            `json:"max_active"`
	MinReady        int            `json:"min_ready"`
	SessionAffinity bool           `json:"session_affinity"`
	DefaultModel    string         `json:"default_model,omitempty"`
	SelectedModel   string         `json:"selected_model,omitempty"`
	DisableReason   string         `json:"disable_reason,omitempty"`
	NextAction      string         `json:"next_action,omitempty"`
	WorkerID        string         `json:"worker_id,omitempty"`
	BoundSessionID  string         `json:"bound_session_id,omitempty"`
	Auth            map[string]any `json:"auth,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type providerPoolPromptResponse struct {
	Provider     string         `json:"provider"`
	Model        string         `json:"model,omitempty"`
	State        string         `json:"state"`
	Status       string         `json:"status"`
	Text         string         `json:"text"`
	ExitCode     int            `json:"exit_code"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Stdout       string         `json:"stdout,omitempty"`
	Stderr       string         `json:"stderr,omitempty"`
	ReadyToApply map[string]any `json:"ready_to_apply,omitempty"`
}

type providerPoolLease struct {
	Provider  string `json:"provider"`
	SessionID string `json:"session_id,omitempty"`
	Workdir   string `json:"workdir"`
	StartedAt string `json:"started_at"`
}

type providerPoolPromptOptions struct {
	Workdir       string
	Provider      string
	Model         string
	Prompt        string
	SessionID     string
	ActiveFile    string
	OpenFiles     []string
	SelectionText string
}

func providerPoolCmd(argv []string) {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe provider-pool <catalog|status|prompt> [flags]")
		os.Exit(2)
	}
	switch argv[0] {
	case "catalog":
		providerPoolCatalogCmd(argv[1:])
	case "status":
		providerPoolStatusCmd(argv[1:])
	case "prompt":
		providerPoolPromptCmd(argv[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown provider-pool subcommand %q\n", argv[0])
		fmt.Fprintln(os.Stderr, "usage: dorkpipe provider-pool <catalog|status|prompt> [flags]")
		os.Exit(2)
	}
}

func providerPoolCatalogCmd(argv []string) {
	fs := flag.NewFlagSet("provider-pool catalog", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	payload, err := buildProviderPoolCatalogResponse(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return
	}
	fmt.Printf("default provider: %s\n", payload.DefaultProvider)
	for _, item := range payload.Providers {
		fmt.Printf("- %s (%s): %s | default model=%s\n", item.ID, item.DisplayName, item.Status.State, item.DefaultModel)
	}
}

func providerPoolStatusCmd(argv []string) {
	fs := flag.NewFlagSet("provider-pool status", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	provider := fs.String("provider", "", "provider id filter")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	payload, err := buildProviderPoolCatalogResponse(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if strings.TrimSpace(*provider) != "" {
		filtered := make([]providerPoolView, 0, 1)
		for _, item := range payload.Providers {
			if item.ID == strings.ToLower(strings.TrimSpace(*provider)) {
				filtered = append(filtered, item)
			}
		}
		payload.Providers = filtered
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return
	}
	for _, item := range payload.Providers {
		fmt.Printf("%s: %s\n", item.ID, item.Status.Status)
	}
}

func providerPoolPromptCmd(argv []string) {
	argv = providerPoolPromptArgs(argv)
	fs := flag.NewFlagSet("provider-pool prompt", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	provider := fs.String("provider", "", "provider override")
	model := fs.String("model", "", "model override")
	prompt := fs.String("prompt", "", "prompt text")
	sessionID := fs.String("session-id", "", "direct-session id")
	activeFile := fs.String("active-file", "", "repo-relative active file hint")
	var openFiles stringListFlag
	fs.Var(&openFiles, "open-file", "repo-relative open file hint (repeatable)")
	selectionText := fs.String("selection-text", "", "selection hint")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	if strings.TrimSpace(*prompt) == "" {
		fmt.Fprintln(os.Stderr, "provider-pool prompt: --prompt is required")
		os.Exit(2)
	}
	wd := mustWorkdir(*workdir)
	result, err := runProviderPoolPrompt(context.Background(), providerPoolPromptOptions{
		Workdir:       wd,
		Provider:      strings.TrimSpace(*provider),
		Model:         strings.TrimSpace(*model),
		Prompt:        strings.TrimSpace(*prompt),
		SessionID:     strings.TrimSpace(*sessionID),
		ActiveFile:    strings.TrimSpace(*activeFile),
		OpenFiles:     uniqueNonEmpty(openFiles),
		SelectionText: strings.TrimSpace(*selectionText),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}
	fmt.Fprintln(os.Stderr, result.Status)
	if strings.TrimSpace(result.Text) != "" {
		fmt.Println(result.Text)
	}
	if result.State == "ready" {
		return
	}
	os.Exit(3)
}

func providerPoolPromptArgs(argv []string) []string {
	out := append([]string(nil), argv...)
	raw := strings.TrimSpace(os.Getenv("DOCKPIPE_ARGS_JSON"))
	if raw == "" {
		return out
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return out
	}
	return append(out, parsed...)
}

func buildProviderPoolCatalogResponse(workdir string) (*providerPoolCatalogResponse, error) {
	doc, err := loadProviderPoolCatalog(workdir)
	if err != nil {
		return nil, err
	}
	resp := &providerPoolCatalogResponse{
		ContractVersion: "v1",
		DefaultProvider: providerPoolDefaultProvider(doc),
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	for _, item := range doc.Providers {
		defaultModel := providerPoolChosenModel(item)
		status, err := providerPoolStatusFor(workdir, item, defaultModel, "")
		if err != nil {
			return nil, err
		}
		resp.Providers = append(resp.Providers, providerPoolView{
			ID:           item.ID,
			DisplayName:  item.DisplayName,
			Description:  item.Description,
			DefaultModel: defaultModel,
			Models:       append([]string(nil), item.Models...),
			Pool:         item.Pool,
			Status:       *status,
		})
	}
	sort.Slice(resp.Providers, func(i, j int) bool { return resp.Providers[i].ID < resp.Providers[j].ID })
	return resp, nil
}

func runProviderPoolPrompt(ctx context.Context, opts providerPoolPromptOptions) (*providerPoolPromptResponse, error) {
	doc, err := loadProviderPoolCatalog(opts.Workdir)
	if err != nil {
		return nil, err
	}
	providerID := strings.ToLower(strings.TrimSpace(opts.Provider))
	if providerID == "" {
		providerID = providerPoolDefaultProvider(doc)
	}
	provider, ok := providerPoolFindProvider(doc, providerID)
	if !ok {
		return nil, fmt.Errorf("unknown provider-pool provider %q", providerID)
	}
	chosenModel := strings.TrimSpace(opts.Model)
	if chosenModel == "" {
		chosenModel = providerPoolChosenModel(provider)
	}
	status, err := providerPoolStatusFor(opts.Workdir, provider, chosenModel, opts.SessionID)
	if err != nil {
		return nil, err
	}
	if status.State != "ready" {
		return &providerPoolPromptResponse{
			Provider: provider.ID,
			Model:    chosenModel,
			State:    status.State,
			Status:   status.Status,
			Text:     providerPoolUnavailableText(status),
			ExitCode: 0,
			Metadata: status.Metadata,
		}, nil
	}
	releaseLease, queued, err := acquireProviderPoolLease(opts.Workdir, provider, opts.SessionID)
	if err != nil {
		return nil, err
	}
	if queued {
		queueStatus := *status
		queueStatus.State = "queued"
		queueStatus.Status = fmt.Sprintf("Provider: %s | State: queued | Active workers reached %d/%d", provider.DisplayName, queueStatus.ActiveWorkers, queueStatus.MaxActive)
		queueStatus.Metadata = ensureMetadata(queueStatus.Metadata)
		queueStatus.Metadata["queue_reason"] = "max_active_reached"
		return &providerPoolPromptResponse{
			Provider: provider.ID,
			Model:    chosenModel,
			State:    "queued",
			Status:   queueStatus.Status,
			Text:     providerPoolUnavailableText(&queueStatus),
			ExitCode: 0,
			Metadata: queueStatus.Metadata,
		}, nil
	}
	if releaseLease != nil {
		defer releaseLease()
	}
	switch provider.ID {
	case "ollama":
		return runProviderPoolOllamaPrompt(ctx, opts, chosenModel)
	case "codex":
		return runProviderPoolCodexPrompt(ctx, opts, chosenModel)
	case "claude":
		return runProviderPoolClaudePrompt(ctx, provider, opts, chosenModel)
	default:
		return nil, fmt.Errorf("provider-pool prompt not implemented for %q", provider.ID)
	}
}

func providerPoolUnavailableText(status *providerPoolStatus) string {
	if status == nil {
		return "Provider-pool status unavailable."
	}
	switch status.State {
	case "auth-required":
		return fmt.Sprintf("%s needs authentication before direct orchestration can run.\n\n%s", status.DisplayName, firstNonEmptyString(status.NextAction, "Complete the provider auth flow and retry."))
	case "warming":
		return fmt.Sprintf("%s is warming. Retry once the warm worker or service becomes ready.", status.DisplayName)
	case "queued":
		return fmt.Sprintf("%s is busy and the pool is queueing new work because max_active was reached.", status.DisplayName)
	case "disabled":
		return fmt.Sprintf("%s is disabled.\n\n%s", status.DisplayName, firstNonEmptyString(status.DisableReason, "Enable the provider pool and retry."))
	case "failed":
		return fmt.Sprintf("%s failed its pool readiness check.\n\n%s", status.DisplayName, firstNonEmptyString(status.DisableReason, "Inspect the provider pool status and retry."))
	default:
		return status.Status
	}
}

func providerPoolStatusFor(workdir string, provider providerPoolProvider, chosenModel, sessionID string) (*providerPoolStatus, error) {
	status := &providerPoolStatus{
		Provider:        provider.ID,
		DisplayName:     provider.DisplayName,
		State:           "disabled",
		Enabled:         providerPoolEnabled(provider),
		MaxActive:       max(1, provider.Pool.MaxActive),
		MinReady:        max(0, provider.Pool.MinReady),
		SessionAffinity: provider.Pool.SessionAffinity,
		DefaultModel:    providerPoolChosenModel(provider),
		SelectedModel:   chosenModel,
		Metadata: map[string]any{
			"provider_preset": provider.ID,
			"warm_mode":       provider.Pool.WarmMode,
			"warm_source":     provider.Pool.WarmSource,
		},
	}
	if !status.Enabled {
		status.DisableReason = fmt.Sprintf("disabled by %s", strings.Join(provider.EnabledEnv, ", "))
		status.Status = fmt.Sprintf("Provider: %s | State: disabled | %s", provider.DisplayName, status.DisableReason)
		return status, nil
	}
	activeWorkers, _ := countProviderPoolLeases(workdir, provider.ID)
	status.ActiveWorkers = activeWorkers
	switch provider.ID {
	case "ollama":
		host := providerPoolOllamaHost()
		status.Metadata["ollama_host"] = host
		if providerPoolOllamaReady(host) {
			status.State = "ready"
			status.ReadyWorkers = 1
			status.Status = fmt.Sprintf("Provider: %s | State: ready | Warm service at %s", provider.DisplayName, host)
			return status, nil
		}
		status.State = "warming"
		status.Status = fmt.Sprintf("Provider: %s | State: warming | Waiting for Ollama at %s", provider.DisplayName, host)
		status.NextAction = "Start or reuse the DorkPipe/Pipeon Ollama service, then retry."
		return status, nil
	case "codex":
		auth := providerPoolCodexAuthStatus()
		status.Auth = auth
		if !boolFromMap(auth, "installed") {
			status.State = "disabled"
			status.DisableReason = "Codex CLI is not installed or not discoverable on the host."
			status.Status = fmt.Sprintf("Provider: %s | State: disabled | %s", provider.DisplayName, status.DisableReason)
			return status, nil
		}
		if !boolFromMap(auth, "authenticated") {
			status.State = "auth-required"
			status.Status = fmt.Sprintf("Provider: %s | State: auth-required | Run codex login", provider.DisplayName)
			status.NextAction = "Run `codex login` on the host and retry."
			return status, nil
		}
		status.State = "ready"
		status.ReadyWorkers = 1
		status.Status = fmt.Sprintf("Provider: %s | State: ready | Host exec resume lane available", provider.DisplayName)
		status.WorkerID = providerPoolCodexWorkerID(workdir)
		status.BoundSessionID = strings.TrimSpace(sessionID)
		return status, nil
	case "claude":
		auth := providerPoolClaudeAuthStatus()
		status.Auth = auth
		if !boolFromMap(auth, "authenticated") {
			status.State = "auth-required"
			status.Status = fmt.Sprintf("Provider: %s | State: auth-required | Run claude auth login", provider.DisplayName)
			status.NextAction = "Run `claude auth login` on the host, or provide a governed Anthropic API key path."
			return status, nil
		}
		dockerPath, err := exec.LookPath("docker")
		if err != nil || strings.TrimSpace(dockerPath) == "" {
			status.State = "disabled"
			status.DisableReason = "docker is required for the guarded Claude warm worker."
			status.Status = fmt.Sprintf("Provider: %s | State: disabled | %s", provider.DisplayName, status.DisableReason)
			return status, nil
		}
		containerName := providerPoolClaudeContainerName(workdir)
		status.Metadata["container_name"] = containerName
		if !providerPoolClaudeImageReady() {
			status.State = "disabled"
			status.DisableReason = "resolver image dockpipe-claude:latest is missing."
			status.NextAction = "Build or materialize the Claude resolver image, then retry."
			status.Status = fmt.Sprintf("Provider: %s | State: disabled | %s", provider.DisplayName, status.DisableReason)
			return status, nil
		}
		running, _ := providerPoolClaudeContainerRunning(containerName)
		if running {
			status.State = "ready"
			status.ReadyWorkers = 1
			status.Status = fmt.Sprintf("Provider: %s | State: ready | Warm guarded worker %s", provider.DisplayName, containerName)
			status.WorkerID = containerName
			status.BoundSessionID = strings.TrimSpace(sessionID)
			return status, nil
		}
		if started, startErr := providerPoolEnsureClaudeWarmContainer(context.Background(), workdir, containerName); startErr != nil {
			status.State = "failed"
			status.DisableReason = startErr.Error()
			status.Status = fmt.Sprintf("Provider: %s | State: failed | %s", provider.DisplayName, status.DisableReason)
			return status, nil
		} else if started {
			status.State = "warming"
			status.Status = fmt.Sprintf("Provider: %s | State: warming | Started guarded worker %s", provider.DisplayName, containerName)
			status.WorkerID = containerName
			status.BoundSessionID = strings.TrimSpace(sessionID)
			status.NextAction = "Retry once the guarded container worker is ready."
			return status, nil
		}
		status.State = "warming"
		status.Status = fmt.Sprintf("Provider: %s | State: warming | Waiting for guarded worker %s", provider.DisplayName, containerName)
		return status, nil
	default:
		return nil, fmt.Errorf("unsupported provider-pool provider %q", provider.ID)
	}
}

func runProviderPoolOllamaPrompt(ctx context.Context, opts providerPoolPromptOptions, chosenModel string) (*providerPoolPromptResponse, error) {
	args := []string{"request", "--execute", "--workdir", opts.Workdir, "--mode", "ask", "--provider-preset", "ollama-stack", "--model-provider", "ollama", "--model", chosenModel, "--message", augmentDirectPrompt(opts.Prompt, opts.ActiveFile, opts.SelectionText, opts.OpenFiles)}
	summary, err := runSelfEventStream(ctx, args)
	if err != nil {
		return nil, err
	}
	status := fmt.Sprintf("Provider: Ollama local | State: ready | Model: %s", chosenModel)
	text := firstNonEmptyString(summary.UserMessage, summary.StreamedText, "(No response text returned.)")
	state := "ready"
	if summary.FinalType == "error" {
		state = "failed"
		status = fmt.Sprintf("Provider: Ollama local | State: failed | %s", firstNonEmptyString(summary.ErrorMessage, "DorkPipe request failed"))
	}
	return &providerPoolPromptResponse{
		Provider:     "ollama",
		Model:        chosenModel,
		State:        state,
		Status:       status,
		Text:         text,
		ExitCode:     summary.ExitCode,
		Stdout:       summary.Stdout,
		Stderr:       summary.Stderr,
		Metadata:     summary.Metadata,
		ReadyToApply: summary.ReadyToApply,
	}, nil
}

func runProviderPoolCodexPrompt(ctx context.Context, opts providerPoolPromptOptions, chosenModel string) (*providerPoolPromptResponse, error) {
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return &providerPoolPromptResponse{
			Provider: "codex",
			Model:    chosenModel,
			State:    "disabled",
			Status:   "Provider: Codex host | State: disabled | Codex CLI missing",
			Text:     "Codex CLI is not installed or not discoverable on the host.",
			ExitCode: -1,
		}, nil
	}
	args := []string{"exec", "-C", opts.Workdir, "--sandbox", "workspace-write"}
	modelArg := strings.TrimSpace(chosenModel)
	if modelArg != "" && !strings.EqualFold(modelArg, "config") {
		args = append(args, "--model", modelArg)
	}
	codexSession := strings.TrimSpace(opts.SessionID)
	if codexSession != "" {
		if binding := loadProviderPoolCodexBindings(opts.Workdir); binding[codexSession] != "" {
			args = []string{"exec", "resume", binding[codexSession]}
			if modelArg != "" && !strings.EqualFold(modelArg, "config") {
				args = append(args, "--model", modelArg)
			}
		}
	}
	lastMessagePath, err := os.CreateTemp("", "dorkpipe-codex-last-message-*.txt")
	if err != nil {
		return nil, err
	}
	lastPath := lastMessagePath.Name()
	_ = lastMessagePath.Close()
	defer os.Remove(lastPath)
	args = append(args, "--output-last-message", lastPath, augmentDirectPrompt(opts.Prompt, opts.ActiveFile, opts.SelectionText, opts.OpenFiles))
	startedAt := time.Now()
	stdout, stderr, code, runErr := runCommandCapture(ctx, opts.Workdir, codexPath, args...)
	if runErr != nil {
		return nil, runErr
	}
	text := strings.TrimSpace(stdout)
	if data, err := os.ReadFile(lastPath); err == nil && strings.TrimSpace(string(data)) != "" {
		text = strings.TrimSpace(string(data))
	}
	if text == "" && strings.TrimSpace(stderr) != "" {
		text = strings.TrimSpace(stderr)
	}
	if text == "" {
		text = "(Codex returned no output.)"
	}
	if codexSession != "" {
		if discovered := latestCodexSessionID(opts.Workdir, startedAt.Add(-2*time.Second)); discovered != "" {
			binding := loadProviderPoolCodexBindings(opts.Workdir)
			binding[codexSession] = discovered
			_ = saveProviderPoolCodexBindings(opts.Workdir, binding)
		}
	}
	state := "ready"
	status := fmt.Sprintf("Provider: Codex host | State: ready | Model: %s", firstNonEmptyString(chosenModel, "config"))
	if code != 0 {
		state = "failed"
		status = fmt.Sprintf("Provider: Codex host | State: failed | Exit %d", code)
	}
	return &providerPoolPromptResponse{
		Provider: "codex",
		Model:    chosenModel,
		State:    state,
		Status:   status,
		Text:     text,
		ExitCode: code,
		Stdout:   stdout,
		Stderr:   stderr,
		Metadata: map[string]any{
			"provider_preset": "codex",
			"sandbox":         "workspace-write",
			"model_source":    map[bool]string{true: "explicit", false: "codex_config"}[!strings.EqualFold(chosenModel, "config") && chosenModel != ""],
			"session_id":      strings.TrimSpace(opts.SessionID),
		},
	}, nil
}

func runProviderPoolClaudePrompt(ctx context.Context, provider providerPoolProvider, opts providerPoolPromptOptions, chosenModel string) (*providerPoolPromptResponse, error) {
	containerName := providerPoolClaudeContainerName(opts.Workdir)
	running, err := providerPoolClaudeContainerRunning(containerName)
	if err != nil {
		return nil, err
	}
	if !running {
		return &providerPoolPromptResponse{
			Provider: "claude",
			Model:    chosenModel,
			State:    "warming",
			Status:   fmt.Sprintf("Provider: %s | State: warming | Guarded worker %s is not ready yet", provider.DisplayName, containerName),
			Text:     "Claude guarded worker is warming. Retry once the pooled container is ready.",
			ExitCode: 0,
			Metadata: map[string]any{
				"provider_preset": "claude",
				"container_name":  containerName,
			},
		}, nil
	}
	args := []string{"exec", "-i", "-w", "/work", containerName, "claude", "--dangerously-skip-permissions"}
	if strings.TrimSpace(chosenModel) != "" {
		args = append(args, "--model", chosenModel)
	}
	args = append(args, "-p", augmentDirectPrompt(opts.Prompt, opts.ActiveFile, opts.SelectionText, opts.OpenFiles))
	stdout, stderr, code, runErr := runCommandCapture(ctx, opts.Workdir, "docker", args...)
	if runErr != nil {
		return nil, runErr
	}
	text := cleanDirectProviderText(stdout)
	if text == "" && strings.TrimSpace(stderr) != "" {
		text = strings.TrimSpace(stderr)
	}
	if text == "" {
		text = "(Claude returned no output.)"
	}
	state := "ready"
	status := fmt.Sprintf("Provider: %s | State: ready | Warm guarded worker %s", provider.DisplayName, containerName)
	if code != 0 {
		state = "failed"
		status = fmt.Sprintf("Provider: %s | State: failed | Exit %d", provider.DisplayName, code)
	}
	return &providerPoolPromptResponse{
		Provider: "claude",
		Model:    chosenModel,
		State:    state,
		Status:   status,
		Text:     text,
		ExitCode: code,
		Stdout:   stdout,
		Stderr:   stderr,
		Metadata: map[string]any{
			"provider_preset": "claude",
			"container_name":  containerName,
			"session_id":      strings.TrimSpace(opts.SessionID),
		},
	}, nil
}

type selfEventStreamSummary struct {
	Stdout       string
	Stderr       string
	ExitCode     int
	FinalType    string
	UserMessage  string
	ErrorMessage string
	StreamedText string
	Metadata     map[string]any
	ReadyToApply map[string]any
}

func runSelfEventStream(ctx context.Context, args []string) (*selfEventStreamSummary, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	stdout, stderr, exitCode, err := runCommandCapture(ctx, "", exe, args...)
	if err != nil {
		return nil, err
	}
	summary := &selfEventStreamSummary{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if t, _ := event["type"].(string); t != "" {
			summary.FinalType = t
		}
		if msg, _ := event["user_message"].(string); strings.TrimSpace(msg) != "" {
			summary.UserMessage = strings.TrimSpace(msg)
		}
		if meta, ok := event["metadata"].(map[string]any); ok {
			summary.Metadata = meta
		}
		if rta, ok := event["ready_to_apply"].(map[string]any); ok {
			summary.ReadyToApply = rta
		}
		if errObj, ok := event["error"].(map[string]any); ok {
			if msg, _ := errObj["user_message"].(string); strings.TrimSpace(msg) != "" {
				summary.ErrorMessage = strings.TrimSpace(msg)
			}
		}
	}
	if summary.UserMessage == "" {
		summary.StreamedText = strings.TrimSpace(stdout)
	}
	return summary, nil
}

func loadProviderPoolCatalog(workdir string) (*providerPoolCatalogDoc, error) {
	catalogPath, err := resolveProviderPoolCatalogPath(workdir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, err
	}
	var doc providerPoolCatalogDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("provider-pool catalog yaml: %w", err)
	}
	return &doc, nil
}

func resolveProviderPoolCatalogPath(workdir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("DORKPIPE_PROVIDER_POOL_CATALOG")); v != "" {
		return v, nil
	}
	if assetsDir := strings.TrimSpace(os.Getenv("DOCKPIPE_ASSETS_DIR")); assetsDir != "" {
		path := filepath.Join(assetsDir, "provider-pools", "catalog.yml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	wd := workdir
	if wd == "" {
		wd, _ = os.Getwd()
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "packages", "dorkpipe", "resolvers", "dorkpipe", "assets", "provider-pools", "catalog.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", errors.New("provider-pool catalog not found")
}

func providerPoolDefaultProvider(doc *providerPoolCatalogDoc) string {
	if doc == nil || strings.TrimSpace(doc.Defaults.Provider) == "" {
		return "ollama"
	}
	return strings.ToLower(strings.TrimSpace(doc.Defaults.Provider))
}

func providerPoolFindProvider(doc *providerPoolCatalogDoc, providerID string) (providerPoolProvider, bool) {
	for _, item := range doc.Providers {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(providerID)) {
			return item, true
		}
	}
	return providerPoolProvider{}, false
}

func providerPoolChosenModel(provider providerPoolProvider) string {
	for _, key := range provider.ModelEnv {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if strings.TrimSpace(provider.DefaultModel) != "" {
		return strings.TrimSpace(provider.DefaultModel)
	}
	if len(provider.Models) > 0 {
		return provider.Models[0]
	}
	return ""
}

func providerPoolEnabled(provider providerPoolProvider) bool {
	for _, key := range provider.EnabledEnv {
		if value, ok := os.LookupEnv(key); ok {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "0", "false", "no", "off", "disabled":
				return false
			case "1", "true", "yes", "on", "enabled":
				return true
			}
		}
	}
	return true
}

func providerPoolOllamaHost() string {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	return strings.TrimRight(host, "/")
}

func providerPoolOllamaReady(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func providerPoolCodexAuthStatus() map[string]any {
	status := map[string]any{
		"provider":      "codex",
		"installed":     false,
		"authenticated": false,
	}
	if path, err := exec.LookPath("codex"); err == nil && strings.TrimSpace(path) != "" {
		status["installed"] = true
		status["cli_path"] = path
	}
	for _, home := range providerPoolHomeDirs() {
		authPath := filepath.Join(home, ".codex", "auth.json")
		if _, err := os.Stat(authPath); err == nil {
			status["authenticated"] = true
			status["auth_file"] = authPath
			break
		}
	}
	return status
}

func providerPoolClaudeAuthStatus() map[string]any {
	status := map[string]any{
		"provider":      "claude",
		"installed":     false,
		"authenticated": false,
	}
	if path, err := exec.LookPath("claude"); err == nil && strings.TrimSpace(path) != "" {
		status["installed"] = true
		status["cli_path"] = path
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			status["authenticated"] = true
			status["env_key"] = key
			status["env_present"] = len(value) > 0
			return status
		}
	}
	for _, home := range providerPoolHomeDirs() {
		configPath := filepath.Join(home, ".claude.json")
		if _, err := os.Stat(configPath); err == nil {
			status["authenticated"] = true
			status["config_file"] = configPath
		}
		authDir := filepath.Join(home, ".claude")
		if _, err := os.Stat(filepath.Join(authDir, ".credentials.json")); err == nil {
			status["authenticated"] = true
			status["auth_dir"] = authDir
			return status
		}
	}
	return status
}

func providerPoolHomeDirs() []string {
	var homes []string
	for _, key := range []string{"CLAUDE_HOME", "HOME", "USERPROFILE"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			homes = append(homes, value)
		}
	}
	return uniqueNonEmpty(homes)
}

func providerPoolCodexWorkerID(workdir string) string {
	return "codex-host-" + providerPoolWorkdirHash(workdir)
}

func providerPoolClaudeContainerName(workdir string) string {
	return "dorkpipe-provider-pool-claude-" + providerPoolWorkdirHash(workdir)
}

func providerPoolWorkdirHash(workdir string) string {
	sum := sha1.Sum([]byte(filepath.Clean(workdir)))
	return hex.EncodeToString(sum[:])[:10]
}

func providerPoolClaudeImageReady() bool {
	_, _, code, err := runCommandCapture(context.Background(), "", "docker", "image", "inspect", "dockpipe-claude:latest")
	return err == nil && code == 0
}

func providerPoolClaudeContainerRunning(containerName string) (bool, error) {
	stdout, _, code, err := runCommandCapture(context.Background(), "", "docker", "inspect", "-f", "{{.State.Running}}", containerName)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, nil
	}
	return strings.EqualFold(strings.TrimSpace(stdout), "true"), nil
}

func providerPoolEnsureClaudeWarmContainer(ctx context.Context, workdir, containerName string) (bool, error) {
	running, err := providerPoolClaudeContainerRunning(containerName)
	if err != nil {
		return false, err
	}
	if running {
		return false, nil
	}
	_, _, _, _ = runCommandCapture(ctx, "", "docker", "rm", "-f", containerName)
	args := []string{"run", "-d", "--name", containerName, "-w", "/work", "--mount", fmt.Sprintf("type=bind,src=%s,dst=/work", workdir)}
	if path, ok := stringFromMap(providerPoolClaudeAuthStatus(), "auth_dir"); ok {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/home/node/.claude", path))
	}
	if path, ok := stringFromMap(providerPoolClaudeAuthStatus(), "config_file"); ok {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/home/node/.claude.json", path))
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
		}
	}
	args = append(args, "dockpipe-claude:latest", "bash", "-lc", "trap 'exit 0' TERM INT; while :; do sleep 3600; done")
	_, stderr, code, err := runCommandCapture(ctx, workdir, "docker", args...)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("start guarded Claude worker: %s", strings.TrimSpace(stderr))
	}
	return true, nil
}

func acquireProviderPoolLease(workdir string, provider providerPoolProvider, sessionID string) (func(), bool, error) {
	leasesDir, err := statepaths.ProviderPoolLeasesDir(workdir)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(leasesDir, 0o755); err != nil {
		return nil, false, err
	}
	leasePath := filepath.Join(leasesDir, provider.ID+".json")
	file, err := os.OpenFile(leasePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, true, nil
		}
		return nil, false, err
	}
	payload := providerPoolLease{
		Provider:  provider.ID,
		SessionID: strings.TrimSpace(sessionID),
		Workdir:   workdir,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(leasePath)
		return nil, false, err
	}
	_ = file.Close()
	return func() {
		_ = os.Remove(leasePath)
	}, false, nil
}

func countProviderPoolLeases(workdir, providerID string) (int, error) {
	leasesDir, err := statepaths.ProviderPoolLeasesDir(workdir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(leasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), providerID) {
			count++
		}
	}
	return count, nil
}

func loadProviderPoolCodexBindings(workdir string) map[string]string {
	path, err := statepaths.ProviderPoolSessionsPath(workdir)
	if err != nil {
		return map[string]string{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var payload map[string]string
	if json.Unmarshal(raw, &payload) != nil {
		return map[string]string{}
	}
	return payload
}

func saveProviderPoolCodexBindings(workdir string, payload map[string]string) error {
	path, err := statepaths.ProviderPoolSessionsPath(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func latestCodexSessionID(workdir string, since time.Time) string {
	home := ""
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			home = value
			break
		}
	}
	if home == "" {
		return ""
	}
	historyDir := filepath.Join(home, ".codex", "sessions")
	var newest string
	var newestTime time.Time
	_ = filepath.Walk(historyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.EqualFold(info.Name(), "history.jsonl") {
			return nil
		}
		if info.ModTime().Before(since) {
			return nil
		}
		rel, err := filepath.Rel(filepath.Join(home, ".codex", "sessions"), path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return nil
		}
		sessionID := strings.TrimSpace(parts[len(parts)-2])
		if sessionID == "" {
			return nil
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = sessionID
			newestTime = info.ModTime()
		}
		return nil
	})
	return newest
}

func runCommandCapture(ctx context.Context, workdir, bin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = workdir
	}
	var outb, errb strings.Builder
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

func augmentDirectPrompt(message, activeFile, selectionText string, openFiles []string) string {
	var parts []string
	if strings.TrimSpace(activeFile) != "" {
		parts = append(parts, "Active file: "+strings.TrimSpace(activeFile))
	}
	if len(openFiles) > 0 {
		parts = append(parts, "Open files:\n- "+strings.Join(openFiles, "\n- "))
	}
	if strings.TrimSpace(selectionText) != "" {
		parts = append(parts, "Selection:\n"+strings.TrimSpace(selectionText))
	}
	if len(parts) == 0 {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(message) + "\n\n---\n\nWorkspace hints:\n" + strings.Join(parts, "\n\n")
}

func cleanDirectProviderText(stdout string) string {
	text := strings.ReplaceAll(stdout, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trimmed, "[dockpipe]") || strings.HasPrefix(trimmed, "Using:") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func ensureMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	return meta
}

func boolFromMap(m map[string]any, key string) bool {
	value, ok := m[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func stringFromMap(m map[string]any, key string) (string, bool) {
	value, ok := m[key]
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text, text != ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
