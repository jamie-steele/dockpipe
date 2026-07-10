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
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
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

type providerPoolWarmResponse struct {
	ContractVersion string               `json:"contract_version"`
	Providers       []providerPoolStatus `json:"providers"`
	GeneratedAt     string               `json:"generated_at"`
}

type providerPoolStopResponse struct {
	ContractVersion string                   `json:"contract_version"`
	Providers       []providerPoolStopStatus `json:"providers"`
	GeneratedAt     string                   `json:"generated_at"`
}

type providerPoolStopStatus struct {
	Provider       string         `json:"provider"`
	State          string         `json:"state"`
	Status         string         `json:"status"`
	StoppedWorkers []string       `json:"stopped_workers,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
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
	Provider      string `json:"provider"`
	LeaseID       string `json:"lease_id"`
	SessionID     string `json:"session_id,omitempty"`
	Role          string `json:"role,omitempty"`
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	NodeID        string `json:"node_id,omitempty"`
	Workdir       string `json:"workdir"`
	StartedAt     string `json:"started_at"`
}

type providerPoolPromptOptions struct {
	Workdir             string
	Provider            string
	Model               string
	Prompt              string
	SessionID           string
	Role                string
	WorkflowRunID       string
	NodeID              string
	MaxActive           int
	QueueTimeoutSeconds int
	ActiveFile          string
	OpenFiles           []string
	SelectionText       string
}

var providerPoolClaudeImageBuild sync.Mutex
var stopProviderPoolClaudeWorkersFunc = stopProviderPoolClaudeWorkers

func providerPoolCmd(argv []string) {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe provider-pool <catalog|status|warm|prompt|stop> [flags]")
		os.Exit(2)
	}
	switch argv[0] {
	case "catalog":
		providerPoolCatalogCmd(argv[1:])
	case "status":
		providerPoolStatusCmd(argv[1:])
	case "warm":
		providerPoolWarmCmd(argv[1:])
	case "prompt":
		providerPoolPromptCmd(argv[1:])
	case "stop":
		providerPoolStopCmd(argv[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown provider-pool subcommand %q\n", argv[0])
		fmt.Fprintln(os.Stderr, "usage: dorkpipe provider-pool <catalog|status|warm|prompt|stop> [flags]")
		os.Exit(2)
	}
}

func providerPoolCatalogCmd(argv []string) {
	fs := flag.NewFlagSet("provider-pool catalog", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	payload, err := buildProviderPoolCatalogResponse(wd, false)
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
	payload, err := buildProviderPoolCatalogResponse(wd, false)
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

func providerPoolWarmCmd(argv []string) {
	fs := flag.NewFlagSet("provider-pool warm", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	provider := fs.String("provider", "", "provider id filter")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	payload, err := warmProviderPools(wd, strings.TrimSpace(*provider))
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
	for _, item := range payload.Providers {
		fmt.Println(item.Status)
	}
}

func providerPoolStopCmd(argv []string) {
	fs := flag.NewFlagSet("provider-pool stop", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	provider := fs.String("provider", "", "provider id filter")
	asJSON := fs.Bool("json", false, "print JSON")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	payload, err := stopProviderPools(context.Background(), wd, strings.TrimSpace(*provider))
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
	for _, item := range payload.Providers {
		fmt.Println(item.Status)
	}
}

func providerPoolPromptCmd(argv []string) {
	argv = providerPoolPromptArgs(argv)
	fs := flag.NewFlagSet("provider-pool prompt", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	provider := fs.String("provider", "", "provider override")
	model := fs.String("model", "", "model override")
	prompt := fs.String("prompt", "", "prompt text")
	sessionID := fs.String("session-id", "", "provider-pool session id")
	role := fs.String("role", "", "provider-pool role hint")
	workflowRunID := fs.String("workflow-run-id", "", "workflow run attribution")
	nodeID := fs.String("node-id", "", "workflow node attribution")
	maxActive := fs.Int("max-active", 0, "provider-pool active lease cap override")
	queueTimeoutSeconds := fs.Int("queue-timeout-seconds", 0, "seconds to wait for a provider-pool lease before returning queued")
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := runProviderPoolPrompt(ctx, providerPoolPromptOptions{
		Workdir:             wd,
		Provider:            strings.TrimSpace(*provider),
		Model:               strings.TrimSpace(*model),
		Prompt:              strings.TrimSpace(*prompt),
		SessionID:           strings.TrimSpace(*sessionID),
		Role:                strings.TrimSpace(*role),
		WorkflowRunID:       strings.TrimSpace(*workflowRunID),
		NodeID:              strings.TrimSpace(*nodeID),
		MaxActive:           *maxActive,
		QueueTimeoutSeconds: *queueTimeoutSeconds,
		ActiveFile:          strings.TrimSpace(*activeFile),
		OpenFiles:           uniqueNonEmpty(openFiles),
		SelectionText:       strings.TrimSpace(*selectionText),
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

func buildProviderPoolCatalogResponse(workdir string, allowWarmStart bool) (*providerPoolCatalogResponse, error) {
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
		status, err := providerPoolStatusFor(workdir, item, defaultModel, "", allowWarmStart)
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
	startedAt := time.Now()
	timings := map[string]int64{}
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
	statusStartedAt := time.Now()
	status, err := providerPoolStatusFor(opts.Workdir, provider, chosenModel, opts.SessionID, true)
	if err != nil {
		return nil, err
	}
	timings["status_ms"] = elapsedMillis(statusStartedAt)
	if status != nil && status.Metadata != nil {
		mergePromptTimings(timings, status.Metadata["timings_ms"])
	}
	if status.State == "warming" {
		waitStartedAt := time.Now()
		waitedStatus, waitErr := waitForProviderPoolPromptReady(ctx, opts.Workdir, provider, chosenModel, opts.SessionID)
		if waitErr != nil {
			return nil, waitErr
		}
		timings["readiness_wait_ms"] = elapsedMillis(waitStartedAt)
		if waitedStatus != nil {
			status = waitedStatus
		}
	}
	if status.State != "ready" {
		status.Metadata = ensureMetadata(status.Metadata)
		status.Metadata["timings_ms"] = promptTimings(timings, startedAt)
		enrichProviderPoolPromptMetadata(status.Metadata, provider.ID, chosenModel, opts)
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
	leaseStartedAt := time.Now()
	queueTimeout := time.Duration(max(0, opts.QueueTimeoutSeconds)) * time.Second
	releaseLease, queued, err := acquireProviderPoolLease(ctx, opts.Workdir, provider, opts.SessionID, opts.Role, opts.WorkflowRunID, opts.NodeID, opts.MaxActive, queueTimeout)
	if err != nil {
		return nil, err
	}
	timings["queue_wait_ms"] = elapsedMillis(leaseStartedAt)
	if queued {
		queueStatus := *status
		queueStatus.State = "queued"
		queueStatus.Status = fmt.Sprintf("Provider: %s | State: queued | Active workers reached %d/%d", provider.DisplayName, queueStatus.ActiveWorkers, queueStatus.MaxActive)
		queueStatus.Metadata = ensureMetadata(queueStatus.Metadata)
		queueStatus.Metadata["queue_reason"] = "max_active_reached"
		queueStatus.Metadata["queue_timeout_seconds"] = max(0, opts.QueueTimeoutSeconds)
		queueStatus.Metadata["max_active"] = effectiveProviderPoolMaxActive(provider, opts.MaxActive)
		queueStatus.Metadata["timings_ms"] = promptTimings(timings, startedAt)
		enrichProviderPoolPromptMetadata(queueStatus.Metadata, provider.ID, chosenModel, opts)
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
	runStartedAt := time.Now()
	var result *providerPoolPromptResponse
	switch provider.ID {
	case "ollama":
		result, err = runProviderPoolOllamaPrompt(ctx, opts, chosenModel)
	case "codex":
		result, err = runProviderPoolCodexPrompt(ctx, opts, chosenModel)
	case "claude":
		result, err = runProviderPoolClaudePrompt(ctx, provider, opts, chosenModel)
	default:
		return nil, fmt.Errorf("provider-pool prompt not implemented for %q", provider.ID)
	}
	if err != nil {
		return nil, err
	}
	timings["provider_prompt_ms"] = elapsedMillis(runStartedAt)
	result.Metadata = ensureMetadata(result.Metadata)
	mergePromptTimings(timings, result.Metadata["timings_ms"])
	result.Metadata["timings_ms"] = promptTimings(timings, startedAt)
	enrichProviderPoolPromptMetadata(result.Metadata, provider.ID, chosenModel, opts)
	return result, nil
}

func enrichProviderPoolPromptMetadata(metadata map[string]any, providerID, chosenModel string, opts providerPoolPromptOptions) {
	if metadata == nil {
		return
	}
	if _, ok := metadata["provider_preset"]; !ok {
		metadata["provider_preset"] = providerID
	}
	if strings.TrimSpace(chosenModel) != "" {
		if _, ok := metadata["selected_model"]; !ok {
			metadata["selected_model"] = chosenModel
		}
	}
	if strings.TrimSpace(opts.SessionID) != "" {
		metadata["session_id"] = strings.TrimSpace(opts.SessionID)
	}
	if strings.TrimSpace(opts.Role) != "" {
		metadata["role"] = strings.TrimSpace(opts.Role)
	}
	if strings.TrimSpace(opts.WorkflowRunID) != "" {
		metadata["workflow_run_id"] = strings.TrimSpace(opts.WorkflowRunID)
	}
	if strings.TrimSpace(opts.NodeID) != "" {
		metadata["node_id"] = strings.TrimSpace(opts.NodeID)
	}
	if opts.MaxActive > 0 {
		metadata["requested_max_active"] = opts.MaxActive
	}
	if opts.QueueTimeoutSeconds > 0 {
		metadata["queue_timeout_seconds"] = opts.QueueTimeoutSeconds
	}
	if _, ok := metadata["prompt_turn_id"]; !ok {
		metadata["prompt_turn_id"] = providerPoolPromptTurnID()
	}
}

func waitForProviderPoolPromptReady(ctx context.Context, workdir string, provider providerPoolProvider, chosenModel, sessionID string) (*providerPoolStatus, error) {
	timeout := providerPoolPromptWarmWaitTimeout()
	if timeout <= 0 {
		return nil, nil
	}
	fmt.Fprintf(os.Stderr, "Provider: %s | State: warming | Waiting up to %s for a ready worker\n", provider.DisplayName, timeout.Round(time.Second))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		status, err := providerPoolStatusFor(workdir, provider, chosenModel, sessionID, true)
		if err != nil {
			return nil, err
		}
		if status.State == "ready" || status.State == "failed" || status.State == "auth-required" || status.State == "disabled" {
			return status, nil
		}
	}
	return providerPoolStatusFor(workdir, provider, chosenModel, sessionID, true)
}

func providerPoolPromptWarmWaitTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS"))
	if raw == "" {
		return 10 * time.Second
	}
	if strings.EqualFold(raw, "0") || strings.EqualFold(raw, "off") || strings.EqualFold(raw, "false") {
		return 0
	}
	seconds, err := time.ParseDuration(raw + "s")
	if err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}
	if duration, err := time.ParseDuration(raw); err == nil {
		if duration < 0 {
			return 0
		}
		return duration
	}
	return 10 * time.Second
}

func warmProviderPools(workdir, providerFilter string) (*providerPoolWarmResponse, error) {
	doc, err := loadProviderPoolCatalog(workdir)
	if err != nil {
		return nil, err
	}
	resp := &providerPoolWarmResponse{
		ContractVersion: "v1",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	for _, item := range doc.Providers {
		if providerFilter != "" && !strings.EqualFold(strings.TrimSpace(item.ID), providerFilter) {
			continue
		}
		chosenModel := providerPoolChosenModel(item)
		status, err := providerPoolStatusFor(workdir, item, chosenModel, "", item.Pool.MinReady > 0)
		if err != nil {
			return nil, err
		}
		resp.Providers = append(resp.Providers, *status)
	}
	sort.Slice(resp.Providers, func(i, j int) bool { return resp.Providers[i].Provider < resp.Providers[j].Provider })
	return resp, nil
}

func stopProviderPools(ctx context.Context, workdir, providerFilter string) (*providerPoolStopResponse, error) {
	doc, err := loadProviderPoolCatalog(workdir)
	if err != nil {
		return nil, err
	}
	resp := &providerPoolStopResponse{
		ContractVersion: "v1",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	for _, item := range doc.Providers {
		if providerFilter != "" && !strings.EqualFold(strings.TrimSpace(item.ID), providerFilter) {
			continue
		}
		status := providerPoolStopStatus{
			Provider: item.ID,
			State:    "stopped",
			Metadata: map[string]any{
				"provider_preset": item.ID,
				"warm_mode":       item.Pool.WarmMode,
				"warm_source":     item.Pool.WarmSource,
			},
		}
		switch item.ID {
		case "claude":
			containerName := providerPoolClaudeContainerName(workdir)
			status.Metadata["container_name"] = containerName
			stopped, stopErr := stopProviderPoolClaudeWorkersFunc(ctx, workdir)
			if stopErr != nil {
				status.State = "failed"
				status.Status = fmt.Sprintf("Provider: %s | State: failed | %s", item.DisplayName, stopErr.Error())
				status.Metadata["error"] = stopErr.Error()
			} else if len(stopped) > 0 {
				status.StoppedWorkers = append(status.StoppedWorkers, stopped...)
				status.Status = fmt.Sprintf("Provider: %s | State: stopped | Removed %d worker(s)", item.DisplayName, len(stopped))
			} else {
				status.Status = fmt.Sprintf("Provider: %s | State: stopped | No worker to remove for this workdir", item.DisplayName)
			}
		default:
			status.Status = fmt.Sprintf("Provider: %s | State: stopped | No managed worker teardown required", item.DisplayName)
			_ = removeProviderPoolLease(workdir, item.ID)
		}
		resp.Providers = append(resp.Providers, status)
	}
	sort.Slice(resp.Providers, func(i, j int) bool { return resp.Providers[i].Provider < resp.Providers[j].Provider })
	return resp, nil
}

func stopProviderPoolClaudeWorkers(ctx context.Context, workdir string) ([]string, error) {
	_ = removeProviderPoolLease(workdir, "claude")
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, nil
	}
	var stopped []string
	for _, name := range providerPoolClaudeContainerNameCandidates(workdir) {
		removed, err := removeDockerContainer(ctx, name)
		if err != nil {
			return stopped, err
		}
		if removed {
			stopped = append(stopped, name)
		}
	}
	for _, hash := range providerPoolWorkdirHashCandidates(workdir) {
		stdout, stderr, code, err := runCommandCapture(ctx, "", "docker", "ps", "-aq",
			"--filter", "label=com.dockpipe.provider-pool=true",
			"--filter", "label=com.dockpipe.provider-pool.provider=claude",
			"--filter", "label=com.dockpipe.provider-pool.workdir-hash="+hash,
		)
		if err != nil {
			return stopped, err
		}
		if code != 0 {
			text := strings.TrimSpace(firstNonEmptyString(stderr, stdout, fmt.Sprintf("docker ps exited %d", code)))
			return stopped, fmt.Errorf("list Claude provider workers: %s", text)
		}
		for _, id := range strings.Fields(stdout) {
			removed, err := removeDockerContainer(ctx, id)
			if err != nil {
				return stopped, err
			}
			if removed {
				stopped = append(stopped, id)
			}
		}
	}
	return uniqueNonEmpty(stopped), nil
}

func removeDockerContainer(ctx context.Context, container string) (bool, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return false, nil
	}
	stdout, stderr, code, err := runCommandCapture(ctx, "", "docker", "rm", "-f", container)
	if err != nil {
		return false, err
	}
	if code != 0 {
		text := strings.TrimSpace(firstNonEmptyString(stderr, stdout))
		if strings.Contains(strings.ToLower(text), "no such container") {
			return false, nil
		}
		return false, fmt.Errorf("remove provider worker %s: %s", container, text)
	}
	return strings.TrimSpace(stdout) != "", nil
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

func providerPoolStatusFor(workdir string, provider providerPoolProvider, chosenModel, sessionID string, allowWarmStart bool) (*providerPoolStatus, error) {
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
		statusTimings := map[string]int64{}
		authStartedAt := time.Now()
		auth := providerPoolClaudeAuthStatus()
		statusTimings["auth_check_ms"] = elapsedMillis(authStartedAt)
		status.Metadata["timings_ms"] = statusTimings
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
		status.Metadata["worker_mode"] = providerPoolClaudeWorkerMode()
		imageStartedAt := time.Now()
		if !providerPoolClaudeImageReady(workdir) {
			statusTimings["image_check_ms"] = elapsedMillis(imageStartedAt)
			if allowWarmStart {
				imageBuildStartedAt := time.Now()
				if err := providerPoolEnsureClaudeImage(context.Background(), workdir); err != nil {
					statusTimings["image_build_ms"] = elapsedMillis(imageBuildStartedAt)
					status.State = "failed"
					status.DisableReason = err.Error()
					status.NextAction = "Inspect the DockPipe image build output, then retry."
					status.Status = fmt.Sprintf("Provider: %s | State: failed | %s", provider.DisplayName, status.DisableReason)
					return status, nil
				}
				statusTimings["image_build_ms"] = elapsedMillis(imageBuildStartedAt)
			}
		} else {
			statusTimings["image_check_ms"] = elapsedMillis(imageStartedAt)
		}
		imageRecheckStartedAt := time.Now()
		if !providerPoolClaudeImageReady(workdir) {
			statusTimings["image_recheck_ms"] = elapsedMillis(imageRecheckStartedAt)
			status.State = "disabled"
			status.DisableReason = "resolver image dockpipe-claude:latest is missing."
			status.NextAction = "Build or materialize the Claude resolver image, then retry."
			status.Status = fmt.Sprintf("Provider: %s | State: disabled | %s", provider.DisplayName, status.DisableReason)
			return status, nil
		}
		statusTimings["image_recheck_ms"] = elapsedMillis(imageRecheckStartedAt)
		runningStartedAt := time.Now()
		running, _ := providerPoolClaudeContainerRunning(containerName)
		statusTimings["container_running_check_ms"] = elapsedMillis(runningStartedAt)
		if running {
			status.State = "ready"
			status.ReadyWorkers = 1
			status.Status = fmt.Sprintf("Provider: %s | State: ready | Warm guarded worker %s", provider.DisplayName, containerName)
			status.WorkerID = containerName
			status.BoundSessionID = strings.TrimSpace(sessionID)
			return status, nil
		}
		if allowWarmStart {
			workerStartStartedAt := time.Now()
			if started, startErr := providerPoolEnsureClaudeWarmContainer(context.Background(), workdir, containerName); startErr != nil {
				statusTimings["worker_start_ms"] = elapsedMillis(workerStartStartedAt)
				status.State = "failed"
				status.DisableReason = startErr.Error()
				status.Status = fmt.Sprintf("Provider: %s | State: failed | %s", provider.DisplayName, status.DisableReason)
				return status, nil
			} else if started {
				statusTimings["worker_start_ms"] = elapsedMillis(workerStartStartedAt)
				status.State = "warming"
				status.Status = fmt.Sprintf("Provider: %s | State: warming | Started guarded worker %s", provider.DisplayName, containerName)
				status.WorkerID = containerName
				status.BoundSessionID = strings.TrimSpace(sessionID)
				status.NextAction = "Retry once the guarded container worker is ready."
				return status, nil
			}
		}
		status.State = "warming"
		status.Status = fmt.Sprintf("Provider: %s | State: warming | Waiting for guarded worker %s", provider.DisplayName, containerName)
		status.WorkerID = containerName
		status.BoundSessionID = strings.TrimSpace(sessionID)
		if allowWarmStart {
			status.NextAction = "Retry once the guarded container worker is ready."
		} else {
			status.NextAction = "Warm the Claude provider pool or open Pipeon so the guarded worker can start."
		}
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
	codexPath, err := providerPoolCodexCLIPath()
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
	scratchDir, err := statepaths.ProviderPoolScratchDir(opts.Workdir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return nil, err
	}
	lastMessagePath, err := os.CreateTemp(scratchDir, "codex-last-message-*.txt")
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
	if code != 0 || providerPoolCodexOutputFailed(text, stdout, stderr) {
		state = "failed"
		if code == 0 {
			code = 1
		}
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
			"selected_model":  chosenModel,
			"sandbox":         "workspace-write",
			"model_source":    map[bool]string{true: "explicit", false: "codex_config"}[!strings.EqualFold(chosenModel, "config") && chosenModel != ""],
			"session_id":      strings.TrimSpace(opts.SessionID),
			"worker_id":       providerPoolCodexWorkerID(opts.Workdir),
			"worker_mode":     map[bool]string{true: "host_resume", false: "host_prompt"}[codexSession != ""],
		},
	}, nil
}

func providerPoolCodexOutputFailed(parts ...string) bool {
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		if strings.Contains(text, "] ERROR:") || strings.HasPrefix(text, "ERROR:") {
			return true
		}
		if strings.Contains(text, "unexpected status 400 Bad Request") || strings.Contains(text, "error sending request for url") {
			return true
		}
	}
	return false
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
				"worker_mode":     providerPoolClaudeWorkerMode(),
			},
		}, nil
	}
	if providerPoolClaudeStreamWorkerEnabled() {
		result, streamErr := runProviderPoolClaudeStreamPrompt(ctx, provider, opts, chosenModel, containerName)
		if streamErr == nil {
			return result, nil
		}
		if !providerPoolClaudeSinglePromptFallbackEnabled() {
			return &providerPoolPromptResponse{
				Provider: "claude",
				Model:    chosenModel,
				State:    "failed",
				Status:   fmt.Sprintf("Provider: %s | State: failed | Stream worker failed", provider.DisplayName),
				Text:     "Claude stream worker failed before returning a response.",
				ExitCode: -1,
				Stderr:   streamErr.Error(),
				Metadata: map[string]any{
					"provider_preset":       "claude",
					"container_name":        containerName,
					"session_id":            strings.TrimSpace(opts.SessionID),
					"worker_id":             providerPoolClaudeStreamWorkerID(opts.Workdir, opts.SessionID, chosenModel),
					"worker_mode":           "stream_worker",
					"stream_restart_reason": "stream_worker_error",
					"stream_error":          streamErr.Error(),
				},
			}, nil
		}
		fallback, fallbackErr := runProviderPoolClaudeSinglePrompt(ctx, provider, opts, chosenModel, containerName)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		fallback.Metadata = ensureMetadata(fallback.Metadata)
		fallback.Metadata["worker_mode"] = "single_prompt_fallback"
		fallback.Metadata["stream_restart_reason"] = "stream_worker_error"
		fallback.Metadata["stream_error"] = streamErr.Error()
		return fallback, nil
	}
	return runProviderPoolClaudeSinglePrompt(ctx, provider, opts, chosenModel, containerName)
}

func runProviderPoolClaudeSinglePrompt(ctx context.Context, provider providerPoolProvider, opts providerPoolPromptOptions, chosenModel, containerName string) (*providerPoolPromptResponse, error) {
	runCtx, cancel := context.WithTimeout(ctx, providerPoolPromptExecutionTimeout("claude", 2*time.Minute))
	defer cancel()
	args := providerPoolClaudePromptDockerArgs(containerName, chosenModel, augmentDirectPrompt(opts.Prompt, opts.ActiveFile, opts.SelectionText, opts.OpenFiles))
	promptStartedAt := time.Now()
	stdout, stderr, code, runErr := runCommandCapture(runCtx, opts.Workdir, "docker", args...)
	promptMs := elapsedMillis(promptStartedAt)
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return &providerPoolPromptResponse{
				Provider: "claude",
				Model:    chosenModel,
				State:    "failed",
				Status:   fmt.Sprintf("Provider: %s | State: failed | Timed out waiting for Claude after %s", provider.DisplayName, providerPoolPromptExecutionTimeout("claude", 2*time.Minute).Round(time.Second)),
				Text:     "Claude direct orchestration timed out before returning a response.",
				ExitCode: -1,
				Stdout:   stdout,
				Stderr:   stderr,
				Metadata: map[string]any{
					"provider_preset": "claude",
					"container_name":  containerName,
					"session_id":      strings.TrimSpace(opts.SessionID),
					"worker_id":       containerName,
					"worker_mode":     "single_prompt",
					"timeout":         providerPoolPromptExecutionTimeout("claude", 2*time.Minute).String(),
					"timings_ms": map[string]int64{
						"claude_command_ms": promptMs,
						"provider_turn_ms":  promptMs,
					},
				},
			}, nil
		}
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
			"selected_model":  chosenModel,
			"container_name":  containerName,
			"session_id":      strings.TrimSpace(opts.SessionID),
			"worker_id":       containerName,
			"worker_mode":     "single_prompt",
			"timings_ms": map[string]int64{
				"claude_command_ms": promptMs,
				"provider_turn_ms":  promptMs,
			},
		},
	}, nil
}

func runProviderPoolClaudeStreamPrompt(ctx context.Context, provider providerPoolProvider, opts providerPoolPromptOptions, chosenModel, containerName string) (*providerPoolPromptResponse, error) {
	workerID := providerPoolClaudeStreamWorkerID(opts.Workdir, opts.SessionID, chosenModel)
	socketPath := providerPoolClaudeStreamSocketPath(workerID)
	timings := map[string]int64{}
	streamStartStartedAt := time.Now()
	reused, err := providerPoolEnsureClaudeStreamWorker(ctx, containerName, socketPath, chosenModel, timings)
	timings["stream_start_ms"] = elapsedMillis(streamStartStartedAt)
	if err != nil {
		return nil, err
	}
	promptTurnID := providerPoolPromptTurnID()
	runCtx, cancel := context.WithTimeout(ctx, providerPoolPromptExecutionTimeout("claude", 2*time.Minute))
	defer cancel()
	requestStartedAt := time.Now()
	stdout, stderr, code, runErr := runCommandCapture(runCtx, opts.Workdir, "docker", providerPoolClaudeStreamClientDockerArgs(containerName, socketPath, augmentDirectPrompt(opts.Prompt, opts.ActiveFile, opts.SelectionText, opts.OpenFiles), promptTurnID)...)
	providerTurnMs := elapsedMillis(requestStartedAt)
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			timings["provider_turn_ms"] = providerTurnMs
			return &providerPoolPromptResponse{
				Provider: "claude",
				Model:    chosenModel,
				State:    "failed",
				Status:   fmt.Sprintf("Provider: %s | State: failed | Timed out waiting for Claude stream worker after %s", provider.DisplayName, providerPoolPromptExecutionTimeout("claude", 2*time.Minute).Round(time.Second)),
				Text:     "Claude stream worker timed out before returning a response.",
				ExitCode: -1,
				Stdout:   stdout,
				Stderr:   stderr,
				Metadata: map[string]any{
					"provider_preset": "claude",
					"selected_model":  chosenModel,
					"container_name":  containerName,
					"session_id":      strings.TrimSpace(opts.SessionID),
					"worker_id":       workerID,
					"worker_mode":     "stream_worker",
					"prompt_turn_id":  promptTurnID,
					"stream_reused":   reused,
					"timeout":         providerPoolPromptExecutionTimeout("claude", 2*time.Minute).String(),
					"timings_ms":      timings,
				},
			}, nil
		}
		return nil, runErr
	}
	var payload providerPoolClaudeStreamClientResponse
	if strings.TrimSpace(stdout) != "" {
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
			return nil, fmt.Errorf("parse Claude stream worker response: %w: %s", err, strings.TrimSpace(stdout))
		}
	}
	mergePromptTimings(timings, payload.TimingsMS)
	if timings["provider_turn_ms"] == 0 {
		timings["provider_turn_ms"] = providerTurnMs
	}
	text := strings.TrimSpace(payload.Text)
	if text == "" && strings.TrimSpace(stderr) != "" {
		text = strings.TrimSpace(stderr)
	}
	if text == "" {
		text = "(Claude returned no output.)"
	}
	state := "ready"
	status := fmt.Sprintf("Provider: %s | State: ready | Stream worker %s", provider.DisplayName, workerID)
	if code != 0 || !payload.OK {
		state = "failed"
		status = fmt.Sprintf("Provider: %s | State: failed | Stream worker exit %d", provider.DisplayName, code)
		if strings.TrimSpace(payload.Error) != "" {
			text = payload.Error
		}
	}
	return &providerPoolPromptResponse{
		Provider: "claude",
		Model:    chosenModel,
		State:    state,
		Status:   status,
		Text:     text,
		ExitCode: code,
		Stdout:   stdout,
		Stderr:   firstNonEmptyString(stderr, payload.Stderr),
		Metadata: map[string]any{
			"provider_preset":       "claude",
			"selected_model":        chosenModel,
			"container_name":        containerName,
			"session_id":            strings.TrimSpace(opts.SessionID),
			"worker_id":             workerID,
			"worker_mode":           "stream_worker",
			"provider_session_id":   payload.ProviderSessionID,
			"provider_request_id":   payload.ProviderRequestID,
			"prompt_turn_id":        promptTurnID,
			"prompt_count":          payload.PromptCount,
			"stream_reused":         reused,
			"stream_restart_reason": map[bool]string{true: "", false: "worker_not_running"}[reused],
			"timings_ms":            timings,
		},
	}, nil
}

type providerPoolClaudeStreamClientResponse struct {
	OK                bool             `json:"ok"`
	Text              string           `json:"text"`
	Error             string           `json:"error"`
	Stderr            string           `json:"stderr"`
	ProviderSessionID string           `json:"provider_session_id"`
	ProviderRequestID string           `json:"provider_request_id"`
	PromptCount       int              `json:"prompt_count"`
	TimingsMS         map[string]int64 `json:"timings_ms"`
}

func providerPoolClaudePromptDockerArgs(containerName, chosenModel, prompt string) []string {
	args := []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work", containerName, "claude", "--dangerously-skip-permissions"}
	if strings.TrimSpace(chosenModel) != "" {
		args = append(args, "--model", chosenModel)
	}
	return append(args, "-p", prompt)
}

func providerPoolClaudeStreamWorkerEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("PIPEON_PROVIDER_POOL_CLAUDE_STREAM_WORKER"))
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "off", "disabled", "single", "single_prompt":
		return false
	default:
		return true
	}
}

func providerPoolClaudeSinglePromptFallbackEnabled() bool {
	for _, key := range []string{"DORKPIPE_PROVIDER_POOL_CLAUDE_SINGLE_PROMPT_FALLBACK", "PIPEON_PROVIDER_POOL_CLAUDE_SINGLE_PROMPT_FALLBACK"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on", "enabled":
			return true
		}
	}
	return false
}

func providerPoolClaudeWorkerMode() string {
	if providerPoolClaudeStreamWorkerEnabled() {
		return "stream_worker"
	}
	return "single_prompt"
}

func providerPoolClaudeStreamWorkerID(workdir, sessionID, chosenModel string) string {
	session := strings.TrimSpace(sessionID)
	if session == "" {
		session = "default"
	}
	model := strings.TrimSpace(chosenModel)
	if model == "" {
		model = "default"
	}
	return "claude-stream-" + providerPoolWorkdirHash(workdir+"|"+session+"|"+model)
}

func providerPoolClaudeStreamSocketPath(workerID string) string {
	return "/tmp/dorkpipe-provider-pool/" + strings.TrimSpace(workerID) + ".sock"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func providerPoolPromptTurnID() string {
	return fmt.Sprintf("turn-%d", time.Now().UTC().UnixNano())
}

func providerPoolEnsureClaudeStreamWorker(ctx context.Context, containerName, socketPath, chosenModel string, timings map[string]int64) (bool, error) {
	if providerPoolClaudeStreamWorkerPing(ctx, containerName, socketPath) {
		return true, nil
	}
	_, _, _, _ = runCommandCapture(ctx, "", "docker", "exec", containerName, "bash", "-lc", "pkill -f "+shellSingleQuote(socketPath)+" 2>/dev/null || true; rm -f "+shellSingleQuote(socketPath))
	startedAt := time.Now()
	stdout, stderr, code, err := runCommandCapture(ctx, "", "docker", providerPoolClaudeStreamDaemonDockerArgs(containerName, socketPath, chosenModel)...)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("start Claude stream worker daemon: %s", strings.TrimSpace(firstNonEmptyString(stderr, stdout, fmt.Sprintf("docker exec exited %d", code))))
	}
	timings["stream_process_start_ms"] = elapsedMillis(startedAt)
	readyStartedAt := time.Now()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if providerPoolClaudeStreamWorkerPing(ctx, containerName, socketPath) {
			timings["stream_ready_ms"] = elapsedMillis(readyStartedAt)
			return false, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	timings["stream_ready_ms"] = elapsedMillis(readyStartedAt)
	return false, errors.New("Claude stream worker daemon did not become ready")
}

func providerPoolClaudeStreamWorkerPing(ctx context.Context, containerName, socketPath string) bool {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stdout, _, code, err := runCommandCapture(pingCtx, "", "docker", providerPoolClaudeStreamClientDockerArgs(containerName, socketPath, "", "ping")...)
	if err != nil || code != 0 {
		return false
	}
	var payload map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload) != nil {
		return false
	}
	protocol, _ := stringFromMap(payload, "protocol_version")
	return boolFromMap(payload, "ok") && protocol == "stream_worker_v1"
}

func providerPoolClaudeStreamDaemonDockerArgs(containerName, socketPath, chosenModel string) []string {
	return []string{"exec", "-d", "-u", "node", "-e", "HOME=/home/node", "-w", "/work", containerName, "node", "-e", providerPoolClaudeStreamDaemonScript(), socketPath, chosenModel}
}

func providerPoolClaudeStreamClientDockerArgs(containerName, socketPath, prompt, promptTurnID string) []string {
	return []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work", containerName, "node", "-e", providerPoolClaudeStreamClientScript(), socketPath, prompt, promptTurnID}
}

func providerPoolClaudeStreamDaemonScript() string {
	return `const net = require('net');
const fs = require('fs');
const path = require('path');
const readline = require('readline');
const { spawn } = require('child_process');

const socketPath = process.argv[1];
const model = (process.argv[2] || '').trim();
fs.mkdirSync(path.dirname(socketPath), { recursive: true });
try { fs.unlinkSync(socketPath); } catch (_) {}

const timingsBase = {};
const stderrLines = [];
let promptCount = 0;
let current = null;

const claudeArgs = ['--dangerously-skip-permissions'];
if (model) claudeArgs.push('--model', model);
claudeArgs.push('-p', '--input-format', 'stream-json', '--output-format', 'stream-json', '--include-partial-messages', '--replay-user-messages', '--verbose');
const child = spawn('claude', claudeArgs, { cwd: '/work', env: { ...process.env, HOME: '/home/node' }, stdio: ['pipe', 'pipe', 'pipe'] });
timingsBase.stream_pid = child.pid || 0;

child.stderr.on('data', chunk => {
  for (const line of String(chunk).split(/\r?\n/)) {
    if (line.trim()) stderrLines.push(line);
  }
  while (stderrLines.length > 40) stderrLines.shift();
});

function eventText(event) {
  if (!event || typeof event !== 'object') return '';
  if (typeof event.result === 'string') return event.result;
  if (typeof event.text === 'string') return event.text;
  if (event.delta && typeof event.delta.text === 'string') return event.delta.text;
  const message = event.message || {};
  const content = event.content || message.content;
  if (Array.isArray(content)) {
    return content.map(item => {
      if (typeof item === 'string') return item;
      if (item && typeof item.text === 'string') return item.text;
      return '';
    }).join('');
  }
  return '';
}

function finish(payload) {
  if (!current) return;
  const response = {
    ok: payload.ok,
    text: (payload.text || current.text.join('')).trim(),
    error: payload.error || '',
    stderr: stderrLines.join('\n'),
    provider_session_id: payload.provider_session_id || current.providerSessionID || '',
    provider_request_id: payload.provider_request_id || current.providerRequestID || '',
    prompt_count: promptCount,
    timings_ms: {
      time_to_request_ms: current.timeToRequestMS,
      time_to_first_token_ms: current.firstTokenAt ? current.firstTokenAt - current.startedAt : 0,
      provider_turn_ms: Date.now() - current.startedAt
    }
  };
  current.socket.end(JSON.stringify(response) + '\n');
  current = null;
}

readline.createInterface({ input: child.stdout }).on('line', line => {
  if (!current) return;
  let event;
  try { event = JSON.parse(line); } catch (_) { return; }
  const text = eventText(event);
  if (text) {
    if (!current.firstTokenAt) current.firstTokenAt = Date.now();
    if (event.type !== 'result') current.text.push(text);
  }
  if (typeof event.session_id === 'string') current.providerSessionID = event.session_id;
  if (typeof event.request_id === 'string') current.providerRequestID = event.request_id;
  if (event.type === 'result') {
    finish({
      ok: true,
      text: typeof event.result === 'string' ? event.result : current.text.join(''),
      provider_session_id: event.session_id || '',
      provider_request_id: event.request_id || ''
    });
  }
});

child.on('exit', (code, signal) => {
  if (current) finish({ ok: false, error: 'claude stream exited: code=' + code + ' signal=' + signal });
  process.exit(code || 0);
});

function handleSocketRequest(socket, raw) {
  let req;
  try { req = JSON.parse(raw || '{}'); } catch (err) {
    socket.end(JSON.stringify({ ok: false, error: 'invalid request json: ' + err.message }) + '\n');
    return;
  }
  if (req.type === 'ping') {
    socket.end(JSON.stringify({ ok: true, protocol_version: 'stream_worker_v1', prompt_count: promptCount, stream_pid: timingsBase.stream_pid }) + '\n');
    return;
  }
  if (current) {
    socket.end(JSON.stringify({ ok: false, error: 'stream worker is busy' }) + '\n');
    return;
  }
  const prompt = String(req.prompt || '').trim();
  if (!prompt) {
    socket.end(JSON.stringify({ ok: false, error: 'prompt is required' }) + '\n');
    return;
  }
  promptCount++;
  const startedAt = Date.now();
  current = { socket, startedAt, firstTokenAt: 0, text: [], providerSessionID: '', providerRequestID: '', timeToRequestMS: 0 };
  const input = { type: 'user', message: { role: 'user', content: [{ type: 'text', text: prompt }] } };
  child.stdin.write(JSON.stringify(input) + '\n', () => {
    if (current) current.timeToRequestMS = Date.now() - startedAt;
  });
}

const server = net.createServer(socket => {
  let raw = '';
  let handled = false;
  socket.on('data', chunk => {
    raw += chunk.toString();
    if (!handled && raw.includes('\n')) {
      handled = true;
      handleSocketRequest(socket, raw.trim());
    }
  });
});

server.listen(socketPath, () => {
  try { fs.chmodSync(socketPath, 0o600); } catch (_) {}
});
`
}

func providerPoolClaudeStreamClientScript() string {
	return `const net = require('net');
const socketPath = process.argv[1];
const prompt = process.argv[2] || '';
const turnID = process.argv[3] || '';
const type = turnID === 'ping' ? 'ping' : 'prompt';
const socket = net.createConnection(socketPath);
let raw = '';
socket.on('connect', () => {
  socket.write(JSON.stringify({ type, prompt, prompt_turn_id: turnID }) + '\n');
});
socket.on('data', chunk => raw += chunk.toString());
socket.on('end', () => {
  process.stdout.write((raw.trim() || '{"ok":false,"error":"empty stream worker response"}') + '\n');
});
socket.on('error', err => {
  process.stdout.write(JSON.stringify({ ok: false, error: err.message }) + '\n');
  process.exitCode = 1;
});
`
}

func elapsedMillis(startedAt time.Time) int64 {
	return time.Since(startedAt).Milliseconds()
}

func promptTimings(timings map[string]int64, startedAt time.Time) map[string]int64 {
	out := map[string]int64{}
	for key, value := range timings {
		out[key] = value
	}
	out["total_ms"] = elapsedMillis(startedAt)
	return out
}

func mergePromptTimings(dst map[string]int64, src any) {
	switch typed := src.(type) {
	case map[string]int64:
		for key, value := range typed {
			dst[key] = value
		}
	case map[string]any:
		for key, value := range typed {
			switch v := value.(type) {
			case int64:
				dst[key] = v
			case int:
				dst[key] = int64(v)
			case float64:
				dst[key] = int64(v)
			}
		}
	}
}

func providerPoolPromptExecutionTimeout(provider string, fallback time.Duration) time.Duration {
	envKey := "DORKPIPE_PROVIDER_POOL_" + strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(provider), "-", "_")) + "_PROMPT_TIMEOUT"
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallback
	}
	if strings.EqualFold(raw, "0") || strings.EqualFold(raw, "off") || strings.EqualFold(raw, "false") {
		return 0
	}
	if duration, err := time.ParseDuration(raw); err == nil {
		if duration < 0 {
			return 0
		}
		return duration
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}
	return fallback
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
	if path, err := providerPoolCodexCLIPath(); err == nil && strings.TrimSpace(path) != "" {
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

func providerPoolCodexCLIPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("CODEX_CLI_PATH")); path != "" && fileExists(path) {
		return path, nil
	}
	for _, home := range providerPoolHomeDirs() {
		for _, candidate := range providerPoolCodexBundledCandidates(home) {
			if fileExists(candidate) {
				return candidate, nil
			}
		}
	}
	return exec.LookPath("codex")
}

func providerPoolCodexBundledCandidates(home string) []string {
	if strings.TrimSpace(home) == "" {
		return nil
	}
	patterns := []string{
		filepath.Join(home, ".vscode", "extensions", "openai.chatgpt-*", "bin", "windows-x86_64", "codex.exe"),
		filepath.Join(home, "AppData", "Local", "OpenAI", "Codex", "bin", "*", "codex.exe"),
	}
	var candidates []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		sort.Slice(matches, func(i, j int) bool {
			left, leftErr := os.Stat(matches[i])
			right, rightErr := os.Stat(matches[j])
			if leftErr == nil && rightErr == nil {
				return left.ModTime().After(right.ModTime())
			}
			return matches[i] > matches[j]
		})
		candidates = append(candidates, matches...)
	}
	return candidates
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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

func providerPoolClaudeContainerNameCandidates(workdir string) []string {
	var names []string
	for _, hash := range providerPoolWorkdirHashCandidates(workdir) {
		names = append(names, "dorkpipe-provider-pool-claude-"+hash)
	}
	return uniqueNonEmpty(names)
}

func providerPoolWorkdirHash(workdir string) string {
	canonical := filepath.Clean(strings.TrimSpace(workdir))
	if canonical == "" {
		canonical = "."
	}
	windowsPath := providerPoolLooksWindowsPath(canonical)
	if abs, err := filepath.Abs(canonical); err == nil {
		canonical = abs
	}
	if runtime.GOOS == "windows" || windowsPath {
		canonical = strings.ToLower(filepath.Clean(canonical))
	}
	sum := sha1.Sum([]byte(canonical))
	return hex.EncodeToString(sum[:])[:10]
}

func providerPoolWorkdirHashCandidates(workdir string) []string {
	var hashes []string
	for _, canonical := range providerPoolWorkdirCanonicalCandidates(workdir) {
		sum := sha1.Sum([]byte(canonical))
		hashes = append(hashes, hex.EncodeToString(sum[:])[:10])
	}
	hashes = append(hashes, providerPoolWorkdirHash(workdir))
	return uniqueNonEmpty(hashes)
}

func providerPoolWorkdirCanonicalCandidates(workdir string) []string {
	trimmed := strings.TrimSpace(workdir)
	if trimmed == "" {
		trimmed = "."
	}
	cleaned := filepath.Clean(trimmed)
	var candidates []string
	candidates = append(candidates, cleaned)
	if abs, err := filepath.Abs(cleaned); err == nil {
		candidates = append(candidates, abs)
	}
	for _, value := range append([]string{}, candidates...) {
		candidates = append(candidates, filepath.ToSlash(value))
		if runtime.GOOS == "windows" || providerPoolLooksWindowsPath(value) {
			candidates = append(candidates, strings.ToLower(filepath.Clean(value)))
			candidates = append(candidates, strings.ToLower(filepath.ToSlash(value)))
		}
	}
	return uniqueNonEmpty(candidates)
}

func providerPoolLooksWindowsPath(path string) bool {
	path = strings.TrimSpace(path)
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.Contains(path, "\\")
}

func providerPoolClaudeImageReady(workdir string) bool {
	_, ok := providerPoolClaudeImageRef(workdir)
	return ok
}

func providerPoolClaudeImageRef(workdir string) (string, bool) {
	candidates := []string{"dockpipe-claude:latest"}
	if version := providerPoolRepoVersion(workdir); version != "" {
		candidates = append(candidates, "dockpipe-claude:"+version)
	}
	for _, candidate := range candidates {
		_, _, code, err := runCommandCapture(context.Background(), "", "docker", "image", "inspect", candidate)
		if err == nil && code == 0 {
			return candidate, true
		}
	}
	return "", false
}

func providerPoolRepoVersion(workdir string) string {
	for _, name := range []string{"VERSION", "version"} {
		path := filepath.Join(workdir, name)
		raw, err := os.ReadFile(path)
		if err == nil {
			if value := strings.TrimSpace(string(raw)); value != "" {
				return value
			}
		}
	}
	return ""
}

func providerPoolEnsureClaudeImage(ctx context.Context, workdir string) error {
	providerPoolClaudeImageBuild.Lock()
	defer providerPoolClaudeImageBuild.Unlock()

	if providerPoolClaudeImageReady(workdir) {
		return nil
	}

	dockpipeBin, err := providerPoolDockpipeBin(workdir)
	if err != nil {
		return fmt.Errorf("locate dockpipe build command for Claude image: %w", err)
	}

	buildCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	stdout, stderr, code, runErr := runCommandCapture(buildCtx, workdir, dockpipeBin, "build", "--workdir", workdir, "--no-source-builds")
	if runErr != nil {
		return fmt.Errorf("build dockpipe-claude image: %w", runErr)
	}
	if code != 0 {
		text := strings.TrimSpace(firstNonEmptyString(stderr, stdout, fmt.Sprintf("dockpipe build exited %d", code)))
		return fmt.Errorf("build dockpipe-claude image: %s", text)
	}
	if !providerPoolClaudeImageReady(workdir) {
		return errors.New("dockpipe build completed but no local dockpipe-claude image tag is available")
	}
	return nil
}

func providerPoolDockpipeBin(workdir string) (string, error) {
	candidates := []string{}
	if value := strings.TrimSpace(os.Getenv("DOCKPIPE_BIN")); value != "" {
		candidates = append(candidates, value)
	}
	if workdir != "" {
		candidates = append(candidates,
			filepath.Join(workdir, "src", "bin", "dockpipe.exe"),
			filepath.Join(workdir, "src", "bin", "dockpipe"),
		)
	}
	if value, err := exec.LookPath("dockpipe"); err == nil && strings.TrimSpace(value) != "" {
		candidates = append(candidates, value)
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("dockpipe binary not found")
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
	args := []string{
		"run", "-d",
		"--name", containerName,
		"--label", "com.dockpipe.provider-pool=true",
		"--label", "com.dockpipe.provider-pool.provider=claude",
		"--label", "com.dockpipe.provider-pool.workdir-hash=" + providerPoolWorkdirHash(workdir),
		"-w", "/work",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/work", workdir),
	}
	if path, ok := stringFromMap(providerPoolClaudeAuthStatus(), "auth_dir"); ok {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/dockpipe-auth/claude,readonly", path))
	}
	if path, ok := stringFromMap(providerPoolClaudeAuthStatus(), "config_file"); ok {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/dockpipe-auth/claude-config/.claude.json,readonly", path))
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
		}
	}
	imageRef, ok := providerPoolClaudeImageRef(workdir)
	if !ok {
		return false, errors.New("dockpipe-claude image is unavailable after build/materialization")
	}
	args = append(args, imageRef, "bash", "-lc", providerPoolClaudeWarmBootstrapScript())
	_, stderr, code, err := runCommandCapture(ctx, workdir, "docker", args...)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("start guarded Claude worker: %s", strings.TrimSpace(stderr))
	}
	return true, nil
}

func providerPoolClaudeWarmBootstrapScript() string {
	return `set -euo pipefail
if id node >/dev/null 2>&1; then
  install -d -o node -g node /home/node/.claude
  if [[ -d /dockpipe-auth/claude ]]; then
    for name in .credentials.json .last-cleanup history.jsonl ide mcp-needs-auth-cache.json plans plugins policy-limits.json projects remote-settings.json session-env sessions settings.json shell-snapshots skills; do
      if [[ -e "/dockpipe-auth/claude/${name}" ]]; then
        cp -a "/dockpipe-auth/claude/${name}" /home/node/.claude/ 2>/dev/null || true
      fi
    done
    chown -R node:node /home/node/.claude 2>/dev/null || true
    chmod -R u+rwX /home/node/.claude 2>/dev/null || true
  fi
  if [[ -f /dockpipe-auth/claude-config/.claude.json ]]; then
    cp /dockpipe-auth/claude-config/.claude.json /home/node/.claude.json 2>/dev/null || true
    chown node:node /home/node/.claude.json 2>/dev/null || true
    chmod u+rw /home/node/.claude.json 2>/dev/null || true
  fi
fi
trap 'exit 0' TERM INT
while :; do sleep 3600; done`
}

func acquireProviderPoolLease(ctx context.Context, workdir string, provider providerPoolProvider, sessionID, role, workflowRunID, nodeID string, maxActive int, queueTimeout time.Duration) (func(), bool, error) {
	deadline := time.Now().Add(queueTimeout)
	for {
		release, queued, err := tryAcquireProviderPoolLease(workdir, provider, sessionID, role, workflowRunID, nodeID, maxActive)
		if err != nil || !queued || queueTimeout <= 0 {
			return release, queued, err
		}
		if !time.Now().Before(deadline) {
			return nil, true, nil
		}
		sleepFor := 250 * time.Millisecond
		if remaining := time.Until(deadline); remaining < sleepFor {
			sleepFor = remaining
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(sleepFor):
		}
	}
}

func tryAcquireProviderPoolLease(workdir string, provider providerPoolProvider, sessionID, role, workflowRunID, nodeID string, maxActive int) (func(), bool, error) {
	leasesDir, err := statepaths.ProviderPoolLeasesDir(workdir)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(leasesDir, 0o755); err != nil {
		return nil, false, err
	}
	active, err := countProviderPoolLeases(workdir, provider.ID)
	if err != nil {
		return nil, false, err
	}
	if active >= effectiveProviderPoolMaxActive(provider, maxActive) {
		return nil, true, nil
	}
	leaseID := providerPoolLeaseID(provider.ID, sessionID, role, workflowRunID, nodeID)
	leasePath := filepath.Join(leasesDir, provider.ID+"-"+leaseID+".json")
	file, err := os.OpenFile(leasePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, true, nil
		}
		return nil, false, err
	}
	payload := providerPoolLease{
		Provider:      provider.ID,
		LeaseID:       leaseID,
		SessionID:     strings.TrimSpace(sessionID),
		Role:          strings.TrimSpace(role),
		WorkflowRunID: strings.TrimSpace(workflowRunID),
		NodeID:        strings.TrimSpace(nodeID),
		Workdir:       workdir,
		StartedAt:     time.Now().UTC().Format(time.RFC3339Nano),
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

func effectiveProviderPoolMaxActive(provider providerPoolProvider, requested int) int {
	effective := max(1, provider.Pool.MaxActive)
	if requested > 0 && requested < effective {
		effective = requested
	}
	return effective
}

func providerPoolLeaseID(parts ...string) string {
	seed := strings.Join(parts, "\x00") + fmt.Sprintf("\x00%d\x00%d", os.Getpid(), time.Now().UnixNano())
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:16]
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
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		provider := strings.ToLower(strings.TrimSpace(providerID))
		if strings.EqualFold(stem, provider) || strings.HasPrefix(strings.ToLower(stem), provider+"-") {
			count++
		}
	}
	return count, nil
}

func removeProviderPoolLease(workdir, providerID string) error {
	leasesDir, err := statepaths.ProviderPoolLeasesDir(workdir)
	if err != nil {
		return err
	}
	provider := strings.ToLower(strings.TrimSpace(providerID))
	entries, err := os.ReadDir(leasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if strings.EqualFold(stem, provider) || strings.HasPrefix(strings.ToLower(stem), provider+"-") {
			if err := os.Remove(filepath.Join(leasesDir, entry.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
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
