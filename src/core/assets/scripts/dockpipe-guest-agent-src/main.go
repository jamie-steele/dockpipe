//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
)

type config struct {
	Port           int    `json:"port"`
	BindAddress    string `json:"bind_address"`
	StatePath      string `json:"state_path"`
	ServiceAccount string `json:"service_account"`
}

type runRequest struct {
	Command string `json:"command"`
}

type clipboardPayload struct {
	Text string `json:"text"`
}

type runResult struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMS int64  `json:"duration_ms"`
}

type agentState struct {
	Status            string `json:"status"`
	StartedAt         string `json:"started_at"`
	ServiceAccount    string `json:"service_account"`
	MachineName       string `json:"machine_name"`
	LastSeen          string `json:"last_seen,omitempty"`
	LastRunExitCode   int    `json:"last_run_exit_code,omitempty"`
	LastRunDurationMS int64  `json:"last_run_duration_ms,omitempty"`
}

type agent struct {
	cfg       config
	root      string
	log       *log.Logger
	startedAt string
	mu        sync.Mutex
}

func main() {
	var (
		_             = flag.Bool("service", false, "run as background service/task")
		_             = flag.Bool("Service", false, "run as background service/task")
		configPath    = flag.String("config", "", "path to config.json")
		configPathAlt = flag.String("ConfigPath", "", "path to config.json")
	)
	flag.Parse()

	selectedConfigPath := strings.TrimSpace(*configPath)
	if selectedConfigPath == "" {
		selectedConfigPath = strings.TrimSpace(*configPathAlt)
	}

	agentRoot := resolveAgentRoot(selectedConfigPath)
	if err := os.MkdirAll(agentRoot, 0o755); err != nil {
		fatalf("create agent root: %v", err)
	}

	logFile, err := os.OpenFile(filepath.Join(agentRoot, "agent.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatalf("open agent log: %v", err)
	}
	defer logFile.Close()

	cfg, err := loadConfig(selectedConfigPath, agentRoot)
	if err != nil {
		fatalf("load config: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	if strings.TrimSpace(cfg.BindAddress) == "" {
		cfg.BindAddress = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 47831
	}
	if strings.TrimSpace(cfg.StatePath) == "" {
		cfg.StatePath = filepath.Join(agentRoot, "state.json")
	}
	if strings.TrimSpace(cfg.ServiceAccount) == "" {
		cfg.ServiceAccount = os.Getenv("USERNAME")
		if strings.TrimSpace(cfg.ServiceAccount) == "" {
			cfg.ServiceAccount = "LocalSystem"
		}
	}

	startedAt := time.Now().Format(time.RFC3339Nano)
	logger := log.New(logFile, "", 0)
	a := &agent{
		cfg:       cfg,
		root:      agentRoot,
		log:       logger,
		startedAt: startedAt,
	}

	a.writeLog(fmt.Sprintf("starting dockpipe guest agent on %s:%d", cfg.BindAddress, cfg.Port))
	if err := a.saveState(agentState{
		Status:         "ready",
		StartedAt:      startedAt,
		ServiceAccount: cfg.ServiceAccount,
		MachineName:    hostname,
	}); err != nil {
		a.writeLog(fmt.Sprintf("save state failed: %v", err))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleRoot)
	mux.HandleFunc("/health", a.handleRoot)
	mux.HandleFunc("/state", a.handleState)
	mux.HandleFunc("/run", a.handleRun)
	mux.HandleFunc("/clipboard", a.handleClipboard)
	mux.HandleFunc("/shutdown", a.handleShutdown)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.writeLog(fmt.Sprintf("http server failed: %v", err))
		fatalf("dockpipe guest agent failed: %v", err)
	}
}

func resolveAgentRoot(configPath string) string {
	if strings.TrimSpace(configPath) != "" {
		return filepath.Dir(configPath)
	}
	programData := os.Getenv("ProgramData")
	if strings.TrimSpace(programData) == "" {
		programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
	}
	return filepath.Join(programData, "DockPipe", "GuestAgent")
}

func loadConfig(configPath, root string) (config, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(root, "config.json")
	}
	var cfg config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{
				Port:           47831,
				BindAddress:    "127.0.0.1",
				StatePath:      filepath.Join(root, "state.json"),
				ServiceAccount: "LocalSystem",
			}, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (a *agent) writeLog(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log.Printf("%s %s", time.Now().Format(time.RFC3339Nano), message)
}

func (a *agent) saveState(state agentState) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	state.LastSeen = time.Now().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.cfg.StatePath, data, 0o644)
}

func (a *agent) baseState() agentState {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return agentState{
		Status:         "ready",
		StartedAt:      a.startedAt,
		ServiceAccount: a.cfg.ServiceAccount,
		MachineName:    hostname,
	}
}

func (a *agent) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":            "dockpipe-guest-agent",
		"status":          "ready",
		"started_at":      a.startedAt,
		"service_account": a.cfg.ServiceAccount,
		"machine_name":    a.baseState().MachineName,
	})
}

func (a *agent) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	data, err := os.ReadFile(a.cfg.StatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, a.baseState())
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, decoded)
}

func (a *agent) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()

	var body runRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if strings.TrimSpace(body.Command) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "command is required"})
		return
	}

	a.writeLog("run request received")
	result, err := runPowerShell(body.Command)
	if err != nil {
		a.writeLog(fmt.Sprintf("run request failed: %v", err))
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	state := a.baseState()
	state.LastRunExitCode = result.ExitCode
	state.LastRunDurationMS = result.DurationMS
	if err := a.saveState(state); err != nil {
		a.writeLog(fmt.Sprintf("save state after run failed: %v", err))
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *agent) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	a.writeLog("shutdown request received")
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "action": "shutdown"})

	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command("shutdown.exe", "/s", "/t", "0", "/f")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Start()
	}()
}

func (a *agent) handleClipboard(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		text, err := readWindowsClipboard()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, clipboardPayload{Text: text})
	case http.MethodPost:
		defer r.Body.Close()
		var body clipboardPayload
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if err := writeWindowsClipboard(body.Text); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func runPowerShell(command string) (runResult, error) {
	start := time.Now()
	encoded := encodePowerShell(command)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := runResult{
		ExitCode:   0,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMS: time.Since(start).Milliseconds(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func readWindowsClipboard() (string, error) {
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		"$ProgressPreference='SilentlyContinue'; try { $text = Get-Clipboard -Raw -Format Text -ErrorAction Stop } catch { $text = '' }; if ($null -eq $text) { $text = '' }; [Console]::Out.Write([string]$text)",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}

func writeWindowsClipboard(text string) error {
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		"$ProgressPreference='SilentlyContinue'; $text = [Console]::In.ReadToEnd(); Set-Clipboard -Value $text",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}

func encodePowerShell(script string) string {
	utf16Data := utf16.Encode([]rune(script))
	buf := make([]byte, len(utf16Data)*2)
	for i, r := range utf16Data {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
