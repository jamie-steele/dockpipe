package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	reEnvExpand         = regexp.MustCompile(`\$\{([^}:]+)(:-([^}]*))?\}`)
	reResponseHash      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	reCommandNotFound   = regexp.MustCompile(`(?i)\b(exec|command): .* not found\b`)
	reBadPlanNarration  = regexp.MustCompile(`\bI will (outline|create|write|complete)\b`)
	reArtifactMimic     = regexp.MustCompile(`(?im)^\s*(#{1,6}\s*)?(Task Artifact|Lane Selection|Worker Result Artifact|Merge Result|Final Report Checklist)\s*:`)
	reSampleJSON        = regexp.MustCompile("(?i)```json\\s*\\{")
	reImplChatter       = regexp.MustCompile(`(?i)\b(files? (were|modified|touched)|validations? run|generated artifacts?)\b`)
	reBoundary1         = regexp.MustCompile(`(?i)\bworkflow declares (?:its )?limitations in concurrency control\b`)
	reBoundary2         = regexp.MustCompile(`(?i)\bworkflow (?:does not|should not|is not responsible to) own concurrency\b`)
	reBoundary3         = regexp.MustCompile(`(?i)\bconcurrency (?:is|should be) (?:owned|managed) by worker results\b`)
	reShape1            = regexp.MustCompile(`(?im)^\s*Here (?:are|is)\b`)
	reShape2            = regexp.MustCompile(`(?im)^\s*(?:Note|Please note)\s*:`)
	reShape3            = regexp.MustCompile(`(?i)\bcould not be completed due to lack of information\b`)
	reShape4            = regexp.MustCompile(`(?i)\badheres to (?:the )?(?:specified )?formatting\b`)
	reShape5            = regexp.MustCompile(`(?im)^###\s+repo_shape\s*$`)
	reShape6            = regexp.MustCompile(`(?i)\buncertainties remain\b`)
	reShape7            = regexp.MustCompile(`(?i)\b(?:lane scores|confidence values) should be cited\b`)
	reExactBullets      = regexp.MustCompile(`(?i)Return exactly (\w+) bullets? and no headings?\.`)
	reBulletPrefix      = regexp.MustCompile(`(?i)Bullet\s+(\d+)\s+must\s+start\s+with\s+"([^"\n]+)"`)
	reBulletMarker      = regexp.MustCompile(`^\s*(?:[-*+]\s+|•\s*)`)
	reSafeCompareSuffix = regexp.MustCompile(`[^A-Za-z0-9_]+`)
	reGuestDocPath      = regexp.MustCompile(`/(?:work|DesignNotes)/[A-Za-z0-9._/\-]+\.(?:md|ya?ml)`)
	reMarkdownLink      = regexp.MustCompile(`\[[^\]]*\]\(([^)#]+)(?:#[^)]+)?\)`)
	reValidationRemoved = regexp.MustCompile("(?im)^- \\*\\*Removed `([^`]+)`")
)

type trainingEntry struct {
	Samples         int     `json:"samples"`
	ConfidenceTotal float64 `json:"confidence_total"`
	LiveSuccesses   int     `json:"live_successes"`
	BudgetHalts     int     `json:"budget_halts"`
	AvgConfidence   float64 `json:"avg_confidence"`
	LiveSuccessRate float64 `json:"live_success_rate"`
	BudgetHaltRate  float64 `json:"budget_halt_rate"`
	Active          bool    `json:"active"`
	Adjustment      float64 `json:"adjustment"`
}

type laneCandidate struct {
	Lane     map[string]any `json:"lane"`
	Score    float64        `json:"score"`
	Reason   []string       `json:"reason"`
	Training trainingEntry  `json:"training"`
}

type taskClass struct {
	Name      string `json:"name"`
	Authority string `json:"authority"`
}

type localHostProfile struct {
	MemoryGB     int    `json:"memory_gb"`
	CPUCores     int    `json:"cpu_cores"`
	Acceleration string `json:"acceleration"`
	Tier         string `json:"tier"`
}

var seededWorkerProfiles = map[string]map[string]any{
	"codex": {
		"preferred_resolver_hint": "codex",
		"model": map[string]any{
			"provider": "codex",
		},
	},
	"claude": {
		"preferred_resolver_hint": "claude",
		"model": map[string]any{
			"provider": "claude",
		},
	},
	"ollama": {
		"preferred_resolver_hint": "ollama",
		"model": map[string]any{
			"provider": "ollama",
		},
	},
}

type schedulerTask struct {
	ID            string         `json:"id"`
	BaseTaskID    string         `json:"base_task_id"`
	Comparison    map[string]any `json:"comparison"`
	DependsOn     []string       `json:"depends_on"`
	Provider      string         `json:"provider"`
	Model         string         `json:"model"`
	OutputPath    string         `json:"output_path"`
	ReuseExisting bool           `json:"reuse_existing"`
}

func Run(args []string, env map[string]string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: orchestrate-helper <subcommand> [args]")
	}
	switch args[0] {
	case "usage-number":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper usage-number <cloud-usage.json> <key>")
		}
		payload := readJSONMap(args[1])
		fmt.Fprintln(stdout, intFromAny(payload[args[2]]))
		return nil
	case "provider-usage-number":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper provider-usage-number <cloud-usage.json> <provider> <field>")
		}
		payload := readJSONMap(args[1])
		providers := mapValue(payload["providers"])
		provider := mapValue(providers[args[2]])
		fmt.Fprintln(stdout, intFromAny(provider[args[3]]))
		return nil
	case "task-id-from-workflow":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper task-id-from-workflow <workflow.yml> <step-id>")
		}
		taskID, err := taskIDFromWorkflow(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, taskID)
		return nil
	case "task-env":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper task-env <task.json>")
		}
		return emitTaskEnv(args[1], stdout)
	case "resolve-target-path":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper resolve-target-path <root> <target>")
		}
		targetPath, _, err := resolveApplyTargetPath(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, targetPath)
		return nil
	case "worker-lease-env":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper worker-lease-env <lease.json>")
		}
		return emitWorkerLeaseEnv(args[1], stdout)
	case "required-auth-providers":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper required-auth-providers <tasks-dir>")
		}
		return emitRequiredAuthProviders(args[1], stdout)
	case "task-model":
		model := ""
		if parsed, err := decodeJSONMapString(env["TASK_MODEL_JSON"]); err == nil {
			model = stringValue(parsed["model"])
		}
		fmt.Fprintln(stdout, model)
		return nil
	case "ollama-chat-request":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper ollama-chat-request <model> <prompt.md> <request.json>")
		}
		return writeOllamaChatRequest(args[1], args[2], args[3])
	case "ollama-chat-response":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper ollama-chat-response <response.json> <response.md>")
		}
		return writeOllamaChatResponse(args[1], args[2])
	case "append-dependency-context":
		if len(args) != 7 {
			return errors.New("usage: orchestrate-helper append-dependency-context <prompt.md> <tasks-dir> <depends-on.json> <max-bytes> <total-max-bytes> <prefer-planner>")
		}
		return appendDependencyContext(args[1], args[2], args[3], intFromString(args[4], 5000), intFromString(args[5], 12000), boolString(args[6]))
	case "validate-live-response":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper validate-live-response <response.md>")
		}
		if !liveResponseIsValid(args[1]) {
			return errors.New("invalid live response")
		}
		return nil
	case "write-task-result":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper write-task-result <result.json>")
		}
		return writeTaskResult(args[1], env)
	case "merge-result-paths":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper merge-result-paths <graph.json> <tasks-dir> <all|main|planning>")
		}
		paths, err := mergeResultPaths(args[1], args[2], args[3])
		if err != nil {
			return err
		}
		for _, path := range paths {
			fmt.Fprintln(stdout, path)
		}
		return nil
	case "merge-plan-env":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper merge-plan-env <plan.json>")
		}
		return emitMergePlanEnv(args[1], stdout)
	case "merge-build-result":
		if len(args) < 3 {
			return errors.New("usage: orchestrate-helper merge-build-result <out.json> <main-result.json>... [--planning <planning-result.json>...]")
		}
		return buildMergeResult(args[1], args[2:])
	case "merge-render-final":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper merge-render-final <merge-result.json> <final.md> <tasks-dir>")
		}
		return renderMergeFinal(args[1], args[2], args[3], env)
	case "verify-plan-env":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper verify-plan-env <plan.json>")
		}
		return emitVerifyPlanEnv(args[1], stdout)
	case "verify-summary-env":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper verify-summary-env <merge.json>")
		}
		return emitVerifySummaryEnv(args[1], stdout)
	case "verify-heuristics":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper verify-heuristics <merge.json> <tasks-dir> <issues.json>")
		}
		return emitVerifyHeuristics(args[1], args[2], args[3], stdout)
	case "verify-apply-coherence":
		if len(args) != 5 {
			return errors.New("usage: orchestrate-helper verify-apply-coherence <root> <artifact-root> <plan.json> <issues.json>")
		}
		return emitVerifyApplyCoherence(args[1], args[2], args[3], args[4], stdout)
	case "apply-results":
		if len(args) != 6 {
			return errors.New("usage: orchestrate-helper apply-results <root> <artifact-root> <plan.json> <approval.md> <result.json>")
		}
		return applyResults(args[1], args[2], args[3], args[4], args[5])
	case "plan":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper plan <workflow.yml> <step-id>")
		}
		return planOrchestration(args[1], args[2], env)
	case "run-tasks":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper run-tasks <graph.json> <runner.sh>")
		}
		return runTasks(args[1], args[2], env, stderr)
	case "optimizer-result-status":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper optimizer-result-status <result.json>")
		}
		fmt.Fprintln(stdout, strings.TrimSpace(stringValue(readJSONMapOptional(args[1])["status"])))
		return nil
	case "optimizer-propose-invalid":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper optimizer-propose-invalid <propose-result.json>")
		}
		payload := readJSONMapOptional(args[1])
		invalid := strings.TrimSpace(stringValue(payload["status"])) == "review" &&
			strings.TrimSpace(stringValue(payload["validation_error"])) != "" &&
			len(listValue(payload["changed_files"])) > 0
		if invalid {
			fmt.Fprintln(stdout, "true")
		} else {
			fmt.Fprintln(stdout, "false")
		}
		return nil
	case "optimize-action":
		if len(args) != 8 {
			return errors.New("usage: orchestrate-helper optimize-action <action> <root> <target-dir> <optimizer-dir> <orch-root> <approval.md> <result.json>")
		}
		return optimizeAction(args[1], args[2], args[3], args[4], args[5], args[6], args[7], env)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func taskIDFromWorkflow(path, stepID string) (string, error) {
	workflow := readYAMLMap(path)
	for _, raw := range listValue(workflow["steps"]) {
		step := mapValue(raw)
		if stringValue(step["id"]) != stepID {
			continue
		}
		return stringValue(mapValue(step["agent"])["task_id"]), nil
	}
	return "", nil
}

func emitTaskEnv(path string, stdout io.Writer) error {
	task := readJSONMap(path)
	lane := mapValue(task["lane"])
	mapping := map[string]string{
		"TASK_BASE_ID":                 fallbackString(stringValue(task["base_id"]), stringValue(task["id"])),
		"TASK_WORKER_PROFILE":          stringValue(task["worker"]),
		"TASK_WORKER_POLICY_MODE":      workerPolicyMode(task),
		"TASK_WORK_MODE":               stringValue(task["work_mode"]),
		"TASK_OUTPUT_PATH":             stringValue(task["output_path"]),
		"TASK_COMPARISON_JSON":         mustJSON(task["comparison"], map[string]any{"enabled": false}),
		"TASK_RESOLVER_HINT":           fallbackString(stringValue(task["resolver_hint"]), "auto"),
		"TASK_REQUESTED_RESOLVER_HINT": fallbackString(stringValue(task["requested_resolver_hint"]), fallbackString(stringValue(task["resolver_hint"]), "auto")),
		"TASK_LANE_JSON":               mustJSON(lane, map[string]any{}),
		"TASK_LANE_ID":                 stringValue(lane["lane_id"]),
		"TASK_GOAL":                    stringValue(task["goal"]),
		"TASK_EXPECTED_OUTPUT":         stringValue(task["expected_output"]),
		"TASK_INPUTS_JSON":             mustJSON(task["inputs"], []any{}),
		"TASK_CLAIMS_JSON":             mustJSON(task["claims"], []any{}),
		"TASK_CITATIONS_JSON":          mustJSON(fallbackAny(task["citations"], task["inputs"]), []any{}),
		"TASK_MAX_CLOUD_TOKENS":        strconv.Itoa(intFromAny(task["max_cloud_tokens"])),
		"TASK_MODEL_JSON":              mustJSON(task["model"], map[string]any{}),
		"TASK_DEPENDS_ON_JSON":         mustJSON(task["depends_on"], []any{}),
	}
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(stdout, "%s=%s\n", key, shellQuote(mapping[key]))
	}
	return nil
}

func emitWorkerLeaseEnv(path string, stdout io.Writer) error {
	lease := readJSONMap(path)
	mapping := map[string]string{
		"LEASE_ID":          stringValue(lease["lease_id"]),
		"LEASE_WORKER_ID":   stringValue(lease["worker_id"]),
		"LEASE_MODE":        stringValue(lease["mode"]),
		"LEASE_VOLUME":      stringValue(lease["volume"]),
		"LEASE_BASE_VOLUME": stringValue(lease["base_volume"]),
		"LEASE_STATUS":      stringValue(lease["status"]),
	}
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(stdout, "%s=%s\n", key, shellQuote(mapping[key]))
	}
	return nil
}

func emitRequiredAuthProviders(tasksDir string, stdout io.Writer) error {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var providers []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		task := readJSONMapOptional(filepath.Join(tasksDir, entry.Name(), "task.json"))
		if len(task) == 0 {
			continue
		}
		if workerPolicyMode(task) != "require" {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(stringValue(mapValue(task["lane"])["provider"])))
		switch provider {
		case "codex", "claude":
		default:
			continue
		}
		if seen[provider] {
			continue
		}
		seen[provider] = true
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		fmt.Fprintln(stdout, provider)
	}
	return nil
}

func appendDependencyContext(promptPath, tasksDir, dependsJSON string, maxBytes, totalMaxBytes int, preferPlanner bool) error {
	var dependsOn []string
	_ = json.Unmarshal([]byte(dependsJSON), &dependsOn)
	if len(dependsOn) == 0 {
		return nil
	}
	if preferPlanner {
		for _, dep := range dependsOn {
			if dep == "planner_brain" {
				dependsOn = []string{"planner_brain"}
				break
			}
		}
	}
	raw, err := os.ReadFile(promptPath)
	if err != nil {
		return nil
	}
	prompt := string(raw)
	marker := "Dependency context from completed upstream tasks:"
	if strings.Contains(prompt, marker) {
		return nil
	}
	remaining := totalMaxBytes
	sections := []string{}
	for _, dep := range dependsOn {
		if remaining <= 0 {
			break
		}
		responsePath := filepath.Join(tasksDir, dep, "response.md")
		text, err := os.ReadFile(responsePath)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(text))
		if trimmed == "" {
			continue
		}
		snippetBytes := truncateUTF8([]byte(trimmed), minInt(maxBytes, remaining))
		snippet := strings.TrimSpace(string(snippetBytes))
		if snippet == "" {
			continue
		}
		remaining -= len([]byte(snippet))
		sections = append(sections, "### "+dep, "", snippet)
		if len([]byte(trimmed)) > len(snippetBytes) {
			sections = append(sections, "[truncated]")
		}
		sections = append(sections, "")
	}
	if len(sections) == 0 {
		return nil
	}
	addition := strings.Join([]string{
		marker,
		"Use this as planning guidance from earlier bounded workers. Do not repeat it verbatim.",
		strings.TrimRight(strings.Join(sections, "\n"), "\n"),
	}, "\n\n")
	return os.WriteFile(promptPath, []byte(strings.TrimRight(prompt, "\n")+"\n\n"+strings.TrimRight(addition, "\n")+"\n"), 0o644)
}

func liveResponseIsValid(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := strings.TrimSpace(string(raw))
	if len(text) < 40 {
		return false
	}
	if reResponseHash.MatchString(text) {
		return false
	}
	return !reCommandNotFound.MatchString(text)
}

func writeTaskResult(path string, env map[string]string) error {
	payload := map[string]any{
		"task_id":                 env["task_id"],
		"base_task_id":            fallbackString(env["TASK_BASE_ID"], env["task_id"]),
		"comparison":              decodeJSONAny(env["TASK_COMPARISON_JSON"], map[string]any{"enabled": false}),
		"status":                  env["status"],
		"provider_requested":      fallbackString(env["resolver_hint"], "auto"),
		"provider_actual":         env["provider"],
		"model":                   env["selected_model"],
		"lane_id":                 env["lane_id"],
		"lane_selection":          decodeJSONAny(env["TASK_LANE_JSON"], map[string]any{}),
		"used_live_model":         boolString(env["used_live_model"]),
		"budget_halt":             boolString(env["budget_halt"]),
		"estimated_input_tokens":  intFromString(env["estimated_input_tokens"], 0),
		"estimated_output_tokens": intFromString(env["estimated_output_tokens"], 0),
		"estimated_total_tokens":  intFromString(env["estimated_total_tokens"], 0),
		"started_at":              env["task_started_at"],
		"finished_at":             env["task_finished_at"],
		"duration_ms":             intFromString(env["duration_ms"], 0),
		"summary":                 env["summary"],
		"claims":                  decodeJSONAny(env["TASK_CLAIMS_JSON"], []any{}),
		"artifacts":               []string{fmt.Sprintf("tasks/%s/task.json", env["task_id"]), fmt.Sprintf("tasks/%s/prompt.md", env["task_id"]), fmt.Sprintf("tasks/%s/response.md", env["task_id"])},
		"citations":               decodeJSONAny(env["TASK_CITATIONS_JSON"], []any{}),
		"confidence":              floatFromString(env["confidence"], 0),
		"issues":                  decodeJSONAny(env["issues_json"], []any{}),
		"next_actions":            decodeJSONAny(env["next_actions_json"], []any{}),
	}
	return writeJSONFile(path, payload)
}

func writeOllamaChatRequest(model, promptPath, outPath string) error {
	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(model) == "" {
		model = "llama3.2"
	}
	return writeJSONFile(outPath, map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": string(prompt)},
		},
		"stream": false,
	})
}

func writeOllamaChatResponse(responsePath, outPath string) error {
	payload := readJSONMap(responsePath)
	message := mapValue(payload["message"])
	content := strings.TrimSpace(stringValue(message["content"]))
	if content == "" {
		return errors.New("ollama response did not include message.content")
	}
	return os.WriteFile(outPath, []byte(content+"\n"), 0o644)
}

func mergeResultPaths(graphPath, tasksDir, mode string) ([]string, error) {
	graph := readJSONMap(graphPath)
	var out []string
	for _, raw := range listValue(graph["tasks"]) {
		task := mapValue(raw)
		workerType := fallbackString(stringValue(task["worker_type"]), "analysis")
		include := false
		switch mode {
		case "all":
			include = workerType != "merge" && workerType != "verify"
		case "main":
			include = workerType != "merge" && workerType != "verify" && workerType != "planning" && workerType != "scout"
		case "planning":
			include = workerType == "planning" || workerType == "scout"
		default:
			return nil, fmt.Errorf("unknown merge result path mode %q", mode)
		}
		if include {
			out = append(out, filepath.Join(tasksDir, stringValue(task["id"]), "result.json"))
		}
	}
	return out, nil
}

func emitMergePlanEnv(planPath string, stdout io.Writer) error {
	plan := readJSONMap(planPath)
	merge := mapValue(plan["merge"])
	fmt.Fprintf(stdout, "MERGE_TITLE=%s\n", shellQuote(fallbackString(stringValue(merge["title"]), "DorkPipe Orchestration Synthesis")))
	fmt.Fprintf(stdout, "MERGE_SUMMARY_POINTS_JSON=%s\n", shellQuote(mustJSON(merge["summary_points"], []any{})))
	return nil
}

func buildMergeResult(outPath string, args []string) error {
	var mainPaths []string
	var planningPaths []string
	planning := false
	for _, arg := range args {
		if arg == "--planning" {
			planning = true
			continue
		}
		if strings.TrimSpace(arg) == "" {
			continue
		}
		if planning {
			planningPaths = append(planningPaths, arg)
		} else {
			mainPaths = append(mainPaths, arg)
		}
	}
	mainTasks := readMergeTaskResults(mainPaths)
	planningTasks := readMergeTaskResults(planningPaths)
	payload := map[string]any{
		"status":                        "ok",
		"tasks":                         mainTasks,
		"average_confidence":            averageMergeConfidence(mainTasks),
		"total_estimated_input_tokens":  sumMergeNumber(mainTasks, "estimated_input_tokens"),
		"total_estimated_output_tokens": sumMergeNumber(mainTasks, "estimated_output_tokens"),
		"total_estimated_task_tokens":   sumMergeNumber(mainTasks, "estimated_total_tokens"),
		"total_duration_ms":             sumMergeNumber(mainTasks, "duration_ms"),
		"max_task_duration_ms":          maxMergeNumber(mainTasks, "duration_ms"),
	}
	if len(planningTasks) > 0 {
		payload["planning_tasks"] = planningTasks
	}
	return writeJSONFile(outPath, payload)
}

func readMergeTaskResults(paths []string) []map[string]any {
	out := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		raw := readJSONMap(path)
		if len(raw) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"task_id":                 stringValue(raw["task_id"]),
			"base_task_id":            stringValue(raw["base_task_id"]),
			"comparison":              mapValue(raw["comparison"]),
			"provider_actual":         stringValue(raw["provider_actual"]),
			"used_live_model":         boolAny(raw["used_live_model"]),
			"budget_halt":             boolAny(raw["budget_halt"]),
			"estimated_input_tokens":  intAny(raw["estimated_input_tokens"]),
			"estimated_output_tokens": intAny(raw["estimated_output_tokens"]),
			"estimated_total_tokens":  intAny(raw["estimated_total_tokens"]),
			"started_at":              stringValue(raw["started_at"]),
			"finished_at":             stringValue(raw["finished_at"]),
			"duration_ms":             intAny(raw["duration_ms"]),
			"summary":                 stringValue(raw["summary"]),
			"confidence":              floatAny(raw["confidence"]),
		})
	}
	return out
}

func averageMergeConfidence(tasks []map[string]any) float64 {
	if len(tasks) == 0 {
		return 0
	}
	var total float64
	for _, task := range tasks {
		total += floatAny(task["confidence"])
	}
	return total / float64(len(tasks))
}

func sumMergeNumber(tasks []map[string]any, key string) int {
	total := 0
	for _, task := range tasks {
		total += intAny(task[key])
	}
	return total
}

func maxMergeNumber(tasks []map[string]any, key string) int {
	max := 0
	for _, task := range tasks {
		if value := intAny(task[key]); value > max {
			max = value
		}
	}
	return max
}

func renderMergeFinal(resultPath, destPath, tasksDir string, env map[string]string) error {
	mergeResult := readJSONMap(resultPath)
	title := fallbackString(env["MERGE_TITLE"], "DorkPipe Orchestration Synthesis")
	var summaryPoints []string
	_ = json.Unmarshal([]byte(env["MERGE_SUMMARY_POINTS_JSON"]), &summaryPoints)
	lines := []string{"# " + title, "", "## Task Summaries", ""}
	for _, raw := range listValue(mergeResult["tasks"]) {
		task := mapValue(raw)
		lines = append(lines, fmt.Sprintf("- `%s` (%s): %s", stringValue(task["task_id"]), fallbackString(stringValue(task["provider_actual"]), "unknown"), stringValue(task["summary"])))
	}
	if len(summaryPoints) > 0 {
		lines = append(lines, "", "## Synthesis", "")
		for _, point := range summaryPoints {
			lines = append(lines, "- "+point)
		}
	}
	planningTasks := listValue(mergeResult["planning_tasks"])
	if len(planningTasks) > 0 {
		lines = append(lines, "", "## Planning Scouts", "")
		for _, raw := range planningTasks {
			task := mapValue(raw)
			lines = append(lines, fmt.Sprintf("- `%s` (%s): %s", stringValue(task["task_id"]), fallbackString(stringValue(task["provider_actual"]), "unknown"), stringValue(task["summary"])))
		}
	}
	lines = append(lines, "", "## Worker Outputs", "")
	for _, raw := range listValue(mergeResult["tasks"]) {
		task := mapValue(raw)
		taskID := stringValue(task["task_id"])
		lines = append(lines, "### "+taskID, "")
		responsePath := filepath.Join(tasksDir, taskID, "response.md")
		if rawResponse, err := os.ReadFile(responsePath); err == nil {
			lines = append(lines, strings.TrimSpace(string(rawResponse)))
		} else {
			lines = append(lines, "_No response artifact was written._")
		}
		lines = append(lines, "")
	}
	if haltPath := env["DORKPIPE_ORCH_HALT_JSON"]; haltPath != "" {
		if _, err := os.Stat(haltPath); err == nil {
			lines = append(lines, "", "## Budget Halt", "", "- This run triggered the cloud budget halt, so later cloud tasks were intentionally skipped.")
		}
	}
	return os.WriteFile(destPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func emitVerifyPlanEnv(planPath string, stdout io.Writer) error {
	plan := readJSONMap(planPath)
	verify := mapValue(plan["verify"])
	fmt.Fprintf(stdout, "VERIFY_NEXT_ACTION_DEFAULT=%s\n", shellQuote(fallbackString(stringValue(verify["next_action_default"]), "human approval before treating orchestration output as final")))
	return nil
}

func emitVerifySummaryEnv(mergePath string, stdout io.Writer) error {
	merge := readJSONMap(mergePath)
	tasks := listValue(merge["tasks"])
	liveCount := 0
	fallbackCount := 0
	for _, raw := range tasks {
		task := mapValue(raw)
		if boolAny(task["used_live_model"]) {
			liveCount++
		} else {
			fallbackCount++
		}
	}
	confidence := floatDefault(merge["average_confidence"], 0.6)
	fmt.Fprintf(stdout, "VERIFY_LIVE_COUNT=%s\n", shellQuote(strconv.Itoa(liveCount)))
	fmt.Fprintf(stdout, "VERIFY_FALLBACK_COUNT=%s\n", shellQuote(strconv.Itoa(fallbackCount)))
	fmt.Fprintf(stdout, "VERIFY_AVG_CONFIDENCE=%s\n", shellQuote(strconv.FormatFloat(confidence, 'f', -1, 64)))
	return nil
}

func emitVerifyHeuristics(mergePath, tasksDir, issuesJSON string, stdout io.Writer) error {
	merge := readJSONMap(mergePath)
	var issues []string
	_ = json.Unmarshal([]byte(issuesJSON), &issues)
	for _, raw := range append(listValue(merge["planning_tasks"]), listValue(merge["tasks"])...) {
		task := mapValue(raw)
		taskID := stringValue(task["task_id"])
		if taskID == "" {
			continue
		}
		responsePath := filepath.Join(tasksDir, taskID, "response.md")
		responseRaw, err := os.ReadFile(responsePath)
		if err != nil {
			issues = append(issues, taskID+": response artifact is missing")
			continue
		}
		text := string(responseRaw)
		promptText := ""
		if rawPrompt, err := os.ReadFile(filepath.Join(tasksDir, taskID, "prompt.md")); err == nil {
			promptText = string(rawPrompt)
		}
		stripped := strings.TrimLeft(text, " \t\r\n")
		switch {
		case reBadPlanNarration.MatchString(text):
			issues = append(issues, taskID+": worker narrated a plan instead of returning the requested artifact")
		case reArtifactMimic.MatchString(text):
			issues = append(issues, taskID+": worker imitated orchestration artifacts instead of answering the task")
		case reSampleJSON.MatchString(text):
			issues = append(issues, taskID+": worker returned sample JSON artifacts instead of concise markdown")
		case reImplChatter.MatchString(text):
			issues = append(issues, taskID+": worker included implementation/reporting chatter")
		}
		switch {
		case reBoundary1.MatchString(text):
			issues = append(issues, taskID+": worker incorrectly said workflow does not own concurrency declaration")
		case reBoundary2.MatchString(text):
			issues = append(issues, taskID+": worker incorrectly said workflow does not own concurrency declaration")
		case reBoundary3.MatchString(text):
			issues = append(issues, taskID+": worker incorrectly assigned concurrency declaration to worker results")
		}
		switch {
		case reShape1.MatchString(text):
			issues = append(issues, taskID+": worker included preamble instead of direct artifact content")
		case reShape2.MatchString(text):
			issues = append(issues, taskID+": worker added a note/footer instead of direct artifact content")
		case reShape3.MatchString(text):
			issues = append(issues, taskID+": worker added a false missing-information footer")
		case reShape4.MatchString(text):
			issues = append(issues, taskID+": worker added formatting commentary instead of direct artifact content")
		case reShape5.MatchString(text):
			issues = append(issues, taskID+": worker repeated task id as a heading")
		case reShape6.MatchString(text):
			issues = append(issues, taskID+": worker added generic uncertainty instead of bounded uncertainty")
		case reShape7.MatchString(text):
			issues = append(issues, taskID+": worker invented lane score citation guidance")
		}
		if strings.Contains(promptText, `The first character of the response must be "-".`) && !strings.HasPrefix(stripped, "-") {
			issues = append(issues, taskID+": worker did not start with the required dash bullet")
			continue
		}
		bulletLines := []string{}
		for _, line := range strings.Split(text, "\n") {
			if reBulletMarker.MatchString(line) {
				bulletLines = append(bulletLines, line)
			}
		}
		for _, match := range reBulletPrefix.FindAllStringSubmatch(promptText, -1) {
			index, _ := strconv.Atoi(match[1])
			index--
			required := match[2]
			if index >= len(bulletLines) {
				issues = append(issues, fmt.Sprintf("%s: bullet %d did not start with %q", taskID, index+1, required))
				continue
			}
			line := reBulletMarker.ReplaceAllString(bulletLines[index], "")
			if !strings.HasPrefix(line, required) {
				issues = append(issues, fmt.Sprintf("%s: bullet %d did not start with %q", taskID, index+1, required))
			}
		}
		if match := reExactBullets.FindStringSubmatch(promptText); len(match) == 2 {
			expected := wordNumber(match[1])
			lines := []string{}
			for _, line := range strings.Split(text, "\n") {
				if strings.TrimSpace(line) != "" {
					lines = append(lines, line)
				}
			}
			if expected > 0 && (len(lines) != expected || anyLine(lines, func(line string) bool { return !reBulletMarker.MatchString(line) })) {
				issues = append(issues, fmt.Sprintf("%s: worker did not return exactly %d markdown bullets", taskID, expected))
				continue
			}
		}
	}
	status := "pass"
	if len(issues) > 0 {
		status = "review"
	}
	fmt.Fprintf(stdout, "VERIFY_HEURISTIC_STATUS=%s\n", shellQuote(status))
	fmt.Fprintf(stdout, "VERIFY_HEURISTIC_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
	return nil
}

func emitVerifyApplyCoherence(rootPath, artifactRootPath, planPath, issuesJSON string, stdout io.Writer) error {
	plan := readJSONMap(planPath)
	applyCfg := mapValue(plan["apply"])
	outputs := listValue(applyCfg["outputs"])
	issues := []string{}
	_ = json.Unmarshal([]byte(issuesJSON), &issues)
	if len(outputs) == 0 {
		fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote("pass"))
		fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
		return nil
	}
	stage, err := stageApplyOutputs(rootPath, artifactRootPath, outputs)
	if err != nil {
		issues = append(issues, err.Error())
		fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote("review"))
		fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
		return nil
	}
	defer os.RemoveAll(stage.TempRoot)
	for _, item := range stage.Files {
		switch strings.ToLower(filepath.Ext(item.TargetPath)) {
		case ".md":
			issues = append(issues, verifyMarkdownTargets(stage, item)...)
			if strings.EqualFold(filepath.Base(item.TargetPath), "validation.md") {
				issues = append(issues, verifyValidationClaims(stage, item)...)
			}
		case ".yml", ".yaml":
			issues = append(issues, verifyYAMLTargets(stage, item)...)
		}
	}
	status := "pass"
	if len(issues) > 0 {
		status = "review"
	}
	fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote(status))
	fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
	return nil
}

type stagedApplyFile struct {
	SourcePath string
	TargetPath string
	StagePath  string
}

type stagedApplyTree struct {
	Root     string
	TempRoot string
	Files    []stagedApplyFile
}

func stageApplyOutputs(rootPath, artifactRootPath string, outputs []any) (stagedApplyTree, error) {
	root, err := filepath.Abs(rootPath)
	if err != nil {
		return stagedApplyTree{}, err
	}
	artifactRoot, err := filepath.Abs(artifactRootPath)
	if err != nil {
		return stagedApplyTree{}, err
	}
	tempRoot, err := os.MkdirTemp("", "dockpipe-orch-apply-*")
	if err != nil {
		return stagedApplyTree{}, err
	}
	stageRoot := filepath.Join(tempRoot, "root")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		_ = os.RemoveAll(tempRoot)
		return stagedApplyTree{}, err
	}
	files := []stagedApplyFile{}
	for _, raw := range outputs {
		item := mapValue(raw)
		source := strings.TrimSpace(stringValue(item["source"]))
		target := strings.TrimSpace(stringValue(item["path"]))
		if source == "" || target == "" {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, errors.New("each apply output needs source and path")
		}
		sourcePath, err := filepath.Abs(filepath.Join(artifactRoot, source))
		if err != nil || !withinRoot(artifactRoot, sourcePath) {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, fmt.Errorf("apply source escapes artifact root: %s", source)
		}
		info, err := os.Stat(sourcePath)
		if err != nil || info.IsDir() {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, fmt.Errorf("apply source is missing: %s", source)
		}
		targetPath, targetRoot, err := resolveApplyTargetPath(root, target)
		if err != nil {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, err
		}
		if !withinRoot(targetRoot, targetPath) {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, fmt.Errorf("apply target escapes worktree: %s", target)
		}
		if !withinRoot(root, targetPath) {
			continue
		}
		stagePath, err := stagedPathForTarget(stageRoot, root, targetPath)
		if err != nil {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, err
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, err
		}
		if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, err
		}
		if err := os.WriteFile(stagePath, content, 0o644); err != nil {
			_ = os.RemoveAll(tempRoot)
			return stagedApplyTree{}, err
		}
		files = append(files, stagedApplyFile{
			SourcePath: sourcePath,
			TargetPath: targetPath,
			StagePath:  stagePath,
		})
	}
	return stagedApplyTree{Root: root, TempRoot: tempRoot, Files: files}, nil
}

func stagedPathForTarget(stageRoot, root, targetPath string) (string, error) {
	rel, err := filepath.Rel(root, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("apply target escapes staging root: %s", targetPath)
	}
	return filepath.Join(stageRoot, rel), nil
}

func stagePathOrTarget(stage stagedApplyTree, targetPath string) string {
	for _, item := range stage.Files {
		if sameFilePath(item.TargetPath, targetPath) {
			return item.StagePath
		}
	}
	return targetPath
}

func sameFilePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func hasSchedulerOutputConflict(task schedulerTask, running map[string]schedulerTask) bool {
	if strings.TrimSpace(task.OutputPath) == "" {
		return false
	}
	for _, active := range running {
		if strings.TrimSpace(active.OutputPath) == "" {
			continue
		}
		if sameFilePath(task.OutputPath, active.OutputPath) {
			return true
		}
	}
	return false
}

func verifyMarkdownTargets(stage stagedApplyTree, item stagedApplyFile) []string {
	raw, err := os.ReadFile(item.StagePath)
	if err != nil {
		return []string{fmt.Sprintf("%s: staged markdown output could not be read", relativeTo(stage.Root, item.TargetPath))}
	}
	issues := []string{}
	for _, match := range reMarkdownLink.FindAllStringSubmatch(string(raw), -1) {
		if len(match) < 2 {
			continue
		}
		target := strings.TrimSpace(match[1])
		if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		if strings.HasPrefix(target, "/") {
			continue
		}
		candidate := filepath.Clean(filepath.Join(filepath.Dir(item.TargetPath), filepath.FromSlash(target)))
		resolved := stagePathOrTarget(stage, candidate)
		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			issues = append(issues, fmt.Sprintf("%s: markdown link target is missing: %s", relativeTo(stage.Root, item.TargetPath), target))
		}
	}
	return issues
}

func verifyYAMLTargets(stage stagedApplyTree, item stagedApplyFile) []string {
	raw, err := os.ReadFile(item.StagePath)
	if err != nil {
		return []string{fmt.Sprintf("%s: staged yaml output could not be read", relativeTo(stage.Root, item.TargetPath))}
	}
	var payload any
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return []string{fmt.Sprintf("%s: yaml could not be parsed: %v", relativeTo(stage.Root, item.TargetPath), err)}
	}
	issues := []string{}
	walkStringScalars(payload, func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "://") {
			return
		}
		lower := strings.ToLower(trimmed)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
			return
		}
		candidate := filepath.Clean(filepath.Join(filepath.Dir(item.TargetPath), filepath.FromSlash(trimmed)))
		resolved := stagePathOrTarget(stage, candidate)
		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			issues = append(issues, fmt.Sprintf("%s: yaml reference target is missing: %s", relativeTo(stage.Root, item.TargetPath), trimmed))
		}
	})
	return issues
}

func verifyValidationClaims(stage stagedApplyTree, item stagedApplyFile) []string {
	raw, err := os.ReadFile(item.StagePath)
	if err != nil {
		return nil
	}
	issues := []string{}
	for _, match := range reValidationRemoved.FindAllStringSubmatch(string(raw), -1) {
		if len(match) < 2 {
			continue
		}
		claimed := strings.TrimSpace(match[1])
		if claimed == "" {
			continue
		}
		candidate := filepath.Clean(filepath.Join(filepath.Dir(item.TargetPath), filepath.FromSlash(claimed)))
		resolved := stagePathOrTarget(stage, candidate)
		if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
			issues = append(issues, fmt.Sprintf("%s: validation claims %s was removed but it still exists", relativeTo(stage.Root, item.TargetPath), claimed))
		}
	}
	return issues
}

func walkStringScalars(value any, visit func(string)) {
	switch typed := value.(type) {
	case string:
		visit(typed)
	case []any:
		for _, item := range typed {
			walkStringScalars(item, visit)
		}
	case map[string]any:
		for _, item := range typed {
			walkStringScalars(item, visit)
		}
	case map[any]any:
		for _, item := range typed {
			walkStringScalars(item, visit)
		}
	}
}

func relativeTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func applyResults(rootPath, artifactRootPath, planPath, approvalPath, resultPath string) error {
	root, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	artifactRoot, err := filepath.Abs(artifactRootPath)
	if err != nil {
		return err
	}
	plan := readJSONMap(planPath)
	applyCfg := mapValue(plan["apply"])
	outputs := listValue(applyCfg["outputs"])
	requireApproval := true
	if raw, ok := applyCfg["require_approval"]; ok {
		requireApproval = boolAny(raw)
	}
	fail := func(message string) error {
		_ = writeJSONFile(resultPath, map[string]any{
			"status":  "skipped",
			"reason":  message,
			"applied": []any{},
		})
		return errors.New(message)
	}
	if len(outputs) == 0 {
		return fail("no apply outputs declared")
	}
	if requireApproval {
		rawApproval, err := os.ReadFile(approvalPath)
		if err != nil {
			return fail("approval artifact is required before apply")
		}
		if !strings.Contains(string(rawApproval), "- Approved: yes") {
			return fail("approval artifact does not approve apply")
		}
	}
	type pendingApply struct {
		sourcePath string
		targetPath string
		content    []byte
	}
	pending := []pendingApply{}
	for _, raw := range outputs {
		item := mapValue(raw)
		source := strings.TrimSpace(stringValue(item["source"]))
		target := strings.TrimSpace(stringValue(item["path"]))
		if source == "" || target == "" {
			return fail("each apply output needs source and path")
		}
		sourcePath, err := filepath.Abs(filepath.Join(artifactRoot, source))
		if err != nil {
			return fail("apply source escapes artifact root: " + source)
		}
		targetPath, targetRoot, err := resolveApplyTargetPath(root, target)
		if err != nil {
			return fail(err.Error())
		}
		if !withinRoot(artifactRoot, sourcePath) {
			return fail("apply source escapes artifact root: " + source)
		}
		if !withinRoot(targetRoot, targetPath) {
			return fail("apply target escapes worktree: " + target)
		}
		info, err := os.Stat(sourcePath)
		if err != nil || info.IsDir() {
			return fail("apply source is missing: " + source)
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		pending = append(pending, pendingApply{
			sourcePath: sourcePath,
			targetPath: targetPath,
			content:    content,
		})
	}
	applied := []map[string]any{}
	for _, item := range pending {
		if err := os.MkdirAll(filepath.Dir(item.targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(item.targetPath, item.content, 0o644); err != nil {
			return err
		}
		relTarget, _ := filepath.Rel(root, item.targetPath)
		relSource, err := filepath.Rel(root, item.sourcePath)
		sourceValue := item.sourcePath
		if err == nil && !strings.HasPrefix(relSource, "..") {
			sourceValue = relSource
		}
		applied = append(applied, map[string]any{
			"source": sourceValue,
			"path":   relTarget,
			"bytes":  len(item.content),
		})
	}
	return writeJSONFile(resultPath, map[string]any{
		"status":  "applied",
		"applied": applied,
	})
}

func resolveApplyTargetPath(root, target string) (string, string, error) {
	if strings.TrimSpace(target) == "" {
		return "", "", errors.New("apply target is empty")
	}
	if strings.HasPrefix(filepath.ToSlash(target), "/") {
		if !guestPathAllowedByPolicy(target, os.Getenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS")) {
			return "", "", fmt.Errorf("apply target is not allowed by DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS: %s", target)
		}
		hostPath, hostRoot, ok := resolveGuestMountTarget(target, os.Getenv("DOCKPIPE_CONTAINER_MOUNTS"))
		if !ok {
			hostPath, hostRoot, ok = resolvePrimaryWorkTarget(root, target)
		}
		if !ok {
			return "", "", fmt.Errorf("apply target uses undeclared guest path: %s", target)
		}
		return hostPath, hostRoot, nil
	}
	targetPath, err := filepath.Abs(filepath.Join(root, target))
	if err != nil {
		return "", "", fmt.Errorf("apply target escapes worktree: %s", target)
	}
	return targetPath, root, nil
}

func inferTaskOutputPath(task map[string]any) string {
	if direct := strings.TrimSpace(stringValue(task["output_path"])); direct != "" {
		return direct
	}
	for _, field := range []string{"expected_output", "prompt"} {
		text := stringValue(task[field])
		if match := reGuestDocPath.FindString(text); match != "" {
			return match
		}
	}
	return ""
}

func resolvePrimaryWorkTarget(root, target string) (string, string, bool) {
	target = cleanGuestPath(target)
	if target == "" || !guestPathContains("/work", target) {
		return "", "", false
	}
	hostRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", false
	}
	rel := strings.TrimPrefix(target, "/work")
	rel = strings.TrimPrefix(rel, "/")
	targetPath, err := filepath.Abs(filepath.Join(hostRoot, filepath.FromSlash(rel)))
	if err != nil || !withinRoot(hostRoot, targetPath) {
		return "", "", false
	}
	return targetPath, hostRoot, true
}

func guestPathAllowedByPolicy(target, allowedRoots string) bool {
	allowedRoots = strings.TrimSpace(allowedRoots)
	if allowedRoots == "" {
		return true
	}
	target = cleanGuestPath(target)
	if target == "" {
		return false
	}
	for _, raw := range strings.FieldsFunc(allowedRoots, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		root := cleanGuestPath(raw)
		if root != "" && guestPathContains(root, target) {
			return true
		}
	}
	return false
}

func resolveGuestMountTarget(target, mountEnv string) (string, string, bool) {
	target = cleanGuestPath(target)
	if target == "" {
		return "", "", false
	}
	bestGuest := ""
	bestHost := ""
	for _, line := range strings.Split(mountEnv, "\n") {
		host, guest, ok := parseContainerMountSpec(line)
		if !ok {
			continue
		}
		guest = cleanGuestPath(guest)
		if guest == "" || !guestPathContains(guest, target) {
			continue
		}
		if len(guest) > len(bestGuest) {
			bestGuest = guest
			bestHost = host
		}
	}
	if bestHost == "" {
		return "", "", false
	}
	hostRoot, err := filepath.Abs(bestHost)
	if err != nil {
		return "", "", false
	}
	rel := strings.TrimPrefix(target, bestGuest)
	rel = strings.TrimPrefix(rel, "/")
	targetPath, err := filepath.Abs(filepath.Join(hostRoot, filepath.FromSlash(rel)))
	if err != nil || !withinRoot(hostRoot, targetPath) {
		return "", "", false
	}
	return targetPath, hostRoot, true
}

func parseContainerMountSpec(spec string) (string, string, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", false
	}
	base := spec
	if idx := strings.LastIndex(base, ":"); idx >= 0 {
		suffix := strings.ToLower(strings.TrimSpace(base[idx+1:]))
		if suffix == "ro" || suffix == "rw" {
			base = base[:idx]
		}
	}
	idx := strings.LastIndex(base, ":")
	if idx <= 0 || idx >= len(base)-1 {
		return "", "", false
	}
	host := strings.TrimSpace(base[:idx])
	guest := strings.TrimSpace(base[idx+1:])
	if host == "" || guest == "" || !strings.HasPrefix(filepath.ToSlash(guest), "/") {
		return "", "", false
	}
	return host, guest, true
}

func cleanGuestPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = normalizeGitBashGuestPath(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == "." {
		return ""
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

func normalizeGitBashGuestPath(path string) string {
	lower := strings.ToLower(path)
	for _, marker := range []string{":/program files/git/", ":/program files (x86)/git/"} {
		if idx := strings.Index(lower, marker); idx > 0 && idx == 1 {
			return "/" + path[idx+len(marker):]
		}
	}
	for _, marker := range []string{"/program files/git/", "/program files (x86)/git/"} {
		if strings.HasPrefix(lower, marker) {
			return "/" + path[len(marker):]
		}
		if len(path) >= 3 && path[0] == '/' && path[2] == '/' && strings.HasPrefix(lower[2:], marker) {
			return "/" + path[2+len(marker):]
		}
	}
	return path
}

func guestPathContains(root, target string) bool {
	if root == target {
		return true
	}
	return strings.HasPrefix(target, strings.TrimRight(root, "/")+"/")
}

func taskIDSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func downstreamTaskClosure(tasks []any, selected map[string]bool) map[string]bool {
	if len(selected) == 0 {
		return map[string]bool{}
	}
	closure := map[string]bool{}
	for key := range selected {
		closure[key] = true
	}
	changed := true
	for changed {
		changed = false
		for _, raw := range tasks {
			task := mapValue(raw)
			taskID := stringValue(task["id"])
			if taskID == "" || closure[taskID] {
				continue
			}
			for _, dep := range stringList(task["depends_on"]) {
				if closure[dep] {
					closure[taskID] = true
					changed = true
					break
				}
			}
		}
	}
	return closure
}

func planOrchestration(workflowPath, stepID string, env map[string]string) error {
	root := env["ROOT"]
	sharedDir := env["DORKPIPE_ORCH_SHARED_DIR"]
	tasksDir := env["DORKPIPE_ORCH_TASKS_DIR"]
	requestJSON := env["DORKPIPE_ORCH_REQUEST_JSON"]
	planJSON := env["DORKPIPE_ORCH_PLAN_JSON"]
	graphJSON := env["DORKPIPE_ORCH_GRAPH_JSON"]
	lanePlanJSON := env["DORKPIPE_ORCH_LANE_PLAN_JSON"]
	modelCatalogPath := env["DORKPIPE_ORCH_MODEL_CATALOG"]
	baselinePolicyPath := env["DORKPIPE_ORCH_BASELINE_POLICY"]
	globalTrainingMetricsPath := env["DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS"]
	workflowName := env["DORKPIPE_ORCH_WORKFLOW"]
	artifactRoot := env["DORKPIPE_ORCH_ROOT"]
	maxTotalCloudTokens := intFromString(env["DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS"], 120000)
	maxTaskCloudTokens := intFromString(env["DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS"], 40000)
	stopOnBudgetExceeded := boolString(env["DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED"])
	trainingMode := fallbackString(env["DORKPIPE_ORCH_TRAINING_MODE"], "observe")
	cloudLanesEnabled := boolString(env["DORKPIPE_ORCH_CLOUD_LANES"])
	forceProvider := strings.ToLower(strings.TrimSpace(fallbackString(env["DORKPIPE_ORCH_FORCE_PROVIDER"], env["DORKPIPE_ORCH_TASK_PROVIDER"])))
	forceProviderScope := strings.ToLower(strings.TrimSpace(fallbackString(env["DORKPIPE_ORCH_FORCE_PROVIDER_SCOPE"], "auto")))
	compareProviders := splitCSVLower(env["DORKPIPE_ORCH_COMPARE_PROVIDERS"])
	compareScope := strings.ToLower(strings.TrimSpace(fallbackString(env["DORKPIPE_ORCH_COMPARE_SCOPE"], "auto")))
	inlineInputContext := boolString(fallbackString(env["DORKPIPE_ORCH_INLINE_INPUT_CONTEXT"], "true"))
	inlineInputMaxBytes := intFromString(env["DORKPIPE_ORCH_INLINE_INPUT_MAX_BYTES"], 6000)
	inlineInputTotalMaxBytes := intFromString(env["DORKPIPE_ORCH_INLINE_INPUT_TOTAL_MAX_BYTES"], 18000)
	followUpRequest := strings.TrimSpace(env["DORKPIPE_ORCH_FOLLOWUP_REQUEST"])
	followUpGoal := strings.TrimSpace(env["DORKPIPE_ORCH_FOLLOWUP_GOAL"])
	followUpTaskIDs := splitCSVTrim(env["DORKPIPE_ORCH_FOLLOWUP_TASK_IDS"])
	followUpMode := followUpRequest != "" || followUpGoal != "" || len(followUpTaskIDs) > 0

	workflow := readYAMLMap(workflowPath)
	workflowModelPolicy := mapValue(workflow["model_policy"])
	steps := listValue(workflow["steps"])
	var currentStep map[string]any
	for _, raw := range steps {
		step := mapValue(raw)
		if stringValue(step["id"]) == stepID {
			currentStep = step
			break
		}
	}
	if currentStep == nil {
		return fmt.Errorf("%s: could not find step id %q", workflowPath, stepID)
	}
	agent := mapValue(currentStep["agent"])
	orchestration := mapValue(agent["orchestration"])
	request := mapValue(orchestration["request"])
	planCfg := mapValue(orchestration["plan"])
	shared := listValue(orchestration["shared"])
	tasks := listValue(orchestration["tasks"])
	merge := mapValue(orchestration["merge"])
	verify := mapValue(orchestration["verify"])
	concurrency := mapValue(orchestration["concurrency"])
	apply := mapValue(orchestration["apply"])
	startupPrompt := stringValue(agent["startup_prompt"])
	includeAgentsMD := true
	if raw, ok := agent["include_agents_md"]; ok {
		includeAgentsMD = boolAny(raw)
	}
	workflowAccessiblePathsRaw := listValue(agent["accessible_paths"])
	workflowAccessRaw := mapValue(agent["access"])
	agentModelPolicy := workflowModelPolicy
	if modelPolicy := mapValue(agent["model_policy"]); len(modelPolicy) > 0 {
		agentModelPolicy = modelPolicy
	}
	if len(tasks) == 0 {
		return fmt.Errorf("%s: steps[].agent.orchestration.tasks must contain at least one task", workflowPath)
	}
	taskIDs := map[string]bool{}
	for _, raw := range tasks {
		taskID := stringValue(mapValue(raw)["id"])
		if taskID != "" {
			taskIDs[taskID] = true
		}
	}
	selectedFollowUpTasks := taskIDSet(followUpTaskIDs)
	for taskID := range selectedFollowUpTasks {
		if !taskIDs[taskID] {
			return fmt.Errorf("%s: DORKPIPE_ORCH_FOLLOWUP_TASK_IDS includes unknown task id %q", workflowPath, taskID)
		}
	}
	rerunTasks := map[string]bool{}
	if followUpMode {
		if len(selectedFollowUpTasks) == 0 {
			for taskID := range taskIDs {
				rerunTasks[taskID] = true
			}
		} else {
			rerunTasks = downstreamTaskClosure(tasks, selectedFollowUpTasks)
		}
	}
	scopeCache := map[string]string{}
	resolveScopeRef := func(value string) (string, error) {
		if !strings.HasPrefix(value, "scope:") {
			return value, nil
		}
		if cached, ok := scopeCache[value]; ok {
			return cached, nil
		}
		parts := strings.SplitN(value, ":", 4)
		if len(parts) < 2 || parts[1] == "" {
			return "", fmt.Errorf("%s: invalid scope reference %q", workflowPath, value)
		}
		args := []string{"scope"}
		kind := parts[1]
		switch kind {
		case "source", "repo", "workdir", "artifacts", "artifact", "output":
			args = append(args, kind)
			if len(parts) >= 3 && parts[2] != "" {
				args = append(args, parts[2])
			}
		case "workflow", "package":
			if len(parts) < 3 || parts[2] == "" {
				return "", fmt.Errorf("%s: scope:%s: requires a name", workflowPath, kind)
			}
			if kind == "workflow" {
				args = append(args, "workflow", parts[2])
			} else {
				args = append(args, "--package", parts[2])
			}
			if len(parts) == 4 && parts[3] != "" {
				args = append(args, parts[3])
			}
		default:
			return "", fmt.Errorf("%s: unsupported scope reference %q", workflowPath, value)
		}
		args = append(args, "--workdir", root)
		resolved, err := runCommandString(root, env, resolveDockpipeCommand(root, env), args...)
		if err != nil {
			return "", fmt.Errorf("%s: could not resolve %q: %w", workflowPath, value, err)
		}
		scopeCache[value] = strings.TrimSpace(resolved)
		return scopeCache[value], nil
	}
	resolveScopeList := func(values []any) ([]string, error) {
		out := []string{}
		for _, raw := range values {
			resolved, err := resolveScopeRef(fmt.Sprint(raw))
			if err != nil {
				return nil, err
			}
			out = append(out, resolved)
		}
		return out, nil
	}
	resolveAccessBlock := func(access map[string]any) (map[string]any, error) {
		read, err := resolveScopeList(listValue(access["read"]))
		if err != nil {
			return nil, err
		}
		write, err := resolveScopeList(listValue(access["write"]))
		if err != nil {
			return nil, err
		}
		deny, err := resolveScopeList(listValue(access["deny"]))
		if err != nil {
			return nil, err
		}
		return map[string]any{"read": read, "write": write, "deny": deny}, nil
	}
	workflowAccessiblePaths, err := resolveScopeList(workflowAccessiblePathsRaw)
	if err != nil {
		return err
	}
	workflowAccess, err := resolveAccessBlock(workflowAccessRaw)
	if err != nil {
		return err
	}
	hostProfile := detectLocalHostProfile(env)
	modelLanes := loadModelLanes(modelCatalogPath, env)
	lanesByProvider := map[string]map[string]any{}
	for _, lane := range modelLanes {
		lanesByProvider[stringValue(lane["provider"])] = lane
	}
	baselinePolicy := readYAMLMapOptional(baselinePolicyPath)
	selectionPolicy := mapValue(baselinePolicy["selection"])
	trainingPolicy := mapValue(baselinePolicy["training"])
	trainingStats := loadTrainingStats(globalTrainingMetricsPath, trainingPolicy)

	for _, raw := range shared {
		entry := mapValue(raw)
		rel := stringValue(entry["path"])
		if rel == "" {
			return fmt.Errorf("%s: each shared entry needs path:", workflowPath)
		}
		dest := filepath.Join(sharedDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rendered, err := renderShared(entry, root, env)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(rendered), 0o644); err != nil {
			return err
		}
	}

	requestText := stringValue(request["text"])
	if followUpRequest != "" {
		requestText = followUpRequest
	}
	requestPayload := map[string]any{
		"contract_version": "v1",
		"workflow":         workflowName,
		"request":          requestText,
		"artifact_root":    artifactRoot,
		"workflow_config":  workflowPath,
		"step_id":          stepID,
		"cloud_budget": map[string]any{
			"max_total_cloud_tokens":  maxTotalCloudTokens,
			"max_task_cloud_tokens":   maxTaskCloudTokens,
			"stop_on_budget_exceeded": stopOnBudgetExceeded,
		},
		"access":               workflowAccess,
		"model_policy":         agentModelPolicy,
		"model_catalog":        modelCatalogPath,
		"training_mode":        trainingMode,
		"force_provider":       forceProvider,
		"force_provider_scope": forceProviderScope,
		"compare_providers":    compareProviders,
		"compare_scope":        compareScope,
	}
	if followUpMode {
		requestPayload["follow_up"] = map[string]any{
			"enabled":        true,
			"request":        followUpRequest,
			"goal":           followUpGoal,
			"selected_tasks": followUpTaskIDs,
			"rerun_tasks":    sortedTaskIDsFromSet(rerunTasks),
		}
	}
	if err := writeJSONFile(requestJSON, requestPayload); err != nil {
		return err
	}
	goalText := fallbackString(stringValue(planCfg["goal"]), requestText)
	if followUpGoal != "" {
		goalText = followUpGoal
	}
	planPayload := map[string]any{
		"goal":         goalText,
		"steps":        listValue(planCfg["steps"]),
		"cloud_budget": requestPayload["cloud_budget"],
		"concurrency": map[string]any{
			"max_workers":       maxInt(1, intFromAny(fallbackAny(concurrency["max_workers"], 1))),
			"max_local_workers": maxInt(1, intFromAny(fallbackAny(concurrency["max_local_workers"], fallbackAny(concurrency["max_workers"], 1)))),
			"max_cloud_workers": maxInt(1, intFromAny(fallbackAny(concurrency["max_cloud_workers"], 1))),
		},
		"merge": map[string]any{
			"title":          fallbackString(stringValue(merge["title"]), "DorkPipe Orchestration Synthesis"),
			"summary_points": listValue(merge["summary_points"]),
		},
		"verify": map[string]any{
			"next_action_default": fallbackString(stringValue(verify["next_action_default"]), "human approval before treating orchestration output as final"),
		},
		"apply": map[string]any{
			"require_approval": boolDefault(apply["require_approval"], true),
			"outputs":          listValue(apply["outputs"]),
		},
	}
	if followUpMode {
		planPayload["follow_up"] = requestPayload["follow_up"]
	}
	if len(compareProviders) > 0 {
		compareWidth := len(compareProviders)
		cloudCompareWidth := 0
		for _, provider := range compareProviders {
			if boolAny(mapValue(lanesByProvider[provider])["cloud"]) {
				cloudCompareWidth++
			}
		}
		localCompareWidth := compareWidth - cloudCompareWidth
		concurrencyMap := mapValue(planPayload["concurrency"])
		concurrencyMap["max_workers"] = maxInt(intFromAny(concurrencyMap["max_workers"]), compareWidth+1)
		concurrencyMap["max_local_workers"] = maxInt(intFromAny(concurrencyMap["max_local_workers"]), localCompareWidth+1)
		concurrencyMap["max_cloud_workers"] = maxInt(intFromAny(concurrencyMap["max_cloud_workers"]), cloudCompareWidth)
	}
	if err := writeJSONFile(planJSON, planPayload); err != nil {
		return err
	}

	graphTasks := []map[string]any{}
	workerIDs := []string{}
	lanePlan := map[string]any{
		"catalog":                 modelCatalogPath,
		"baseline_policy":         baselinePolicyPath,
		"training_mode":           trainingMode,
		"force_provider":          forceProvider,
		"force_provider_scope":    forceProviderScope,
		"compare_providers":       compareProviders,
		"compare_scope":           compareScope,
		"cloud_lanes_enabled":     cloudLanesEnabled,
		"global_training_metrics": globalTrainingMetricsPath,
		"policy":                  agentModelPolicy,
		"local_host_profile": map[string]any{
			"memory_gb":     hostProfile.MemoryGB,
			"cpu_cores":     hostProfile.CPUCores,
			"acceleration":  hostProfile.Acceleration,
			"hardware_tier": hostProfile.Tier,
		},
		"thresholds": map[string]any{
			"cloud_score_threshold":           floatDefault(selectionPolicy["cloud_score_threshold"], 14.0),
			"high_risk_cloud_score_threshold": floatDefault(selectionPolicy["high_risk_cloud_score_threshold"], 10.0),
			"min_samples_before_adjustment":   intFromAny(fallbackAny(trainingPolicy["min_samples_before_adjustment"], 20)),
		},
		"lanes": []map[string]any{},
		"tasks": []map[string]any{},
	}
	if followUpMode {
		lanePlan["follow_up"] = requestPayload["follow_up"]
	}
	for _, lane := range modelLanes {
		lanePlan["lanes"] = append(lanePlan["lanes"].([]map[string]any), map[string]any{
			"id":               lane["id"],
			"provider":         lane["provider"],
			"resolver_hint":    lane["resolver_hint"],
			"model":            lane["model"],
			"local":            boolAny(lane["local"]),
			"cloud":            boolAny(lane["cloud"]),
			"available":        boolAny(lane["available"]),
			"missing_commands": listValue(lane["missing_commands"]),
			"capabilities":     listValue(lane["capabilities"]),
			"context_window":   intFromAny(lane["context_window"]),
		})
	}
	for _, rawTask := range tasks {
		task, err := applyTaskWorkerProfile(mapValue(rawTask), env)
		if err != nil {
			return fmt.Errorf("%s: %w", workflowPath, err)
		}
		baseTaskID := stringValue(task["id"])
		if baseTaskID == "" {
			return fmt.Errorf("%s: each task needs id:", workflowPath)
		}
		var taskVariants []map[string]string
		if comparisonEnabledForTask(task, compareProviders, compareScope) {
			for _, provider := range compareProviders {
				taskVariants = append(taskVariants, map[string]string{
					"task_id":            comparisonTaskID(baseTaskID, provider),
					"base_task_id":       baseTaskID,
					"compare_provider":   provider,
					"requested_override": provider,
				})
			}
		} else {
			taskVariants = append(taskVariants, map[string]string{
				"task_id":            baseTaskID,
				"base_task_id":       baseTaskID,
				"compare_provider":   "",
				"requested_override": "",
			})
		}
		for _, variant := range taskVariants {
			class := classifyTask(task, selectionPolicy)
			taskID := variant["task_id"]
			workerIDs = append(workerIDs, taskID)
			taskDir := filepath.Join(tasksDir, taskID)
			reuseExisting := followUpMode && !rerunTasks[taskID]
			if !reuseExisting {
				_ = os.RemoveAll(taskDir)
			}
			if err := os.MkdirAll(taskDir, 0o755); err != nil {
				return err
			}
			taskModel := mapValue(task["model"])
			if len(taskModel) == 0 {
				taskModel = mapValue(agent["model"])
			}
			taskPolicy := agentModelPolicy
			if modelPolicy := mapValue(task["model_policy"]); len(modelPolicy) > 0 {
				taskPolicy = modelPolicy
			}
			laneSelection := selectLane(task, taskPolicy, variant["requested_override"], forceProvider, forceProviderScope, env, modelLanes, selectionPolicy, trainingPolicy, trainingStats, cloudLanesEnabled, compareProviders)
			laneSelection["task_id"] = taskID
			laneSelection["base_task_id"] = variant["base_task_id"]
			if variant["compare_provider"] != "" {
				laneSelection["comparison"] = map[string]any{
					"enabled":      true,
					"base_task_id": variant["base_task_id"],
					"provider":     variant["compare_provider"],
					"providers":    compareProviders,
				}
			} else {
				laneSelection["comparison"] = map[string]any{"enabled": false}
			}
			if err := writeJSONFile(filepath.Join(taskDir, "lane-selection.json"), laneSelection); err != nil {
				return err
			}
			laneSelection["reuse_existing"] = reuseExisting
			lanePlan["tasks"] = append(lanePlan["tasks"].([]map[string]any), laneSelection)

			accessiblePaths := append([]string{}, workflowAccessiblePaths...)
			taskAccessiblePaths, err := resolveScopeList(listValue(task["accessible_paths"]))
			if err != nil {
				return err
			}
			for _, path := range taskAccessiblePaths {
				if !containsString(accessiblePaths, path) {
					accessiblePaths = append(accessiblePaths, path)
				}
			}
			taskAccess := map[string][]string{
				"read":  stringList(workflowAccess["read"]),
				"write": stringList(workflowAccess["write"]),
				"deny":  stringList(workflowAccess["deny"]),
			}
			taskAccessRaw := mapValue(task["access"])
			for _, mode := range []string{"read", "write", "deny"} {
				paths, err := resolveScopeList(listValue(taskAccessRaw[mode]))
				if err != nil {
					return err
				}
				for _, path := range paths {
					if !containsString(taskAccess[mode], path) {
						taskAccess[mode] = append(taskAccess[mode], path)
					}
				}
			}
			taskInputs, err := resolveScopeList(listValue(task["inputs"]))
			if err != nil {
				return err
			}
			dependsOn := []string{}
			for _, depRaw := range listValue(task["depends_on"]) {
				dep := fmt.Sprint(depRaw)
				if variant["compare_provider"] != "" && taskHasComparison(tasks, dep, compareProviders, compareScope) {
					dep = comparisonTaskID(dep, variant["compare_provider"])
				}
				dependsOn = append(dependsOn, dep)
			}
			taskPayload := map[string]any{
				"id":                      taskID,
				"base_id":                 variant["base_task_id"],
				"reuse_existing":          reuseExisting,
				"worker":                  stringValue(task["worker"]),
				"worker_policy":           mapValue(task["worker_policy"]),
				"comparison":              laneSelection["comparison"],
				"goal":                    stringValue(task["goal"]),
				"inputs":                  taskInputs,
				"constraints":             listValue(task["constraints"]),
				"expected_output":         stringValue(task["expected_output"]),
				"output_path":             inferTaskOutputPath(task),
				"worker_type":             fallbackString(stringValue(task["worker_type"]), "analysis"),
				"work_mode":               stringValue(task["work_mode"]),
				"resolver_hint":           fallbackString(stringValue(laneSelection["resolver_hint"]), expandEnv(fallbackString(stringValue(task["resolver_hint"]), "auto"), env)),
				"requested_resolver_hint": expandEnv(fallbackString(stringValue(task["resolver_hint"]), "auto"), env),
				"lane":                    laneSelection,
				"max_cloud_tokens":        intFromAny(fallbackAny(task["max_cloud_tokens"], fallbackAny(laneSelection["max_task_tokens"], maxTaskCloudTokens))),
				"depends_on":              dependsOn,
				"claims":                  listValue(task["claims"]),
				"citations":               mustResolveScopeCitations(task, taskInputs, resolveScopeList),
				"startup_prompt":          startupPrompt,
				"include_agents_md":       includeAgentsMD,
				"accessible_paths":        accessiblePaths,
				"access":                  taskAccess,
				"model":                   taskModel,
				"model_policy":            taskPolicy,
				"task_class": map[string]any{
					"name":      class.Name,
					"authority": class.Authority,
				},
			}
			if len(taskModel) == 0 {
				taskPayload["model"] = map[string]any{
					"provider": laneSelection["provider"],
					"model":    laneSelection["model"],
					"num_ctx":  laneSelection["context_window"],
				}
			}
			if followUpMode {
				taskPayload["follow_up"] = requestPayload["follow_up"]
			}
			if err := writeJSONFile(filepath.Join(taskDir, "task.json"), taskPayload); err != nil {
				return err
			}
			prompt := stringValue(task["prompt"])
			if prompt == "" {
				lines := []string{
					"You are one worker in a DorkPipe orchestration graph.",
					"",
					"Task id: " + taskID,
					"Base task id: " + variant["base_task_id"],
					"Goal: " + stringValue(taskPayload["goal"]),
					"Expected output: " + stringValue(taskPayload["expected_output"]),
					"",
					"Rules:",
					"- Treat this as one bounded task, not the whole request.",
					"- Ground claims in the referenced inputs.",
					"- Return concise markdown suitable for downstream merge.",
					"- Return the requested artifact content directly; do not narrate your tool workflow.",
					"- Call out uncertainty explicitly.",
				}
				if variant["compare_provider"] != "" {
					lines = append(lines, "", "Comparison mode:", fmt.Sprintf("- You are the %s fork for base task `%s`.", variant["compare_provider"], variant["base_task_id"]), "- Produce an independent answer for later side-by-side evaluation.")
				}
				prompt = strings.Join(lines, "\n") + "\n"
			}
			outputContract := []string{}
			if boolString(fallbackString(env["DORKPIPE_ORCH_STRICT_OUTPUT_CONTRACT"], "true")) {
				outputContract = append(outputContract,
					"DorkPipe worker output contract:",
					"- Return only the requested markdown artifact content.",
					"- Answer the task directly in final markdown.",
					"- Do not describe files you wrote, commands you ran, validation steps, source-control status, or container behavior.",
					"- Do not say you completed the task; produce the task answer itself.",
					"- Do not create or describe task.json, lane-selection.json, result.json, merge artifacts, or example artifacts in the response.",
					"- Do not write an execution plan, checklist, final report, or sample output unless the task explicitly asks for one.",
					"- Do not write or modify source files unless the task explicitly asks for edits.",
					"- Use the same output standard regardless of provider or model lane.",
				)
			}
			if variant["compare_provider"] != "" {
				outputContract = append(outputContract, "", "Comparison mode:", fmt.Sprintf("- You are the %s fork for base task `%s`.", variant["compare_provider"], variant["base_task_id"]), "- Produce an independent answer for side-by-side evaluation.", "- Do not mention that you are in a competition or compare yourself to other lanes.")
			}
			if len(outputContract) > 0 {
				prompt = strings.Join(outputContract, "\n") + "\n\n" + strings.TrimLeft(prompt, "\n")
			}
			localLane := boolAny(laneSelection["local"])
			prefix := []string{}
			if followUpMode && !reuseExisting {
				followUpLines := []string{
					"Follow-up repair mode:",
					"- This is a targeted rerun on top of an existing orchestration workspace.",
					"- Preserve the existing doc set shape unless the follow-up request requires a concrete correction.",
					"- Prefer minimal edits that fix the stated issues without broad rewrites.",
				}
				if followUpRequest != "" {
					followUpLines = append(followUpLines, "- Follow-up request: "+followUpRequest)
				}
				if followUpGoal != "" {
					followUpLines = append(followUpLines, "- Follow-up goal: "+followUpGoal)
				}
				prefix = append(prefix, strings.Join(followUpLines, "\n"))
			}
			if startupPrompt != "" {
				prefix = append(prefix, strings.TrimRight(startupPrompt, "\n"))
			}
			if len(accessiblePaths) > 0 {
				prefix = append(prefix, "", "Accessible paths:")
				for _, path := range accessiblePaths {
					prefix = append(prefix, "- "+path)
				}
			}
			if len(taskAccess["read"])+len(taskAccess["write"])+len(taskAccess["deny"]) > 0 {
				prefix = append(prefix, "", "Access policy:")
				for _, mode := range []string{"read", "write", "deny"} {
					if len(taskAccess[mode]) == 0 {
						continue
					}
					prefix = append(prefix, mode+":")
					for _, path := range taskAccess[mode] {
						prefix = append(prefix, "- "+path)
					}
				}
			}
			if includeAgentsMD {
				agentsPath := filepath.Join(root, "AGENTS.md")
				if rawAgents, err := os.ReadFile(agentsPath); err == nil {
					prefix = append(prefix, "", "AGENTS.md context:", "", strings.TrimRight(string(rawAgents), "\n"))
				}
			}
			inputContext := renderInputContext(taskInputs, stringValue(laneSelection["provider"]), root, artifactRoot, sharedDir, inlineInputContext, inlineInputMaxBytes, inlineInputTotalMaxBytes)
			if inputContext != "" && !localLane {
				prefix = append(prefix, "", strings.TrimRight(inputContext, "\n"))
			}
			if len(prefix) > 0 {
				if localLane {
					prompt = strings.TrimRight(prompt, "\n") + "\n\n" + strings.TrimSpace(strings.Join(prefix, "\n")) + "\n"
				} else {
					prompt = strings.TrimRight(strings.Join(prefix, "\n"), "\n") + "\n\n" + strings.TrimLeft(prompt, "\n")
				}
			}
			if inputContext != "" && localLane {
				prompt = strings.TrimRight(prompt, "\n") + "\n\nReference context excerpts:\n\n" + strings.TrimRight(inputContext, "\n") + "\n"
			}
			if err := os.WriteFile(filepath.Join(taskDir, "prompt.md"), []byte(prompt), 0o644); err != nil {
				return err
			}
			graphTasks = append(graphTasks, map[string]any{
				"id":             taskID,
				"base_task_id":   variant["base_task_id"],
				"comparison":     laneSelection["comparison"],
				"depends_on":     dependsOn,
				"resolver_hint":  taskPayload["resolver_hint"],
				"lane_id":        laneSelection["lane_id"],
				"provider":       laneSelection["provider"],
				"model":          laneSelection["model"],
				"output_path":    taskPayload["output_path"],
				"worker_type":    taskPayload["worker_type"],
				"reuse_existing": reuseExisting,
			})
		}
	}
	mergeID := fallbackString(stringValue(merge["id"]), "merge_final")
	verifyID := fallbackString(stringValue(verify["id"]), "verify_final")
	graphTasks = append(graphTasks, map[string]any{"id": mergeID, "depends_on": workerIDs, "worker_type": "merge"})
	graphTasks = append(graphTasks, map[string]any{"id": verifyID, "depends_on": []string{mergeID}, "worker_type": "verify"})
	graphPayload := map[string]any{"concurrency": planPayload["concurrency"], "tasks": graphTasks}
	if followUpMode {
		graphPayload["follow_up"] = requestPayload["follow_up"]
	}
	if err := writeJSONFile(graphJSON, graphPayload); err != nil {
		return err
	}
	return writeJSONFile(lanePlanJSON, lanePlan)
}

func runTasks(graphPath, runner string, env map[string]string, stderr io.Writer) error {
	graph := readJSONMap(graphPath)
	concurrency := mapValue(graph["concurrency"])
	maxWorkers := maxInt(1, intFromAny(fallbackAny(concurrency["max_workers"], 1)))
	maxLocalWorkers := maxInt(1, intFromAny(fallbackAny(concurrency["max_local_workers"], maxWorkers)))
	maxCloudWorkers := maxInt(1, intFromAny(fallbackAny(concurrency["max_cloud_workers"], 1)))
	tasks := map[string]schedulerTask{}
	for _, raw := range listValue(graph["tasks"]) {
		item := mapValue(raw)
		workerType := fallbackString(stringValue(item["worker_type"]), "analysis")
		taskID := stringValue(item["id"])
		if taskID == "" || workerType == "merge" || workerType == "verify" {
			continue
		}
		tasks[taskID] = schedulerTask{
			ID:            taskID,
			BaseTaskID:    fallbackString(stringValue(item["base_task_id"]), taskID),
			Comparison:    mapValue(item["comparison"]),
			DependsOn:     stringList(item["depends_on"]),
			Provider:      stringValue(item["provider"]),
			Model:         stringValue(item["model"]),
			OutputPath:    stringValue(item["output_path"]),
			ReuseExisting: boolAny(item["reuse_existing"]),
		}
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no runnable worker tasks in %s", graphPath)
	}
	animationPref := strings.ToLower(fallbackString(env["DORKPIPE_ORCH_COMPARE_ANIMATION"], "auto"))
	renderer := strings.ToLower(fallbackString(env["DORKPIPE_ORCH_COMPARE_RENDERER"], "clear"))
	workerLogMode := strings.ToLower(fallbackString(env["DORKPIPE_ORCH_COMPARE_WORKER_LOGS"], "artifact"))
	renderInterval := floatFromString(fallbackString(env["DORKPIPE_ORCH_COMPARE_ANIMATION_INTERVAL"], "0.35"), 0.35)
	hasComparison := false
	for _, task := range tasks {
		if boolAny(task.Comparison["enabled"]) {
			hasComparison = true
			break
		}
	}
	animationEnabled := hasComparison && animationPref != "false" && animationPref != "0"
	if animationPref == "auto" {
		animationEnabled = animationEnabled && isTerminal(os.Stderr)
	}
	if renderer != "clear" && renderer != "inline" {
		renderer = "clear"
	}
	if workerLogMode != "artifact" && workerLogMode != "terminal" {
		workerLogMode = "artifact"
	}
	cloudProviders := map[string]bool{"codex": true, "claude": true}
	type taskExit struct {
		taskID string
		err    error
	}
	running := map[string]*exec.Cmd{}
	runningTasks := map[string]schedulerTask{}
	runningLogs := map[string]*os.File{}
	exitCh := make(chan taskExit, len(tasks))
	done := map[string]bool{}
	failed := map[string]string{}
	started := map[string]bool{}
	startedAt := map[string]time.Time{}
	finishedAt := map[string]time.Time{}
	taskResults := map[string]map[string]any{}
	lastRender := time.Time{}
	frameIndex := 0
	renderedLines := 0

	readTaskResult := func(taskID string) map[string]any {
		if cached, ok := taskResults[taskID]; ok {
			return cached
		}
		root := env["DORKPIPE_ORCH_TASKS_DIR"]
		if root == "" {
			return map[string]any{}
		}
		payload := readJSONMapOptional(filepath.Join(root, taskID, "result.json"))
		taskResults[taskID] = payload
		return payload
	}
	for taskID, task := range tasks {
		if !task.ReuseExisting {
			continue
		}
		result := readTaskResult(taskID)
		if len(result) == 0 {
			failed[taskID] = "follow-up reuse requested but no existing result.json was found"
			continue
		}
		if strings.EqualFold(strings.TrimSpace(stringValue(result["status"])), "failed") {
			failed[taskID] = "follow-up reuse requested but the existing result is failed"
			continue
		}
		done[taskID] = true
		started[taskID] = true
		if !animationEnabled {
			fmt.Fprintf(stderr, "[dorkpipe] reusing existing orchestration task %s\n", taskID)
		}
	}
	taskStatus := func(taskID string) string {
		if _, ok := failed[taskID]; ok {
			return "failed"
		}
		if done[taskID] {
			return "done"
		}
		if _, ok := running[taskID]; ok {
			return "running"
		}
		if started[taskID] {
			return "started"
		}
		return "queued"
	}
	comparisonGroups := func() map[string][]string {
		out := map[string][]string{}
		for taskID, task := range tasks {
			if !boolAny(task.Comparison["enabled"]) {
				continue
			}
			base := fallbackString(task.BaseTaskID, fallbackString(stringValue(task.Comparison["base_task_id"]), taskID))
			out[base] = append(out[base], taskID)
		}
		filtered := map[string][]string{}
		for base, ids := range out {
			if len(ids) >= 2 {
				sort.Strings(ids)
				filtered[base] = ids
			}
		}
		return filtered
	}
	comparisonTaskIDs := func() map[string]bool {
		out := map[string]bool{}
		for _, group := range comparisonGroups() {
			for _, taskID := range group {
				out[taskID] = true
			}
		}
		return out
	}
	localSummary := func() string {
		comparisonIDs := comparisonTaskIDs()
		parts := []string{}
		keys := sortedTaskIDs(tasks)
		for _, taskID := range keys {
			if comparisonIDs[taskID] {
				continue
			}
			task := tasks[taskID]
			parts = append(parts, fmt.Sprintf("%s:%s:%s", taskID, fighterLabel(task), taskStatus(taskID)))
		}
		if len(parts) == 0 {
			return ""
		}
		return "local scout " + strings.Join(parts, "  ")
	}
	repaint := func(lines []string) {
		if renderer == "clear" {
			fmt.Fprint(stderr, "\033[?25l\033[2J\033[H")
		} else if renderedLines > 0 {
			fmt.Fprintf(stderr, "\033[%dF", renderedLines)
			for i := 0; i < renderedLines; i++ {
				fmt.Fprint(stderr, "\033[2K\033[1E")
			}
			fmt.Fprintf(stderr, "\033[%dF", renderedLines)
		} else {
			fmt.Fprint(stderr, "\033[?25l")
		}
		fmt.Fprint(stderr, strings.Join(lines, "\n")+"\n")
		renderedLines = len(lines)
	}
	renderFight := func(force bool) {
		if !animationEnabled {
			return
		}
		now := time.Now()
		if !force && now.Sub(lastRender) < time.Duration(renderInterval*float64(time.Second)) {
			return
		}
		lastRender = now
		frameIndex++
		lines := []string{"DorkPipe comparison lanes", "=========================", ""}
		groupKeys := make([]string, 0, len(comparisonGroups()))
		groups := comparisonGroups()
		for base := range groups {
			groupKeys = append(groupKeys, base)
		}
		sort.Strings(groupKeys)
		for _, base := range groupKeys {
			lines = append(lines, base)
			ids := groups[base]
			sort.Slice(ids, func(i, j int) bool {
				return tasks[ids[i]].Provider < tasks[ids[j]].Provider
			})
			for _, taskID := range ids {
				status := taskStatus(taskID)
				lines = append(lines, fmt.Sprintf("  %-18s %s %-7s %s %s", fighterLabel(tasks[taskID]), fighterBar(status, frameIndex), status, formatElapsed(startedAt[taskID], finishedAt[taskID], now), formatTokens(readTaskResult(taskID))))
			}
			lines = append(lines, "            VS", "")
		}
		if scout := localSummary(); scout != "" {
			lines = append(lines, scout)
		}
		comparisonIDs := comparisonTaskIDs()
		comparisonDone := 0
		for taskID := range comparisonIDs {
			if done[taskID] {
				comparisonDone++
			}
		}
		lines = append(lines, fmt.Sprintf("comparison %d/%d  total %d/%d  failed %d  running %d", comparisonDone, len(comparisonIDs), len(done), len(tasks), len(failed), len(running)))
		repaint(lines)
	}
	closeFight := func() {
		if animationEnabled {
			renderFight(true)
			fmt.Fprint(stderr, "\033[?25h\n")
		}
	}
	printFailureLog := func(taskID string) {
		path := workerLogPath(env["DORKPIPE_ORCH_TASKS_DIR"], taskID)
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if len(lines) == 0 || lines[0] == "" {
			return
		}
		fmt.Fprintf(stderr, "[dorkpipe] worker log tail for %s (%s):\n", taskID, path)
		start := 0
		if len(lines) > 40 {
			start = len(lines) - 40
		}
		for _, line := range lines[start:] {
			fmt.Fprintln(stderr, line)
		}
	}
	activeCounts := func() (int, int, int) {
		total := len(running)
		cloud := 0
		for _, task := range runningTasks {
			if cloudProviders[task.Provider] {
				cloud++
			}
		}
		return total, total - cloud, cloud
	}
	runnable := func() []schedulerTask {
		out := []schedulerTask{}
		total, local, cloud := activeCounts()
		for _, taskID := range sortedTaskIDs(tasks) {
			task := tasks[taskID]
			if done[taskID] || started[taskID] {
				continue
			}
			if _, failedAlready := failed[taskID]; failedAlready {
				continue
			}
			depFailed := false
			depsDone := true
			for _, dep := range task.DependsOn {
				if _, ok := failed[dep]; ok {
					failed[taskID] = "dependency failed"
					depFailed = true
					break
				}
				if _, depTask := tasks[dep]; depTask && !done[dep] {
					depsDone = false
				}
			}
			if depFailed || !depsDone {
				continue
			}
			if hasSchedulerOutputConflict(task, runningTasks) {
				continue
			}
			if total >= maxWorkers {
				break
			}
			if cloudProviders[task.Provider] {
				if cloud >= maxCloudWorkers {
					continue
				}
				cloud++
			} else {
				if local >= maxLocalWorkers {
					continue
				}
				local++
			}
			total++
			out = append(out, task)
		}
		return out
	}
	for len(done)+len(failed) < len(tasks) {
		launched := false
		for _, task := range runnable() {
			taskID := task.ID
			var stdoutDest io.Writer
			var stderrDest io.Writer
			var logHandle *os.File
			if animationEnabled && workerLogMode == "artifact" {
				logPath := workerLogPath(env["DORKPIPE_ORCH_TASKS_DIR"], taskID)
				if logPath != "" {
					if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
						return err
					}
					handle, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
					if err != nil {
						return err
					}
					logHandle = handle
					_, _ = logHandle.WriteString(fmt.Sprintf("[dorkpipe] starting %s (%s)\n", taskID, fallbackString(task.Provider, "unknown")))
					stdoutDest = logHandle
					stderrDest = logHandle
				}
			}
			cmd := exec.Command("bash", runner, taskID)
			cmd.Env = envList(env)
			if stdoutDest != nil {
				cmd.Stdout = stdoutDest
				cmd.Stderr = stderrDest
			} else {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}
			if err := cmd.Start(); err != nil {
				if logHandle != nil {
					_ = logHandle.Close()
				}
				return err
			}
			go func(taskID string, cmd *exec.Cmd) {
				exitCh <- taskExit{taskID: taskID, err: cmd.Wait()}
			}(taskID, cmd)
			running[taskID] = cmd
			runningTasks[taskID] = task
			if logHandle != nil {
				runningLogs[taskID] = logHandle
			}
			started[taskID] = true
			startedAt[taskID] = time.Now()
			launched = true
			if !animationEnabled {
				fmt.Fprintf(stderr, "[dorkpipe] started orchestration task %s (%s)\n", taskID, fallbackString(task.Provider, "unknown"))
			}
			renderFight(true)
		}
		if len(running) == 0 && !launched {
			blocked := []string{}
			for _, taskID := range sortedTaskIDs(tasks) {
				if !done[taskID] {
					if _, failedAlready := failed[taskID]; !failedAlready {
						blocked = append(blocked, taskID)
					}
				}
			}
			return fmt.Errorf("orchestration scheduler stalled; blocked tasks: %s", strings.Join(blocked, ", "))
		}
		if len(running) == 0 {
			continue
		}
		renderFight(false)
		item := <-exitCh
		finished := []taskExit{item}
	drain:
		for {
			select {
			case next := <-exitCh:
				finished = append(finished, next)
			default:
				break drain
			}
		}
		for _, item := range finished {
			taskID := item.taskID
			if _, ok := running[taskID]; !ok {
				continue
			}
			delete(running, taskID)
			task := runningTasks[taskID]
			delete(runningTasks, taskID)
			if logHandle := runningLogs[taskID]; logHandle != nil {
				statusCode := 0
				if item.err != nil {
					statusCode = 1
					if exitErr, ok := item.err.(*exec.ExitError); ok {
						statusCode = exitErr.ExitCode()
					}
				}
				_, _ = logHandle.WriteString(fmt.Sprintf("[dorkpipe] finished %s with exit status %d\n", taskID, statusCode))
				_ = logHandle.Close()
				delete(runningLogs, taskID)
			}
			finishedAt[taskID] = time.Now()
			if item.err == nil {
				done[taskID] = true
				if !animationEnabled {
					fmt.Fprintf(stderr, "[dorkpipe] completed orchestration task %s\n", taskID)
				}
			} else {
				reason := "exit status 1"
				if exitErr, ok := item.err.(*exec.ExitError); ok {
					reason = fmt.Sprintf("exit status %d", exitErr.ExitCode())
				}
				failed[taskID] = reason
				if !animationEnabled {
					fmt.Fprintf(stderr, "[dorkpipe] failed orchestration task %s: %s\n", taskID, reason)
				}
			}
			_ = task
			renderFight(true)
		}
	}
	if len(failed) > 0 {
		closeFight()
		if animationEnabled {
			keys := make([]string, 0, len(failed))
			for key := range failed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, taskID := range keys {
				printFailureLog(taskID)
			}
		}
		keys := make([]string, 0, len(failed))
		for key := range failed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := []string{}
		for _, taskID := range keys {
			parts = append(parts, fmt.Sprintf("%s (%s)", taskID, failed[taskID]))
		}
		return fmt.Errorf("orchestration task failure(s): %s", strings.Join(parts, ", "))
	}
	closeFight()
	fmt.Fprintf(stderr, "[dorkpipe] ran %d orchestration task(s) with max_workers=%d max_local_workers=%d max_cloud_workers=%d\n", len(done), maxWorkers, maxLocalWorkers, maxCloudWorkers)
	return nil
}

func optimizeAction(action, rootPath, targetDirPath, optimizerDirPath, orchRootPath, approvalPath, resultPath string, env map[string]string) error {
	root, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	targetDir, err := filepath.Abs(targetDirPath)
	if err != nil {
		return err
	}
	optimizerDir, err := filepath.Abs(optimizerDirPath)
	if err != nil {
		return err
	}
	orchRoot, err := filepath.Abs(orchRootPath)
	if err != nil {
		return err
	}
	targetWorkflowConfig := filepath.Join(root, "workflows", "agent", "docs.orchestrate", "config.yml")
	verifierScript := filepath.Join(root, "packages", "dorkpipe", "resolvers", "dorkpipe", "assets", "scripts", "orchestrate-verify-results.sh")
	patchPath := filepath.Join(optimizerDir, "proposed.patch")
	assessmentMD := filepath.Join(optimizerDir, "assessment.md")
	recommendationMD := filepath.Join(optimizerDir, "recommendation.md")
	historyDir := filepath.Join(optimizerDir, "history")
	allowedFiles := []string{
		filepath.Join(root, "workflows", "agent", "docs.optimize-orchestrate", "README.md"),
		filepath.Join(root, "workflows", "agent", "docs.optimize-orchestrate", "config.yml"),
		targetWorkflowConfig,
		filepath.Join(root, "packages", "dorkpipe", "resolvers", "dorkpipe", "assets", "scripts", "orchestrate-optimize.sh"),
		verifierScript,
	}
	allowedFileSet := map[string]bool{}
	for _, path := range allowedFiles {
		if abs, absErr := filepath.Abs(path); absErr == nil {
			allowedFileSet[abs] = true
		}
	}
	displayPath := func(path string) string {
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return path
		}
		if rel, relErr := filepath.Rel(root, abs); relErr == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
		return abs
	}
	readText := func(path string) string {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return ""
		}
		return string(raw)
	}
	writeResult := func(payload map[string]any) error {
		return writeJSONFile(resultPath, payload)
	}
	codexResponsePath := filepath.Join(orchRoot, "tasks", "codex_patch_decision", "response.md")
	extractUnifiedDiff := func(text string) string {
		fenceRE := regexp.MustCompile("(?is)```(?:diff|patch)?\\s*\\n(.*?)```")
		matches := fenceRE.FindAllStringSubmatch(text, -1)
		candidates := []string{}
		for _, match := range matches {
			if len(match) >= 2 {
				candidates = append(candidates, match[1])
			}
		}
		if len(candidates) == 0 {
			candidates = append(candidates, text)
		}
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate) + "\n"
			if strings.Contains(candidate, "--- a/") && strings.Contains(candidate, "+++ b/") && strings.Contains(candidate, "@@") {
				return candidate
			}
		}
		return ""
	}
	validatePatchText := func(text string) (bool, string, []string) {
		if strings.TrimSpace(text) == "" {
			return false, "codex response did not include a unified diff", nil
		}
		touched := []string{}
		for _, line := range strings.Split(text, "\n") {
			if !strings.HasPrefix(line, "--- a/") && !strings.HasPrefix(line, "+++ b/") {
				continue
			}
			path := strings.TrimPrefix(strings.TrimPrefix(line, "--- a/"), "+++ b/")
			if path == "/dev/null" {
				continue
			}
			candidate, absErr := filepath.Abs(filepath.Join(root, path))
			if absErr != nil || !allowedFileSet[candidate] {
				return false, "patch touches non-allowlisted path: " + path, nil
			}
			touched = appendIfMissing(touched, path)
		}
		if len(touched) == 0 {
			return false, "patch did not declare any allowlisted file paths", nil
		}
		cmd := exec.Command("git", "apply", "--recount", "--check", "-")
		cmd.Dir = root
		cmd.Env = envList(env)
		cmd.Stdin = strings.NewReader(text)
		output, runErr := cmd.CombinedOutput()
		if runErr != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = "git apply --check failed"
			}
			return false, message, touched
		}
		return true, "", touched
	}
	applyEnabled := func() bool {
		return boolString(env["DORKPIPE_OPTIMIZER_APPLY"])
	}
	snapshotPreviousOptimizerRun := func() error {
		if err := os.MkdirAll(historyDir, 0o755); err != nil {
			return err
		}
		copies := [][2]string{
			{recommendationMD, filepath.Join(historyDir, "previous-recommendation.md")},
			{patchPath, filepath.Join(historyDir, "previous-proposed.patch")},
			{filepath.Join(optimizerDir, "propose", "result.json"), filepath.Join(historyDir, "previous-propose-result.json")},
			{codexResponsePath, filepath.Join(historyDir, "previous-codex-response.md")},
			{filepath.Join(orchRoot, "merge", "final.md"), filepath.Join(historyDir, "previous-merge-final.md")},
			{filepath.Join(orchRoot, "verify", "result.json"), filepath.Join(historyDir, "previous-verify-result.json")},
		}
		snapshot := []string{}
		for _, pair := range copies {
			raw, readErr := os.ReadFile(pair[0])
			if readErr != nil {
				continue
			}
			if err := os.WriteFile(pair[1], raw, 0o644); err != nil {
				return err
			}
			snapshot = append(snapshot, displayPath(pair[1]))
		}
		lines := []string{"# Previous Optimizer Run", ""}
		if len(snapshot) == 0 {
			lines = append(lines, "- No previous optimizer artifacts were available.")
		} else {
			for _, item := range snapshot {
				lines = append(lines, "- `"+item+"`")
			}
		}
		lines = append(lines, "")
		return os.WriteFile(filepath.Join(historyDir, "previous-run-summary.md"), []byte(strings.Join(lines, "\n")), 0o644)
	}
	collectIssues := func() ([]string, map[string]any, []string) {
		verifyPath := filepath.Join(targetDir, "verify", "result.json")
		responses := []string{}
		if entries, globErr := filepath.Glob(filepath.Join(targetDir, "tasks", "*", "response.md")); globErr == nil {
			sort.Strings(entries)
			responses = entries
		}
		issues := []string{}
		verify := map[string]any{}
		if _, statErr := os.Stat(verifyPath); statErr == nil {
			verify = readJSONMap(verifyPath)
			for _, issue := range stringList(verify["issues"]) {
				issues = append(issues, "verifier: "+issue)
			}
		} else {
			issues = append(issues, "missing target verify artifact: "+displayPath(verifyPath))
		}
		smellPatterns := []struct {
			re    *regexp.Regexp
			label string
		}{
			{regexp.MustCompile(`(?im)^\s*(?:Note|Please note)\s*:`), "note/footer instead of direct artifact content"},
			{regexp.MustCompile(`(?im)^\s*Here (?:are|is)\b`), "preamble before requested artifact"},
			{regexp.MustCompile(`(?i)\bcould not be completed due to lack of information\b`), "false missing-information footer"},
			{regexp.MustCompile(`(?i)\badheres to (?:the )?(?:specified )?formatting\b`), "formatting commentary"},
		}
		for _, response := range responses {
			text := readText(response)
			taskID := filepath.Base(filepath.Dir(response))
			for _, pattern := range smellPatterns {
				if pattern.re.MatchString(text) {
					issues = append(issues, taskID+": "+pattern.label)
					break
				}
			}
		}
		return issues, verify, responses
	}
	writeAssessment := func() error {
		issues, verify, responses := collectIssues()
		lines := []string{
			"# DorkPipe Ollama Optimizer Assessment",
			"",
			"- Target artifact root: `" + displayPath(targetDir) + "`",
			"- Target verify status: `" + fallbackString(stringValue(verify["status"]), "missing") + "`",
			"- Target confidence: `" + fallbackString(stringValue(verify["confidence"]), "unknown") + "`",
			fmt.Sprintf("- Response artifacts inspected: %d", len(responses)),
			"",
			"## Findings",
			"",
		}
		if len(issues) == 0 {
			lines = append(lines, "- No known optimizer smell patterns found in the latest target run.")
		} else {
			for _, issue := range issues {
				lines = append(lines, "- "+issue)
			}
		}
		lines = append(lines,
			"",
			"## Optimizer Policy",
			"",
			"- Keep this loop local-first with Ollama workers.",
			"- Let Codex make the code-change decision.",
			"- Write proposed patch artifacts only; never modify the working tree in proposal mode.",
			"- Restrict edits to the docs orchestration workflow and DorkPipe verifier heuristics.",
			"",
		)
		if err := os.WriteFile(assessmentMD, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
		return writeResult(map[string]any{
			"status":      "ready",
			"target_root": displayPath(targetDir),
			"issues":      issues,
			"assessment":  displayPath(assessmentMD),
		})
	}
	writePatch := func() error {
		if err := writeAssessment(); err != nil {
			return err
		}
		responseText := readText(codexResponsePath)
		patchText := extractUnifiedDiff(responseText)
		valid, validationError, changedFiles := validatePatchText(patchText)
		if valid {
			if err := os.WriteFile(patchPath, []byte(patchText), 0o644); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(patchPath, []byte(""), 0o644); err != nil {
				return err
			}
		}
		scopeLines := []string{}
		if len(changedFiles) == 0 {
			scopeLines = append(scopeLines, "- No valid patch proposed.")
		} else {
			for _, item := range changedFiles {
				scopeLines = append(scopeLines, "- `"+item+"`")
			}
		}
		recommendation := strings.Join([]string{
			"# DorkPipe Codex Optimizer Recommendation",
			"",
			"Codex-authored patch proposal. Review this artifact before applying anything to the working tree.",
			"",
			"## Proposed Scope",
			"",
			strings.Join(scopeLines, "\n"),
			"",
			"## Why",
			"",
			"- Codex owns the code-change decision in this workflow.",
			"- DorkPipe validates the diff path allowlist and `git apply --check` only.",
			"- Proposal mode never modifies the working tree and never commits.",
			"",
		}, "\n")
		if err := os.WriteFile(recommendationMD, []byte(recommendation), 0o644); err != nil {
			return err
		}
		status := "review"
		if valid {
			status = "ready"
		}
		return writeResult(map[string]any{
			"status":           status,
			"patch":            displayPath(patchPath),
			"recommendation":   displayPath(recommendationMD),
			"codex_response":   displayPath(codexResponsePath),
			"changed_files":    changedFiles,
			"validation_error": validationError,
			"applied":          false,
		})
	}
	checkPatchPaths := func() error {
		text := readText(patchPath)
		for _, line := range strings.Split(text, "\n") {
			if !strings.HasPrefix(line, "--- a/") && !strings.HasPrefix(line, "+++ b/") {
				continue
			}
			path := strings.TrimPrefix(strings.TrimPrefix(line, "--- a/"), "+++ b/")
			if path == "/dev/null" {
				continue
			}
			candidate, absErr := filepath.Abs(filepath.Join(root, path))
			if absErr != nil || !allowedFileSet[candidate] {
				return fmt.Errorf("patch touches non-allowlisted path: %s", path)
			}
		}
		return nil
	}
	applyPatch := func() error {
		if !applyEnabled() {
			return writeResult(map[string]any{
				"status": "skipped",
				"reason": "set DORKPIPE_OPTIMIZER_APPLY=1 to apply the proposed patch to the working tree",
				"patch":  displayPath(patchPath),
				"commit": false,
			})
		}
		if err := checkPatchPaths(); err != nil {
			return err
		}
		text := readText(patchPath)
		if strings.TrimSpace(text) == "" {
			return writeResult(map[string]any{"status": "noop", "reason": "proposed patch is empty"})
		}
		checkCmd := exec.Command("git", "apply", "--recount", "--check", patchPath)
		checkCmd.Dir = root
		checkCmd.Env = envList(env)
		if output, runErr := checkCmd.CombinedOutput(); runErr != nil {
			return errors.New(strings.TrimSpace(string(output)))
		}
		applyCmd := exec.Command("git", "apply", "--recount", patchPath)
		applyCmd.Dir = root
		applyCmd.Env = envList(env)
		if output, runErr := applyCmd.CombinedOutput(); runErr != nil {
			return errors.New(strings.TrimSpace(string(output)))
		}
		appliedFiles := []string{}
		for _, path := range allowedFiles {
			if rel, relErr := filepath.Rel(root, path); relErr == nil {
				appliedFiles = append(appliedFiles, filepath.ToSlash(rel))
			}
		}
		return writeResult(map[string]any{
			"status":        "applied",
			"patch":         displayPath(patchPath),
			"applied_files": appliedFiles,
			"commit":        false,
		})
	}
	validate := func() error {
		dockpipeBin := filepath.Join(root, "src", "bin", "dockpipe")
		if runtime.GOOS == "windows" {
			if _, statErr := os.Stat(dockpipeBin + ".exe"); statErr == nil {
				dockpipeBin += ".exe"
			}
		}
		commands := [][]string{
			{dockpipeBin, "workflow", "validate", "workflows/agent/docs.optimize-orchestrate/config.yml"},
			{dockpipeBin, "workflow", "validate", "workflows/agent/docs.orchestrate/config.yml"},
		}
		results := []map[string]any{}
		ok := true
		for _, command := range commands {
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Dir = root
			cmd.Env = envList(env)
			output, runErr := cmd.CombinedOutput()
			exitCode := 0
			if runErr != nil {
				ok = false
				exitCode = 1
				if exitErr, isExit := runErr.(*exec.ExitError); isExit {
					exitCode = exitErr.ExitCode()
				}
			}
			out := string(output)
			if len(out) > 4000 {
				out = out[len(out)-4000:]
			}
			results = append(results, map[string]any{
				"command":   strings.Join(command, " "),
				"exit_code": exitCode,
				"output":    out,
			})
		}
		status := "fail"
		if ok {
			status = "pass"
		}
		if err := writeResult(map[string]any{"status": status, "results": results}); err != nil {
			return err
		}
		if !ok {
			return errors.New("optimizer validation failed")
		}
		return nil
	}
	switch action {
	case "prepare", "assess":
		if err := snapshotPreviousOptimizerRun(); err != nil {
			return err
		}
		return writeAssessment()
	case "propose":
		return writePatch()
	case "apply", "apply-if-enabled":
		return applyPatch()
	case "validate":
		_ = approvalPath
		return validate()
	default:
		return fmt.Errorf("unknown action %s", action)
	}
}

func readJSONMap(path string) map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func readJSONMapOptional(path string) map[string]any { return readJSONMap(path) }

func readYAMLMap(path string) map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var payload map[string]any
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func readYAMLMapOptional(path string) map[string]any { return readYAMLMap(path) }

func writeJSONFile(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func mapValue(v any) map[string]any {
	switch typed := v.(type) {
	case map[string]any:
		return typed
	case map[any]any:
		out := map[string]any{}
		for key, value := range typed {
			out[fmt.Sprint(key)] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func listValue(v any) []any {
	switch typed := v.(type) {
	case []any:
		return typed
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return []any{}
	}
}

func stringList(v any) []string {
	out := []string{}
	for _, item := range listValue(v) {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func stringValue(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func intFromAny(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	case string:
		return intFromString(typed, 0)
	case bool:
		if typed {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func intFromString(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func intAny(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
		asFloat, err := typed.Float64()
		if err == nil {
			return int(asFloat)
		}
		return 0
	case string:
		return intFromString(typed, 0)
	default:
		return 0
	}
}

func floatFromString(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatAny(v any) float64 {
	return floatDefault(v, 0)
}

func floatDefault(v any, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	switch typed := v.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		return floatFromString(typed, fallback)
	default:
		return fallback
	}
}

func boolString(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boolAny(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		return boolString(typed)
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func boolDefault(v any, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return boolAny(v)
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func fallbackAny(value, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}

func decodeJSONMapString(raw string) (map[string]any, error) {
	var out map[string]any
	err := json.Unmarshal([]byte(raw), &out)
	return out, err
}

func decodeJSONAny(raw string, fallback any) any {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return fallback
	}
	return out
}

func mustJSON(value any, fallback any) string {
	if value == nil {
		value = fallback
	}
	raw, err := json.Marshal(value)
	if err != nil {
		raw, _ = json.Marshal(fallback)
	}
	return string(raw)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func truncateUTF8(raw []byte, max int) []byte {
	if len(raw) <= max {
		return raw
	}
	for max > 0 && (raw[max]&0xC0) == 0x80 {
		max--
	}
	if max <= 0 {
		return []byte{}
	}
	return raw[:max]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func withinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendIfMissing(values []string, target string) []string {
	if containsString(values, target) {
		return values
	}
	return append(values, target)
}

func expandEnv(text string, env map[string]string) string {
	return reEnvExpand.ReplaceAllStringFunc(text, func(match string) string {
		parts := reEnvExpand.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		if value, ok := env[parts[1]]; ok {
			return value
		}
		return parts[3]
	})
}

func splitCSVLower(raw string) []string {
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSVTrim(raw string) []string {
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func envInt(env map[string]string, key string) int {
	raw := strings.TrimSpace(env[key])
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func detectHostMemoryGB() int {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "MemTotal:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			kb, err := strconv.Atoi(fields[1])
			if err != nil || kb <= 0 {
				return 0
			}
			return int((int64(kb) + 1024*1024 - 1) / (1024 * 1024))
		}
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil || bytes <= 0 {
			return 0
		}
		return int((bytes + (1 << 30) - 1) / (1 << 30))
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command", "[math]::Ceiling((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB)").Output()
		if err != nil {
			return 0
		}
		value, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil || value <= 0 {
			return 0
		}
		return value
	}
	return 0
}

func detectLocalHostProfile(env map[string]string) localHostProfile {
	profile := localHostProfile{
		MemoryGB: envInt(env, "DORKPIPE_ORCH_HOST_MEMORY_GB"),
		CPUCores: envInt(env, "DORKPIPE_ORCH_HOST_CPU_CORES"),
	}
	if profile.CPUCores == 0 {
		profile.CPUCores = runtime.NumCPU()
	}
	if profile.MemoryGB == 0 {
		profile.MemoryGB = detectHostMemoryGB()
	}
	acceleration := strings.ToLower(strings.TrimSpace(env["DORKPIPE_ORCH_LOCAL_ACCELERATION"]))
	if acceleration == "" {
		switch strings.ToLower(strings.TrimSpace(env["DORKPIPE_DEV_STACK_GPU"])) {
		case "nvidia", "gpu", "all":
			acceleration = "gpu"
		case "cpu", "none", "off", "false", "0":
			acceleration = "cpu"
		}
	}
	if acceleration == "" {
		if commandAvailable("nvidia-smi") {
			acceleration = "gpu"
		} else {
			acceleration = "cpu"
		}
	}
	profile.Acceleration = acceleration
	profile.Tier = strings.ToLower(strings.TrimSpace(env["DORKPIPE_ORCH_LOCAL_HARDWARE_TIER"]))
	if profile.Tier == "" {
		switch {
		case profile.Acceleration == "gpu" || profile.MemoryGB >= 48 || profile.CPUCores >= 16:
			profile.Tier = "high"
		case profile.MemoryGB >= 24 || profile.CPUCores >= 8:
			profile.Tier = "medium"
		default:
			profile.Tier = "low"
		}
	}
	return profile
}

func estimateModelMemoryGB(model string) float64 {
	matches := regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)b\b`).FindStringSubmatch(model)
	if len(matches) < 2 {
		return 0
	}
	paramsB, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || paramsB <= 0 {
		return 0
	}
	required := paramsB * 1.2
	if required < 4 {
		required = 4
	}
	return required
}

func loadModelLanes(path string, env map[string]string) []map[string]any {
	raw := readYAMLMapOptional(path)
	out := []map[string]any{}
	for _, laneRaw := range listValue(raw["lanes"]) {
		lane := mapValue(laneRaw)
		if stringValue(lane["id"]) == "" {
			continue
		}
		item := map[string]any{}
		for key, value := range lane {
			item[key] = value
		}
		item["model"] = expandEnv(stringValue(item["model"]), env)
		commands := stringList(mapValue(item["availability"])["commands"])
		missing := []string{}
		for _, command := range commands {
			if !commandAvailable(command) {
				missing = append(missing, command)
			}
		}
		item["available"] = len(missing) == 0
		item["missing_commands"] = missing
		out = append(out, item)
	}
	return out
}

func commandAvailable(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return true
	}
	if _, err := exec.LookPath(command); err == nil {
		return true
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(command), ".exe") {
		if _, err := exec.LookPath(command + ".exe"); err == nil {
			return true
		}
	}
	for _, dir := range executableSearchPathEntries(os.Getenv("PATH")) {
		for _, candidate := range executableCandidateNames(command) {
			info, err := os.Stat(filepath.Join(dir, candidate))
			if err == nil && !info.IsDir() {
				return true
			}
		}
	}
	if runtime.GOOS == "windows" {
		for _, dir := range windowsExecutableFallbackDirs(command) {
			for _, candidate := range executableCandidateNames(command) {
				info, err := os.Stat(filepath.Join(dir, candidate))
				if err == nil && !info.IsDir() {
					return true
				}
			}
		}
	}
	return false
}

func windowsExecutableFallbackDirs(command string) []string {
	switch strings.ToLower(strings.TrimSuffix(command, ".exe")) {
	case "docker", "docker-compose":
		return []string{
			`C:\Program Files\Docker\Docker\resources\bin`,
			`C:\ProgramData\DockerDesktop\version-bin`,
		}
	default:
		return nil
	}
}

func executableCandidateNames(command string) []string {
	if runtime.GOOS != "windows" || strings.HasSuffix(strings.ToLower(command), ".exe") {
		return []string{command}
	}
	return []string{command, command + ".exe"}
}

func executableSearchPathEntries(pathValue string) []string {
	if pathValue == "" {
		return nil
	}
	delimiter := string(os.PathListSeparator)
	if runtime.GOOS == "windows" && !strings.Contains(pathValue, ";") && strings.Contains(pathValue, ":/") {
		delimiter = ":"
	}
	parts := strings.Split(pathValue, delimiter)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, normalizeExecutableSearchPath(part))
	}
	return out
}

func normalizeExecutableSearchPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == '/' && isASCIILetter(path[1]) {
		return strings.ToUpper(string(path[1])) + ":" + filepath.FromSlash(path[2:])
	}
	return filepath.FromSlash(path)
}

func isASCIILetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func loadTrainingStats(path string, trainingPolicy map[string]any) map[string]trainingEntry {
	stats := map[string]trainingEntry{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return stats
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var metric map[string]any
		if err := json.Unmarshal([]byte(line), &metric); err != nil {
			continue
		}
		laneID := fallbackString(stringValue(metric["lane_id"]), stringValue(metric["provider"]))
		if laneID == "" {
			continue
		}
		entry := stats[laneID]
		entry.Samples++
		entry.ConfidenceTotal += floatDefault(metric["confidence"], 0)
		if boolAny(metric["used_live_model"]) && stringValue(metric["status"]) == "ok" {
			entry.LiveSuccesses++
		}
		if boolAny(metric["budget_halt"]) {
			entry.BudgetHalts++
		}
		stats[laneID] = entry
	}
	for laneID, entry := range stats {
		samples := maxInt(1, entry.Samples)
		entry.AvgConfidence = entry.ConfidenceTotal / float64(samples)
		entry.LiveSuccessRate = float64(entry.LiveSuccesses) / float64(samples)
		entry.BudgetHaltRate = float64(entry.BudgetHalts) / float64(samples)
		minSamples := intFromAny(fallbackAny(trainingPolicy["min_samples_before_adjustment"], 20))
		if entry.Samples >= minSamples {
			entry.Active = true
			target := floatDefault(trainingPolicy["target_confidence"], 0.72)
			weight := floatDefault(trainingPolicy["score_adjustment_weight"], 6.0)
			capValue := floatDefault(trainingPolicy["max_score_adjustment"], 4.0)
			adjustment := (entry.AvgConfidence-target)*weight - (entry.BudgetHaltRate * weight)
			entry.Adjustment = math.Max(-capValue, math.Min(capValue, adjustment))
		}
		stats[laneID] = entry
	}
	return stats
}

func laneScore(lane, task, policy, selectionPolicy map[string]any, env map[string]string, trainingStats map[string]trainingEntry, requested string) (float64, []string, trainingEntry) {
	score := 0.0
	reason := []string{}
	provider := stringValue(lane["provider"])
	resolverHint := fallbackString(stringValue(lane["resolver_hint"]), provider)
	capabilities := stringSet(listValue(lane["capabilities"]))
	taskClass := classifyTask(task, selectionPolicy)
	workerPreferred := stringValue(task["worker_preferred_resolver_hint"])
	workerPolicyMode := workerPolicyMode(task)
	text := strings.ToLower(strings.Join([]string{
		stringValue(task["goal"]),
		stringValue(task["expected_output"]),
		strings.Join(stringList(task["constraints"]), " "),
		stringValue(task["worker_type"]),
	}, " "))
	if requested != "" && requested != "auto" {
		if requested == provider || requested == resolverHint || requested == stringValue(lane["id"]) {
			score += 100
			reason = append(reason, "explicit resolver_hint matched "+requested)
		} else {
			score -= 100
		}
	} else if workerPolicyMode == "prefer" && workerPreferred != "" && (workerPreferred == provider || workerPreferred == resolverHint || workerPreferred == stringValue(lane["id"])) {
		score += floatDefault(selectionPolicy["worker_preference_bonus"], 10.0)
		reason = append(reason, "seeded worker preference matched "+workerPreferred)
	}
	attemptPref := strings.ToLower(stringValue(mapValue(policy["attempt"])["preference"]))
	validatePref := strings.ToLower(stringValue(mapValue(policy["validate"])["preference"]))
	if (attemptPref == "local" || attemptPref == "local-first" || attemptPref == "cheap" || attemptPref == "cheap-first") && boolAny(lane["local"]) {
		score += floatDefault(selectionPolicy["local_first_bonus"], 15.0)
		reason = append(reason, "local/cheap attempt preference")
	}
	if boolAny(lane["cloud"]) {
		score -= floatDefault(selectionPolicy["cloud_cost_penalty"], 2.0)
	}
	if (validatePref == "strong" || validatePref == "strongest" || validatePref == "strongest_available") && capabilities["strong_validation"] {
		score += floatDefault(selectionPolicy["strong_validation_bonus"], 8.0)
		reason = append(reason, "strong validation capability")
	}
	if wordsInText(text, stringListOrDefault(selectionPolicy["code_keywords"], []string{"patch", "code", "implementation", "edit"})) && capabilities["code"] {
		score += floatDefault(selectionPolicy["code_task_bonus"], 4.0)
		reason = append(reason, "code task capability")
	}
	if wordsInText(text, stringListOrDefault(selectionPolicy["safety_keywords"], []string{"safety", "approval", "risk", "security", "review"})) && (capabilities["safety"] || capabilities["review"]) {
		score += floatDefault(selectionPolicy["safety_review_bonus"], 4.0)
		reason = append(reason, "review/safety capability")
	}
	if taskClass.Name == "extraction" && boolAny(lane["local"]) {
		score += floatDefault(selectionPolicy["extraction_local_bonus"], 8.0)
		reason = append(reason, "local extractor bonus")
	}
	if taskClass.Authority == "high" && boolAny(lane["cloud"]) {
		score += floatDefault(selectionPolicy["authority_cloud_bonus"], 8.0)
		reason = append(reason, "high-authority task favors strong cloud lane")
	}
	if boolAny(lane["local"]) {
		switch taskClass.Name {
		case "architecture":
			score -= floatDefault(selectionPolicy["local_architecture_penalty"], 18.0)
			reason = append(reason, "local lane penalized for architecture task")
		case "validation":
			score -= floatDefault(selectionPolicy["local_validation_penalty"], 20.0)
			reason = append(reason, "local lane penalized for validation task")
		case "routing":
			score -= floatDefault(selectionPolicy["local_routing_penalty"], 18.0)
			reason = append(reason, "local lane penalized for routing task")
		case "edit":
			score -= floatDefault(selectionPolicy["local_edit_penalty"], 16.0)
			reason = append(reason, "local lane penalized for edit task")
		}
		host := detectLocalHostProfile(env)
		if taskClass.Authority == "high" && host.Tier == "low" {
			score -= floatDefault(selectionPolicy["low_tier_local_authority_penalty"], 10.0)
			reason = append(reason, "host local-model tier is low for high-authority task")
		}
		requiredMemGB := estimateModelMemoryGB(stringValue(lane["model"]))
		if requiredMemGB > 0 && host.MemoryGB > 0 {
			if requiredMemGB > float64(host.MemoryGB) {
				oversizeRatio := requiredMemGB / float64(host.MemoryGB)
				score -= floatDefault(selectionPolicy["oversized_local_model_penalty"], 14.0) * oversizeRatio
				reason = append(reason, fmt.Sprintf("local model likely exceeds host memory budget (%0.1fGB>%dGB)", requiredMemGB, host.MemoryGB))
			} else if requiredMemGB > float64(host.MemoryGB)*0.7 {
				score -= floatDefault(selectionPolicy["tight_fit_local_model_penalty"], 6.0)
				reason = append(reason, fmt.Sprintf("local model is a tight fit for host memory (%0.1fGB/%dGB)", requiredMemGB, host.MemoryGB))
			} else {
				score += floatDefault(selectionPolicy["local_model_fit_bonus"], 3.0)
				reason = append(reason, "local model fits host memory profile")
			}
		}
		if taskClass.Name == "extraction" && host.Acceleration == "gpu" {
			score += floatDefault(selectionPolicy["gpu_local_extraction_bonus"], 2.0)
			reason = append(reason, "GPU-backed local extraction bonus")
		}
	}
	if !boolAny(lane["available"]) {
		score -= floatDefault(selectionPolicy["unavailable_penalty"], 25.0)
		reason = append(reason, "lane availability check failed")
	}
	score += floatDefault(mapValue(lane["training"])["exploration_weight"], 0)
	training := trainingStats[stringValue(lane["id"])]
	if training.Active && training.Adjustment != 0 {
		score += training.Adjustment
		reason = append(reason, fmt.Sprintf("historical training adjustment %+0.2f", training.Adjustment))
	}
	return score, reason, training
}

func selectLane(task, policy map[string]any, requestedOverride, forceProvider, forceProviderScope string, env map[string]string, modelLanes []map[string]any, selectionPolicy, trainingPolicy map[string]any, trainingStats map[string]trainingEntry, cloudLanesEnabled bool, compareProviders []string) map[string]any {
	requested := expandEnv(fallbackString(stringValue(task["resolver_hint"]), "auto"), env)
	workerPreferred := expandEnv(stringValue(task["worker_preferred_resolver_hint"]), env)
	workerMode := workerPolicyMode(task)
	if requestedOverride != "" {
		requested = requestedOverride
	} else if forceProvider != "" && (forceProviderScope == "all" || requested == "auto") {
		requested = forceProvider
	} else if requested == "auto" && workerMode == "require" && workerPreferred != "" {
		requested = workerPreferred
	} else if requested == "auto" && containsString(stringList(task["depends_on"]), "planner_brain") {
		brainProvider := expandEnv(env["DORKPIPE_ORCH_BRAIN_PROVIDER"], env)
		fanoutProvider := expandEnv(env["DORKPIPE_ORCH_FANOUT_PROVIDER"], env)
		if brainProvider != "" {
			if fanoutProvider != "" {
				requested = fanoutProvider
			} else {
				requested = "ollama"
			}
		}
	}
	candidates := []laneCandidate{}
	for _, lane := range modelLanes {
		score, reason, training := laneScore(lane, task, policy, selectionPolicy, env, trainingStats, requested)
		candidates = append(candidates, laneCandidate{Lane: lane, Score: score, Reason: reason, Training: training})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	selected := laneCandidate{
		Lane: map[string]any{
			"id":            fallbackString(requested, "ollama.local.default"),
			"provider":      fallbackString(requested, "ollama"),
			"resolver_hint": fallbackString(requested, "ollama"),
			"model":         stringValue(mapValue(task["model"])["model"]),
			"cloud":         requested == "codex" || requested == "claude",
			"available":     false,
		},
		Score:  0,
		Reason: []string{"fallback lane because catalog is unavailable"},
		Training: trainingEntry{
			Active: false,
		},
	}
	if len(candidates) > 0 {
		selected = candidates[0]
	}
	gatedByBaseline := false
	baselineGateReason := ""
	explicitHint := requested != "" && requested != "auto"
	if boolAny(selected.Lane["cloud"]) && !cloudLanesEnabled {
		bypass := explicitHint && boolDefault(selectionPolicy["explicit_hint_bypasses_cloud_gate"], true)
		if !bypass {
			if local := firstLane(candidates, func(item laneCandidate) bool { return boolAny(item.Lane["local"]) && boolAny(item.Lane["available"]) }); local != nil {
				selected = *local
				gatedByBaseline = true
				baselineGateReason = "cloud lane gated because DORKPIPE_ORCH_CLOUD_LANES=false"
			}
		}
	}
	if boolAny(selected.Lane["cloud"]) && cloudLanesEnabled {
		thresholdKey := "cloud_score_threshold"
		if highRiskTask(task, selectionPolicy) {
			thresholdKey = "high_risk_cloud_score_threshold"
		}
		threshold := floatDefault(selectionPolicy[thresholdKey], 14.0)
		if selected.Score < threshold {
			localAcceptThreshold := floatDefault(selectionPolicy["local_accept_score_threshold"], 0.0)
			if local := firstLane(candidates, func(item laneCandidate) bool {
				return boolAny(item.Lane["local"]) && boolAny(item.Lane["available"]) && item.Score >= localAcceptThreshold
			}); local != nil {
				selected = *local
				gatedByBaseline = true
				baselineGateReason = fmt.Sprintf("cloud lane gated by baseline threshold %.1f", threshold)
			}
		}
	}
	lane := copyMap(selected.Lane)
	taskModel := mapValue(task["model"])
	if model := stringValue(taskModel["model"]); model != "" {
		lane["model"] = expandEnv(model, env)
	}
	return map[string]any{
		"task_id":              stringValue(task["id"]),
		"worker":               stringValue(task["worker"]),
		"worker_policy_mode":   workerMode,
		"worker_preference":    workerPreferred,
		"requested":            requested,
		"lane_id":              stringValue(lane["id"]),
		"provider":             fallbackString(stringValue(lane["provider"]), fallbackString(stringValue(lane["resolver_hint"]), requested)),
		"resolver_hint":        fallbackString(stringValue(lane["resolver_hint"]), fallbackString(stringValue(lane["provider"]), requested)),
		"model":                stringValue(lane["model"]),
		"cloud":                boolAny(lane["cloud"]),
		"local":                boolAny(lane["local"]),
		"available":            boolAny(lane["available"]),
		"missing_commands":     listValue(lane["missing_commands"]),
		"capabilities":         listValue(lane["capabilities"]),
		"context_window":       intFromAny(lane["context_window"]),
		"max_task_tokens":      intFromAny(fallbackAny(mapValue(lane["budget"])["max_task_tokens"], env["DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS"])),
		"score":                selected.Score,
		"reasons":              selected.Reason,
		"gated_by_baseline":    gatedByBaseline,
		"baseline_gate_reason": baselineGateReason,
		"training":             selected.Training,
	}
}

func applyTaskWorkerProfile(task map[string]any, env map[string]string) (map[string]any, error) {
	profile := strings.ToLower(strings.TrimSpace(expandEnv(stringValue(task["worker"]), env)))
	out := copyMap(task)
	if profile == "" {
		return out, nil
	}
	defaults, ok := seededWorkerProfiles[profile]
	if !ok {
		keys := make([]string, 0, len(seededWorkerProfiles))
		for key := range seededWorkerProfiles {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("unknown worker profile %q (expected one of: %s)", profile, strings.Join(keys, ", "))
	}
	out["worker"] = profile
	policy := copyMap(mapValue(out["worker_policy"]))
	if mode := strings.ToLower(strings.TrimSpace(expandEnv(stringValue(policy["mode"]), env))); mode != "" {
		if mode != "prefer" && mode != "require" {
			return nil, fmt.Errorf("worker profile %q uses unsupported worker_policy.mode %q (expected prefer or require)", profile, mode)
		}
		policy["mode"] = mode
	} else {
		if strings.EqualFold(strings.TrimSpace(stringValue(out["work_mode"])), "edit") {
			policy["mode"] = "require"
		} else {
			policy["mode"] = "prefer"
		}
	}
	out["worker_policy"] = policy
	out["worker_preferred_resolver_hint"] = stringValue(defaults["preferred_resolver_hint"])
	model := copyMap(mapValue(out["model"]))
	defaultModel := mapValue(defaults["model"])
	for key, value := range defaultModel {
		if _, exists := model[key]; !exists || stringValue(model[key]) == "" {
			model[key] = value
		}
	}
	if len(model) > 0 {
		out["model"] = model
	}
	return out, nil
}

func workerPolicyMode(task map[string]any) string {
	mode := strings.ToLower(strings.TrimSpace(stringValue(mapValue(task["worker_policy"])["mode"])))
	if mode == "require" {
		return "require"
	}
	if strings.TrimSpace(stringValue(task["worker"])) != "" {
		return "prefer"
	}
	return ""
}

func wordsInText(text string, words []string) bool {
	for _, word := range words {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(word)) + `\b`).MatchString(text) {
			return true
		}
	}
	return false
}

func stringSet(values []any) map[string]bool {
	out := map[string]bool{}
	for _, item := range values {
		out[fmt.Sprint(item)] = true
	}
	return out
}

func stringListOrDefault(v any, fallback []string) []string {
	values := stringList(v)
	if len(values) == 0 {
		return fallback
	}
	return values
}

func classifyTask(task, selectionPolicy map[string]any) taskClass {
	text := strings.ToLower(strings.Join([]string{
		stringValue(task["id"]),
		stringValue(task["goal"]),
		stringValue(task["expected_output"]),
		strings.Join(stringList(task["constraints"]), " "),
		stringValue(task["worker_type"]),
		stringValue(task["work_mode"]),
	}, " "))
	switch {
	case wordsInText(text, stringListOrDefault(selectionPolicy["validation_keywords"], []string{"validate", "validation", "verifier", "review"})):
		return taskClass{Name: "validation", Authority: "high"}
	case wordsInText(text, stringListOrDefault(selectionPolicy["routing_keywords"], []string{"router", "routing", "yaml", "index.yaml", "machine-readable"})):
		return taskClass{Name: "routing", Authority: "high"}
	case wordsInText(text, stringListOrDefault(selectionPolicy["architecture_keywords"], []string{"architecture", "contract", "source-of-truth", "policy", "acceptance criteria", "synthesis"})):
		return taskClass{Name: "architecture", Authority: "high"}
	case stringValue(task["work_mode"]) == "edit":
		return taskClass{Name: "edit", Authority: "high"}
	case wordsInText(text, stringListOrDefault(selectionPolicy["extraction_keywords"], []string{"extract", "inventory", "fact packet", "facts only", "path groups", "boundary signals"})):
		return taskClass{Name: "extraction", Authority: "low"}
	case wordsInText(text, stringListOrDefault(selectionPolicy["code_keywords"], []string{"patch", "code", "implementation", "edit"})):
		return taskClass{Name: "code", Authority: "medium"}
	default:
		return taskClass{Name: "analysis", Authority: "medium"}
	}
}

func highRiskTask(task, selectionPolicy map[string]any) bool {
	text := strings.ToLower(strings.Join([]string{
		stringValue(task["id"]),
		stringValue(task["goal"]),
		stringValue(task["expected_output"]),
		strings.Join(stringList(task["constraints"]), " "),
		stringValue(task["worker_type"]),
	}, " "))
	return wordsInText(text, stringList(selectionPolicy["high_risk_keywords"]))
}

func comparisonEnabledForTask(task map[string]any, compareProviders []string, compareScope string) bool {
	if len(compareProviders) == 0 {
		return false
	}
	if workerPolicyMode(task) == "require" {
		return false
	}
	requested := stringValue(task["resolver_hint"])
	if compareScope == "all" {
		return true
	}
	return requested == "" || requested == "auto"
}

func comparisonTaskID(taskID, provider string) string {
	safe := strings.Trim(reSafeCompareSuffix.ReplaceAllString(provider, "_"), "_")
	if safe == "" {
		safe = "lane"
	}
	return taskID + "__" + safe
}

func renderShared(entry map[string]any, root string, env map[string]string) (string, error) {
	collector := fallbackString(stringValue(entry["collector"]), "literal")
	switch collector {
	case "literal":
		return stringValue(entry["text"]), nil
	case "repo_map":
		tracked, _ := runCommandString(root, env, "git", "-C", root, "ls-files")
		trackedCount := 0
		for _, line := range strings.Split(tracked, "\n") {
			if strings.TrimSpace(line) != "" {
				trackedCount++
			}
		}
		lines := []string{"# Repo Map", "", fmt.Sprintf("- Tracked files: %d", trackedCount)}
		if focus := stringValue(entry["focus"]); focus != "" {
			lines = append(lines, "- Focus: "+focus)
		}
		paths := stringList(entry["paths"])
		if len(paths) > 0 {
			lines = append(lines, "", "## Key Paths", "")
			for _, rel := range paths {
				if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
					lines = append(lines, "- `"+rel+"`")
				}
			}
		}
		return strings.Join(lines, "\n") + "\n", nil
	case "dockpipe_cli_inspect":
		dockpipeCmd := resolveDockpipeCommand(root, env)
		version, _ := runCommandString(root, env, dockpipeCmd, "--version")
		packages, _ := runCommandString(root, env, dockpipeCmd, "package", "list", "--workdir", root)
		return strings.Join([]string{
			"# CLI Inspect",
			"",
			"```text",
			strings.TrimSpace(version),
			"```",
			"",
			"## Package List",
			"",
			"```text",
			strings.TrimRight(packages, "\n"),
			"```",
			"",
		}, "\n"), nil
	default:
		return "", fmt.Errorf("unknown shared collector %q", collector)
	}
}

func runCommandString(root string, env map[string]string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Env = envList(env)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimRight(stdout.String(), "\n")
	if err != nil {
		if stderr.Len() > 0 {
			return strings.TrimRight(stderr.String(), "\n"), err
		}
		return out, err
	}
	return out, nil
}

func envList(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func resolveInputPath(rel, root, artifactRoot, sharedDir string) string {
	candidates := []string{}
	if filepath.IsAbs(rel) {
		candidates = append(candidates, rel)
	} else {
		candidates = append(candidates, filepath.Join(artifactRoot, rel), filepath.Join(filepath.Dir(sharedDir), rel), filepath.Join(root, rel))
	}
	absRoot, _ := filepath.Abs(root)
	absArtifact, _ := filepath.Abs(artifactRoot)
	for _, candidate := range candidates {
		resolved, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			continue
		}
		if withinRoot(absRoot, resolved) || withinRoot(absArtifact, resolved) {
			return resolved
		}
	}
	return ""
}

func renderInputContext(inputPaths []string, provider, root, artifactRoot, sharedDir string, enabled bool, maxBytes, totalMaxBytes int) string {
	if !enabled || len(inputPaths) == 0 || totalMaxBytes <= 0 {
		return ""
	}
	remaining := totalMaxBytes
	sections := []string{
		"Input context excerpts:",
		"",
		"Use these excerpts as the primary source of truth for this task. Cite paths in prose when useful, but keep the final answer compact.",
	}
	included := 0
	ordered := append([]string{}, inputPaths...)
	sort.Slice(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		leftRank := 0
		rightRank := 0
		if strings.HasPrefix(left, "shared/cli-inspect") {
			leftRank = 1
		}
		if strings.HasPrefix(right, "shared/cli-inspect") {
			rightRank = 1
		}
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftMD := strings.HasSuffix(left, ".md")
		rightMD := strings.HasSuffix(right, ".md")
		if leftMD != rightMD {
			return leftMD
		}
		return left < right
	})
	for _, rel := range ordered {
		if remaining <= 0 {
			break
		}
		path := resolveInputPath(rel, root, artifactRoot, sharedDir)
		if path == "" {
			continue
		}
		text, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		snippetBytes := truncateUTF8(text, minInt(maxBytes, remaining))
		snippet := strings.TrimRight(string(snippetBytes), "\n")
		if strings.TrimSpace(snippet) == "" {
			continue
		}
		included++
		remaining -= len([]byte(snippet))
		sections = append(sections, "", "### "+rel, "", "```text", snippet)
		if len(text) > len(snippetBytes) {
			sections = append(sections, "[truncated]")
		}
		sections = append(sections, "```")
	}
	if included == 0 {
		return ""
	}
	if provider == "ollama" {
		sections = append(sections, "", "Local model lane guidance:", "- Prefer direct claims from the excerpts over broad background knowledge.", "- If the excerpts do not prove a point, mark it as uncertain instead of filling the gap.", "- Keep the answer short and concrete so the merge step can compare it against stronger lanes.")
	}
	return strings.Join(sections, "\n") + "\n"
}

func resolveDockpipeCommand(root string, env map[string]string) string {
	if env != nil {
		if configured := strings.TrimSpace(env["DOCKPIPE_BIN"]); configured != "" {
			return configured
		}
	}
	dockpipeBin := filepath.Join(root, "src", "bin", "dockpipe")
	if runtime.GOOS == "windows" {
		dockpipeBinExe := dockpipeBin + ".exe"
		if _, err := os.Stat(dockpipeBinExe); err == nil {
			return dockpipeBinExe
		}
	}
	if _, err := os.Stat(dockpipeBin); err == nil {
		return dockpipeBin
	}
	return "dockpipe"
}

func mustResolveScopeCitations(task map[string]any, taskInputs []string, resolve func([]any) ([]string, error)) []string {
	paths, err := resolve(listValue(fallbackAny(task["citations"], anySlice(taskInputs))))
	if err != nil {
		return taskInputs
	}
	return paths
}

func anySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func taskHasComparison(tasks []any, taskID string, compareProviders []string, compareScope string) bool {
	for _, raw := range tasks {
		task := mapValue(raw)
		if stringValue(task["id"]) == taskID {
			return comparisonEnabledForTask(task, compareProviders, compareScope)
		}
	}
	return false
}

func copyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstLane(candidates []laneCandidate, keep func(laneCandidate) bool) *laneCandidate {
	for _, item := range candidates {
		if keep(item) {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func wordNumber(word string) int {
	switch strings.ToLower(word) {
	case "one":
		return 1
	case "two":
		return 2
	case "three":
		return 3
	case "four":
		return 4
	case "five":
		return 5
	default:
		return 0
	}
}

func anyLine(lines []string, predicate func(string) bool) bool {
	for _, line := range lines {
		if predicate(line) {
			return true
		}
	}
	return false
}

func sortedTaskIDs(tasks map[string]schedulerTask) []string {
	keys := make([]string, 0, len(tasks))
	for key := range tasks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedTaskIDsFromSet(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key, ok := range values {
		if ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func fighterLabel(task schedulerTask) string {
	provider := strings.ToUpper(fallbackString(task.Provider, "unknown"))
	if task.Model != "" {
		return provider + "(" + task.Model + ")"
	}
	return provider
}

func fighterBar(status string, frame int) string {
	switch status {
	case "done":
		return "[##########]"
	case "failed":
		return "[xxx   xxx]"
	case "queued":
		return "[          ]"
	default:
		patterns := []string{"[>        <]", "[=>      <=]", "[==>    <==]", "[===>  <===]", "[==>    <==]", "[=>      <=]"}
		return patterns[frame%len(patterns)]
	}
}

func formatElapsed(start, end, now time.Time) string {
	if start.IsZero() {
		return "  --.-s"
	}
	finish := end
	if finish.IsZero() {
		finish = now
	}
	return fmt.Sprintf("%6.1fs", finish.Sub(start).Seconds())
}

func formatTokens(result map[string]any) string {
	tokens := intFromAny(result["estimated_total_tokens"])
	if tokens <= 0 {
		return "    -- tok"
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%6.1fk tok", float64(tokens)/1000)
	}
	return fmt.Sprintf("%6d tok", tokens)
}

func workerLogPath(tasksDir, taskID string) string {
	if tasksDir == "" {
		return ""
	}
	return filepath.Join(tasksDir, taskID, "worker.log")
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
