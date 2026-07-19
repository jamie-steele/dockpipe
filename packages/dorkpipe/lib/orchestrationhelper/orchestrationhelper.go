package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
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
	reDurableHostPath   = regexp.MustCompile("(?i)(?:[A-Z]:[\\\\/]|\\\\\\\\|/(?:home|Users|tmp|var|mnt|opt)/)[^`<>\\r\\n]*?\\.(?:md|ya?ml|txt|json|go|js|jsx|ts|tsx|xml|toml|ini|cs|java|py|rb|rs|sh|ps1|sql)")
	reDurableHostToken  = regexp.MustCompile(`(?i)(?:[A-Z]:[\\/]|\\\\|/(?:home|Users|tmp|var|mnt|opt)/)[A-Za-z0-9._~@%+,\-\\/]+`)
	reMarkdownLink      = regexp.MustCompile(`\[[^\]]*\]\(([^)#]+)(?:#[^)]+)?\)`)
	reValidationRemoved = regexp.MustCompile("(?im)^- \\*\\*Removed `([^`]+)`")
)

var durableOutputForbiddenTerms = []struct {
	label   string
	pattern *regexp.Regexp
}{
	{label: "DockPipe or DorkPipe", pattern: regexp.MustCompile(`(?i)\b(?:dockpipe|dorkpipe)\b`)},
	{label: "orchestration", pattern: regexp.MustCompile(`(?i)\borchestrat(?:ion|or|ed|ing)\b`)},
	{label: "runtime mount terminology", pattern: regexp.MustCompile(`(?i)\b(?:runtime mounts?|mount labels?|mounted source(?: roots?)?)\b`)},
	{label: "artifact root terminology", pattern: regexp.MustCompile(`(?i)\b(?:artifact|workflow) roots?\b`)},
	{label: "lane terminology", pattern: regexp.MustCompile(`(?i)\b(?:worker|model|provider) lanes?\b|\blane selection\b`)},
	{label: "provider metadata", pattern: regexp.MustCompile(`(?i)\bproviders?_(?:actual|requested)\b|\bresolver hints?\b`)},
	{label: "run artifact terminology", pattern: regexp.MustCompile(`(?i)\b(?:source packets?|task graphs?|worker results?|merge results?|materialized outputs?|artifact handoffs?|worker artifacts?)\b`)},
}

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
	case "provider-pool-response-env":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper provider-pool-response-env <response.json> <response.md>")
		}
		return emitProviderPoolResponseEnv(args[1], args[2], stdout)
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
	case "materialize-task-outputs":
		if len(args) != 6 {
			return errors.New("usage: orchestrate-helper materialize-task-outputs <response.md> <task-dir> <outputs.json> <result.json> <repo-root>")
		}
		return materializeTaskOutputs(args[1], args[2], args[3], args[4], args[5], env["DOCKPIPE_CONTAINER_MOUNTS"])
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
	case "build-verify-result":
		if len(args) != 11 {
			return errors.New("usage: orchestrate-helper build-verify-result <out.json> <plan.json> <graph.json> <merge.json> <cloud-usage.json> <halt.json> <status> <confidence> <issues.json> <next-action>")
		}
		return buildVerifyResult(args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], env)
	case "apply-results":
		if len(args) != 6 && len(args) != 7 {
			return errors.New("usage: orchestrate-helper apply-results <root> <artifact-root> <plan.json> <approval.md> <result.json> [verify-result.json]")
		}
		verifyPath := ""
		if len(args) == 7 {
			verifyPath = args[6]
		}
		return applyResultsWithVerify(args[1], args[2], args[3], args[4], args[5], verifyPath, boolString(env["DORKPIPE_ORCH_APPLY_ON_REVIEW"]))
	case "plan":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper plan <workflow.yml> <step-id>")
		}
		return planOrchestration(args[1], args[2], env)
	case "software-dev-compile":
		if len(args) != 7 && len(args) != 8 {
			return errors.New("usage: orchestrate-helper software-dev-compile <package-workflow.yml> <package-step-id> <repo-root> <task-pack.yml> <task-pack-step-id> <artifact-root> [planner-proposal]")
		}
		proposalPath := ""
		if len(args) == 8 {
			proposalPath = args[7]
		}
		return compileSoftwareDevArtifacts(args[1], args[2], args[3], args[4], args[5], args[6], proposalPath)
	case "software-dev-stage-proposal":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper software-dev-stage-proposal <repo-root> <repo-relative-proposal> <target>")
		}
		return stageSoftwareDevProposal(args[1], args[2], args[3], env["DORKPIPE_ORCH_ROOT"])
	case "software-dev-evaluate-promotion":
		if len(args) != 5 {
			return errors.New("usage: orchestrate-helper software-dev-evaluate-promotion <repo-root> <task-pack.yml> <task-pack-step-id> <artifact-root>")
		}
		return evaluateSoftwareDevPromotionArtifacts(args[1], args[2], args[3], args[4])
	case "software-dev-build-promotion-patch":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper software-dev-build-promotion-patch <repo-root> <artifact-root>")
		}
		return buildSoftwareDevPromotionPatchArtifacts(args[1], args[2])
	case "software-dev-apply-promotion":
		if len(args) != 4 {
			return errors.New("usage: orchestrate-helper software-dev-apply-promotion <repo-root> <artifact-root> <approval.json>")
		}
		return applySoftwareDevPromotionPatch(args[1], args[2], args[3])
	case "backlog-inspect":
		if len(args) != 7 {
			return errors.New("usage: orchestrate-helper backlog-inspect <repo-root> <task-index.yml> <task-id> <bounded-slice> <baseline-commit> <artifact-root>")
		}
		return inspectBacklogSelection(args[1], args[2], args[3], args[4], args[5], args[6])
	case "backlog-compile":
		if len(args) != 9 {
			return errors.New("usage: orchestrate-helper backlog-compile <repo-root> <artifact-root> <environment-ref> <branch-ref> <allowed-paths-json> <hard-boundaries-json> <required-validation-json> <routed-sources-json>")
		}
		return compileBacklogRemoteRequest(args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8])
	case "backlog-dispatch-fixture":
		if len(args) != 3 {
			return errors.New("usage: orchestrate-helper backlog-dispatch-fixture <artifact-root> <fixture.json>")
		}
		return dispatchBacklogFixture(args[1], args[2])
	case "backlog-followup":
		if len(args) != 2 {
			return errors.New("usage: orchestrate-helper backlog-followup <artifact-root>")
		}
		followup, err := loadBacklogFollowup(args[1])
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(followup)
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

func loadTaskPack(repoRoot, taskPackPath, stepID string) (map[string]any, error) {
	displayPath := strings.TrimSpace(taskPackPath)
	if displayPath == "" {
		return nil, errors.New("task pack path is required")
	}
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("task pack path %q cannot be loaded without a consumer repo root", displayPath)
	}
	if filepath.IsAbs(displayPath) || filepath.VolumeName(displayPath) != "" {
		return nil, fmt.Errorf("task pack path %q must be relative to the consumer repo", displayPath)
	}

	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("task pack path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	candidatePath, err := filepath.Abs(filepath.Join(rootPath, filepath.Clean(filepath.FromSlash(displayPath))))
	if err != nil || !withinRoot(rootPath, candidatePath) {
		return nil, fmt.Errorf("task pack path %q escapes the consumer repo", displayPath)
	}
	info, err := os.Stat(candidatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task pack path %q does not exist", displayPath)
		}
		return nil, fmt.Errorf("task pack path %q cannot be read: %w", displayPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("task pack path %q is not a workflow file", displayPath)
	}

	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return nil, fmt.Errorf("task pack path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("task pack path %q cannot be resolved: %w", displayPath, err)
	}
	if !withinRoot(resolvedRoot, resolvedCandidate) {
		return nil, fmt.Errorf("task pack path %q escapes the consumer repo", displayPath)
	}

	selectedStepID := strings.TrimSpace(stepID)
	if selectedStepID == "" {
		return nil, fmt.Errorf("task pack step id is required for %q", displayPath)
	}
	raw, err := os.ReadFile(resolvedCandidate)
	if err != nil {
		return nil, fmt.Errorf("task pack path %q cannot be read: %w", displayPath, err)
	}
	workflow := map[string]any{}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		return nil, fmt.Errorf("task pack path %q is not valid workflow YAML: %w", displayPath, err)
	}

	matches := []map[string]any{}
	for _, rawStep := range listValue(workflow["steps"]) {
		step := mapValue(rawStep)
		if stringValue(step["id"]) == selectedStepID {
			matches = append(matches, step)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s: task pack step id %q was not found", displayPath, selectedStepID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s: task pack step id %q is ambiguous (%d matches)", displayPath, selectedStepID, len(matches))
	}

	agent, ok := mapDeclaration(matches[0]["agent"])
	if !ok {
		return nil, fmt.Errorf("%s: task pack step id %q has no agent.orchestration declaration", displayPath, selectedStepID)
	}
	if _, ok := mapDeclaration(agent["orchestration"]); !ok {
		return nil, fmt.Errorf("%s: task pack step id %q has no agent.orchestration declaration", displayPath, selectedStepID)
	}
	if err := infrastructure.ValidateResolvedWorkflowYAML(resolvedCandidate); err != nil {
		return nil, fmt.Errorf("task pack path %q is not a valid workflow: %w", displayPath, err)
	}
	return copyMap(agent), nil
}

func mapDeclaration(value any) (map[string]any, bool) {
	switch value.(type) {
	case map[string]any, map[any]any:
		return mapValue(value), true
	default:
		return nil, false
	}
}

type parsedPlannerProposal struct {
	Format      string         `json:"format"`
	Declaration map[string]any `json:"declaration"`
}

type compiledExecutableContract struct {
	Plan             map[string]any `json:"plan"`
	TaskGraph        map[string]any `json:"task_graph"`
	TaskArtifacts    []any          `json:"task_artifacts"`
	ProposalMetadata map[string]any `json:"proposal_metadata"`
}

// parsePlannerProposal accepts exactly one structured JSON or YAML document. It returns data only;
// the result is not executable until compileExecutableContract validates the complete contract.
func parsePlannerProposal(raw []byte) (*parsedPlannerProposal, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("planner proposal is empty")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(trimmed))
	document := yaml.Node{}
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	extra := yaml.Node{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("planner proposal must contain exactly one structured document")
		}
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("planner proposal root must be an object")
	}

	declaration := map[string]any{}
	if err := document.Content[0].Decode(&declaration); err != nil {
		return nil, fmt.Errorf("planner proposal is not valid JSON or YAML: %w", err)
	}
	if problems := validatePlannerProposal(declaration); len(problems) > 0 {
		return nil, fmt.Errorf("invalid planner proposal: %s", strings.Join(problems, "; "))
	}
	format := "yaml"
	if json.Valid(trimmed) {
		format = "json"
	}
	return &parsedPlannerProposal{Format: format, Declaration: declaration}, nil
}

func validatePlannerProposal(proposal map[string]any) []string {
	problems := []string{}
	appendUnknownProposalFields(proposal, "planner proposal root", []string{
		"startup_prompt", "include_agents_md", "access", "cloud_budget", "constraints", "publish", "sync", "orchestration",
	}, &problems)
	validateProposalOptionalString(proposal, "startup_prompt", "planner proposal.startup_prompt", &problems)
	validateProposalOptionalBool(proposal, "include_agents_md", "planner proposal.include_agents_md", &problems)
	validateProposalStringListField(proposal, "constraints", "planner proposal.constraints", &problems)
	validateProposalOptionalBool(proposal, "publish", "planner proposal.publish", &problems)
	validateProposalOptionalBool(proposal, "sync", "planner proposal.sync", &problems)
	validateProposalAccess(proposal["access"], "planner proposal.access", &problems)
	validateProposalBudget(proposal["cloud_budget"], "planner proposal.cloud_budget", &problems)

	orchestration, ok := mapDeclaration(proposal["orchestration"])
	if !ok {
		problems = append(problems, "planner proposal.orchestration is required")
		return problems
	}
	appendUnknownProposalFields(orchestration, "planner proposal.orchestration", []string{
		"request", "plan", "agents", "tasks", "merge", "verify", "apply", "access", "cloud_budget", "constraints", "publish", "sync",
	}, &problems)
	validateProposalRequest(orchestration["request"], "planner proposal.orchestration.request", &problems)
	validateProposalPlan(orchestration["plan"], "planner proposal.orchestration.plan", &problems)
	validateProposalRoles(orchestration["agents"], "planner proposal.orchestration.agents", &problems)
	validateProposalTasks(orchestration["tasks"], "planner proposal.orchestration.tasks", &problems)
	validateProposalScalarMap(orchestration["merge"], "planner proposal.orchestration.merge", &problems)
	validateProposalScalarMap(orchestration["verify"], "planner proposal.orchestration.verify", &problems)
	validateProposalApply(orchestration["apply"], "planner proposal.orchestration.apply", &problems)
	validateProposalAccess(orchestration["access"], "planner proposal.orchestration.access", &problems)
	validateProposalBudget(orchestration["cloud_budget"], "planner proposal.orchestration.cloud_budget", &problems)
	validateProposalStringListField(orchestration, "constraints", "planner proposal.orchestration.constraints", &problems)
	validateProposalOptionalBool(orchestration, "publish", "planner proposal.orchestration.publish", &problems)
	validateProposalOptionalBool(orchestration, "sync", "planner proposal.orchestration.sync", &problems)
	return problems
}

func appendUnknownProposalFields(value map[string]any, field string, allowed []string, problems *[]string) {
	allowedSet := map[string]bool{}
	for _, key := range allowed {
		allowedSet[key] = true
	}
	unknown := []string{}
	for key := range value {
		if !allowedSet[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		*problems = append(*problems, fmt.Sprintf("%s field %q is not allowed", field, key))
	}
}

func validateProposalOptionalString(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	if _, ok := raw.(string); !ok {
		*problems = append(*problems, field+" must be a string")
	}
}

func validateProposalRequiredString(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	text, ok := raw.(string)
	if !present || !ok || strings.TrimSpace(text) == "" {
		*problems = append(*problems, field+" is required")
	}
}

func validateProposalOptionalBool(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	if _, ok := raw.(bool); !ok {
		*problems = append(*problems, field+" must be a boolean")
	}
}

func proposalArray(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func validateProposalStringListField(value map[string]any, key, field string, problems *[]string) {
	raw, present := value[key]
	if !present {
		return
	}
	items, ok := proposalArray(raw)
	if !ok {
		*problems = append(*problems, field+" must be an array of strings")
		return
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			*problems = append(*problems, field+" must be an array of strings")
			return
		}
	}
}

func proposalInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case uint64:
		if typed <= uint64(^uint(0)>>1) {
			return int(typed), true
		}
	case float64:
		if typed == math.Trunc(typed) && typed >= 0 && typed <= float64(^uint(0)>>1) {
			return int(typed), true
		}
	}
	return 0, false
}

func validateProposalAccess(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	access, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(access, field, []string{"read", "write", "deny", "remove_deny"}, problems)
	for _, key := range []string{"read", "write", "deny", "remove_deny"} {
		validateProposalStringListField(access, key, field+"."+key, problems)
	}
}

func validateProposalBudget(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	budget, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(budget, field, []string{"max_total_tokens", "max_task_tokens", "max_tasks"}, problems)
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		raw, present := budget[key]
		if !present {
			continue
		}
		if number, ok := proposalInteger(raw); !ok || number < 0 {
			*problems = append(*problems, field+"."+key+" must be a non-negative integer")
		}
	}
}

func validateProposalRequest(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	request, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(request, field, []string{"text"}, problems)
	validateProposalOptionalString(request, "text", field+".text", problems)
}

func validateProposalPlan(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	plan, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(plan, field, []string{"goal", "steps", "constraints"}, problems)
	validateProposalOptionalString(plan, "goal", field+".goal", problems)
	validateProposalStringListField(plan, "steps", field+".steps", problems)
	validateProposalStringListField(plan, "constraints", field+".constraints", problems)
}

func validateProposalRoles(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	roles, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		definition, ok := mapDeclaration(roles[id])
		roleField := fmt.Sprintf("%s[%q]", field, id)
		if strings.TrimSpace(id) == "" {
			*problems = append(*problems, field+" role id must not be empty")
		}
		if !ok {
			*problems = append(*problems, roleField+" must be an object")
			continue
		}
		appendUnknownProposalFields(definition, roleField, []string{
			"role", "worker", "worker_type", "work_mode", "worker_policy", "authority", "constraints", "accessible_paths", "access", "model", "model_policy", "max_cloud_tokens", "cloud_budget", "require_approval", "publish", "sync",
		}, problems)
		for _, key := range []string{"role", "worker", "worker_type", "work_mode"} {
			validateProposalOptionalString(definition, key, roleField+"."+key, problems)
		}
		for _, key := range []string{"worker_policy", "authority", "model", "model_policy"} {
			if raw, present := definition[key]; present {
				if _, ok := mapDeclaration(raw); !ok {
					*problems = append(*problems, roleField+"."+key+" must be an object")
				}
			}
		}
		validateProposalStringListField(definition, "constraints", roleField+".constraints", problems)
		validateProposalStringListField(definition, "accessible_paths", roleField+".accessible_paths", problems)
		validateProposalAccess(definition["access"], roleField+".access", problems)
		validateProposalBudget(definition["cloud_budget"], roleField+".cloud_budget", problems)
		validateProposalOptionalBool(definition, "require_approval", roleField+".require_approval", problems)
		validateProposalOptionalBool(definition, "publish", roleField+".publish", problems)
		validateProposalOptionalBool(definition, "sync", roleField+".sync", problems)
		if raw, present := definition["max_cloud_tokens"]; present {
			if number, ok := proposalInteger(raw); !ok || number < 0 {
				*problems = append(*problems, roleField+".max_cloud_tokens must be a non-negative integer")
			}
		}
	}
}

func validateProposalTasks(value any, field string, problems *[]string) {
	tasks, ok := proposalArray(value)
	if !ok {
		*problems = append(*problems, field+" must be an array")
		return
	}
	if len(tasks) == 0 {
		*problems = append(*problems, field+" must contain at least one task")
		return
	}
	for index, raw := range tasks {
		taskField := fmt.Sprintf("%s[%d]", field, index)
		task, ok := mapDeclaration(raw)
		if !ok {
			*problems = append(*problems, taskField+" must be an object")
			continue
		}
		appendUnknownProposalFields(task, taskField, []string{
			"id", "agent", "goal", "brief", "context", "expected_output", "prompt", "constraints", "depends_on", "claims", "citations", "max_cloud_tokens", "materialize_outputs",
		}, problems)
		validateProposalRequiredString(task, "id", taskField+".id", problems)
		for _, key := range []string{"agent", "goal", "brief", "expected_output", "prompt"} {
			validateProposalOptionalString(task, key, taskField+"."+key, problems)
		}
		for _, key := range []string{"constraints", "depends_on", "claims", "citations"} {
			validateProposalStringListField(task, key, taskField+"."+key, problems)
		}
		validateProposalTaskContext(task["context"], taskField+".context", problems)
		validateProposalMaterializeOutputs(task["materialize_outputs"], taskField+".materialize_outputs", problems)
		if raw, present := task["max_cloud_tokens"]; present {
			if number, ok := proposalInteger(raw); !ok || number < 0 {
				*problems = append(*problems, taskField+".max_cloud_tokens must be a non-negative integer")
			}
		}
	}
}

func validateProposalTaskContext(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	context, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(context, field, []string{"required_artifacts", "seed_paths", "source_roots"}, problems)
	for _, key := range []string{"required_artifacts", "seed_paths", "source_roots"} {
		validateProposalStringListField(context, key, field+"."+key, problems)
	}
}

func validateProposalMaterializeOutputs(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	outputs, ok := proposalArray(value)
	if !ok {
		*problems = append(*problems, field+" must be an array")
		return
	}
	for index, raw := range outputs {
		outputField := fmt.Sprintf("%s[%d]", field, index)
		output, ok := mapDeclaration(raw)
		if !ok {
			*problems = append(*problems, outputField+" must be an object")
			continue
		}
		appendUnknownProposalFields(output, outputField, []string{"id", "path"}, problems)
		validateProposalOptionalString(output, "id", outputField+".id", problems)
		validateProposalRequiredString(output, "path", outputField+".path", problems)
	}
}

func validateProposalScalarMap(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	data, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !isContractScalar(data[key]) {
			*problems = append(*problems, field+"."+key+" must be a scalar")
		}
	}
}

func validateProposalApply(value any, field string, problems *[]string) {
	if value == nil {
		return
	}
	apply, ok := mapDeclaration(value)
	if !ok {
		*problems = append(*problems, field+" must be an object")
		return
	}
	appendUnknownProposalFields(apply, field, []string{"require_approval", "target_root", "required_artifacts"}, problems)
	validateProposalOptionalBool(apply, "require_approval", field+".require_approval", problems)
	validateProposalOptionalString(apply, "target_root", field+".target_root", problems)
	validateProposalStringListField(apply, "required_artifacts", field+".required_artifacts", problems)
}

// compileExecutableContract is the authority boundary between parsed planner data and runnable
// artifacts. It constructs no partial graph or task artifact until normalization fully succeeds.
func compileExecutableContract(packageDefaults, repoTaskPack map[string]any, proposal *parsedPlannerProposal) (*compiledExecutableContract, error) {
	var proposalDeclaration map[string]any
	if proposal != nil {
		proposalDeclaration = proposal.Declaration
	}
	normalized, err := normalizeTaskPackContract(packageDefaults, repoTaskPack, proposalDeclaration)
	if err != nil {
		return nil, err
	}

	rolesByID := map[string]map[string]any{}
	for _, rawRole := range listValue(normalized["roles"]) {
		role := mapValue(rawRole)
		rolesByID[stringValue(role["id"])] = role
	}
	taskArtifacts := make([]any, 0, len(listValue(normalized["tasks"])))
	graphTasks := make([]any, 0, len(listValue(normalized["tasks"])))
	for _, rawTask := range listValue(normalized["tasks"]) {
		task := mapValue(rawTask)
		artifact := map[string]any{}
		for _, key := range []string{"id", "agent", "goal", "brief", "context", "constraints", "expected_output", "prompt", "claims", "citations", "max_cloud_tokens", "depends_on", "materialize_outputs"} {
			if value, present := task[key]; present {
				artifact[key] = value
			}
		}
		role := rolesByID[stringValue(task["agent"])]
		for _, key := range []string{"role", "worker", "worker_type", "work_mode", "worker_policy", "model", "model_policy", "accessible_paths", "authority"} {
			if value, present := role[key]; present {
				artifact[key] = value
			}
		}
		roleConstraints := stringList(role["constraints"])
		artifact["constraints"] = anySlice(appendStableContractStrings(roleConstraints, stringList(task["constraints"])...))
		effectiveAccess := mapValue(normalized["access"])
		if len(role) > 0 {
			effectiveAccess = mapValue(role["access"])
		}
		if taskAccess, present := mapDeclaration(task["access"]); present {
			effectiveAccess = intersectContractAccess(effectiveAccess, taskAccess)
		}
		artifact["access"] = effectiveAccess
		if _, present := artifact["max_cloud_tokens"]; !present {
			if roleBudget := intFromAny(role["max_cloud_tokens"]); roleBudget > 0 {
				artifact["max_cloud_tokens"] = roleBudget
			} else if runBudget := intFromAny(mapValue(normalized["cloud_budget"])["max_task_tokens"]); runBudget > 0 {
				artifact["max_cloud_tokens"] = runBudget
			}
		}
		if stringValue(artifact["role"]) == "" && stringValue(artifact["agent"]) != "" {
			artifact["role"] = artifact["agent"]
		}
		if len(mapValue(artifact["context"])) == 0 {
			artifact["context"] = map[string]any{}
		}
		worker := fallbackString(strings.TrimSpace(stringValue(artifact["worker"])), "auto")
		model := mapValue(artifact["model"])
		lane := map[string]any{
			"lane_id":       "compiled." + worker,
			"provider":      worker,
			"resolver_hint": worker,
			"model":         stringValue(model["model"]),
			"available":     true,
		}
		artifact["base_id"] = artifact["id"]
		artifact["comparison"] = map[string]any{"enabled": false}
		artifact["resolver_hint"] = worker
		artifact["requested_resolver_hint"] = worker
		artifact["lane"] = lane
		artifact["output_path"] = inferTaskOutputPath(artifact)
		artifact["context_paths"] = anySlice(stringList(mapValue(artifact["context"])["required_artifacts"]))
		artifact["startup_prompt"] = normalized["startup_prompt"]
		artifact["include_agents_md"] = normalized["include_agents_md"]
		taskArtifacts = append(taskArtifacts, artifact)
		graphTasks = append(graphTasks, map[string]any{
			"id":                  artifact["id"],
			"base_task_id":        artifact["id"],
			"agent":               artifact["agent"],
			"depends_on":          artifact["depends_on"],
			"resolver_hint":       artifact["resolver_hint"],
			"lane_id":             lane["lane_id"],
			"provider":            lane["provider"],
			"model":               lane["model"],
			"worker_type":         fallbackString(stringValue(artifact["worker_type"]), "analysis"),
			"output_path":         artifact["output_path"],
			"materialize_outputs": artifact["materialize_outputs"],
		})
	}

	planCfg := mapValue(normalized["plan"])
	concurrency := mapValue(contractOrchestration(packageDefaults)["concurrency"])
	plan := map[string]any{
		"goal":              stringValue(planCfg["goal"]),
		"request":           mapValue(normalized["request"]),
		"plan":              planCfg,
		"merge":             mapValue(normalized["merge"]),
		"verify":            mapValue(normalized["verify"]),
		"startup_prompt":    normalized["startup_prompt"],
		"include_agents_md": normalized["include_agents_md"],
		"constraints":       normalized["constraints"],
		"required_outputs":  normalized["required_outputs"],
		"access":            normalized["access"],
		"cloud_budget":      normalized["cloud_budget"],
		"roles":             normalized["roles"],
		"concurrency":       concurrency,
		"approval_required": normalized["approval_required"],
		"publish":           normalized["publish"],
		"sync":              normalized["sync"],
		"apply": map[string]any{
			"require_approval":   normalized["approval_required"],
			"target_root":        normalized["apply_target"],
			"required_artifacts": normalized["required_outputs"],
		},
	}
	return &compiledExecutableContract{
		Plan:             plan,
		TaskGraph:        map[string]any{"concurrency": concurrency, "tasks": graphTasks},
		TaskArtifacts:    taskArtifacts,
		ProposalMetadata: normalizedProposalMetadata(proposal, normalized),
	}, nil
}

func normalizedProposalMetadata(proposal *parsedPlannerProposal, normalized map[string]any) map[string]any {
	metadata := map[string]any{
		"present":             false,
		"source_format":       "none",
		"selected_graph":      false,
		"role_ids":            []any{},
		"constraints":         []any{},
		"task_ids":            []any{},
		"dependencies":        []any{},
		"materialize_outputs": []any{},
	}
	if proposal == nil {
		return metadata
	}
	metadata["present"] = true
	metadata["source_format"] = proposal.Format
	metadata["selected_graph"] = true
	roleIDs := []string{}
	for id := range mapValue(contractOrchestration(proposal.Declaration)["agents"]) {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	metadata["role_ids"] = anySlice(roleIDs)
	metadata["constraints"] = anySlice(contractConstraints(proposal.Declaration))
	taskIDs := []string{}
	dependencies := []any{}
	outputs := []any{}
	for _, rawTask := range listValue(normalized["tasks"]) {
		task := mapValue(rawTask)
		id := stringValue(task["id"])
		taskIDs = append(taskIDs, id)
		dependencies = append(dependencies, map[string]any{"task_id": id, "depends_on": task["depends_on"]})
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			outputs = append(outputs, map[string]any{"task_id": id, "path": stringValue(mapValue(rawOutput)["path"])})
		}
	}
	metadata["task_ids"] = anySlice(taskIDs)
	metadata["dependencies"] = dependencies
	metadata["materialize_outputs"] = outputs
	return metadata
}

func compileSoftwareDevArtifacts(packageWorkflowPath, packageStepID, repoRoot, taskPackPath, taskPackStepID, artifactRoot, proposalPath string) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	artifactRoot = rootPath
	cleanupOnError := func(err error, rawProposal []byte) error {
		if len(rawProposal) > 0 {
			if recordErr := recordRejectedSoftwareDevProposal(artifactRoot, rawProposal); recordErr != nil {
				err = fmt.Errorf("%v; record rejected proposal: %w", err, recordErr)
			}
		}
		if cleanupErr := cleanupSoftwareDevExecutableArtifacts(artifactRoot); cleanupErr != nil {
			return fmt.Errorf("%v; clean executable artifacts: %w", err, cleanupErr)
		}
		return err
	}

	packageWorkflowAbs, err := filepath.Abs(packageWorkflowPath)
	if err != nil {
		return cleanupOnError(fmt.Errorf("invalid package workflow path: %w", err), nil)
	}
	packageDefaults, err := loadTaskPack(filepath.Dir(packageWorkflowAbs), filepath.Base(packageWorkflowAbs), packageStepID)
	if err != nil {
		return cleanupOnError(fmt.Errorf("load package software.dev contract: %w", err), nil)
	}
	packageDefaults["cloud_budget"] = map[string]any{
		"max_total_tokens": 60000,
		"max_task_tokens":  20000,
		"max_tasks":        8,
	}
	packageDefaults["constraints"] = []any{
		"execute only tasks in the successfully compiled graph",
		"preserve package access, budget, approval, apply, publish, and sync authority",
	}
	packageDefaults["publish"] = false
	packageDefaults["sync"] = false
	repoTaskPack, err := loadTaskPack(repoRoot, taskPackPath, taskPackStepID)
	if err != nil {
		return cleanupOnError(err, nil)
	}

	var proposal *parsedPlannerProposal
	var rawProposal []byte
	if strings.TrimSpace(proposalPath) != "" {
		rawProposal, err = os.ReadFile(proposalPath)
		if err != nil {
			return cleanupOnError(fmt.Errorf("planner proposal cannot be read: %w", err), nil)
		}
		proposal, err = parsePlannerProposal(rawProposal)
		if err != nil {
			return cleanupOnError(err, rawProposal)
		}
	}

	compiled, err := compileExecutableContract(packageDefaults, repoTaskPack, proposal)
	if err != nil {
		return cleanupOnError(err, rawProposal)
	}
	canonicalTaskPackPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(taskPackPath))))
	compiled.ProposalMetadata["task_pack"] = map[string]any{
		"path":    canonicalTaskPackPath,
		"step_id": strings.TrimSpace(taskPackStepID),
	}
	if err := materializeSoftwareDevContract(artifactRoot, repoRoot, compiled, proposal, rawProposal); err != nil {
		return cleanupOnError(err, rawProposal)
	}
	return nil
}

func stageSoftwareDevProposal(repoRoot, repoRelativeProposal, target, artifactRoot string) error {
	raw, err := readRepoRelativeRegularFile(repoRoot, repoRelativeProposal)
	if err != nil {
		return err
	}
	if strings.TrimSpace(artifactRoot) == "" {
		return errors.New("software.dev artifact root is required when staging a proposal")
	}
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	targetPath, err := filepath.Abs(target)
	if err != nil || !withinRoot(rootPath, targetPath) {
		return fmt.Errorf("software.dev staged proposal target escapes the run artifact root: %s", target)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return fmt.Errorf("software.dev artifact root cannot be resolved: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(targetPath))
	if err != nil || !withinRoot(resolvedRoot, resolvedParent) {
		return fmt.Errorf("software.dev staged proposal target escapes the resolved run artifact root: %s", target)
	}
	if info, statErr := os.Lstat(targetPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("software.dev staged proposal target cannot be a symlink: %s", target)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	return os.WriteFile(targetPath, raw, 0o644)
}

func readRepoRelativeRegularFile(repoRoot, relativePath string) ([]byte, error) {
	displayPath := strings.TrimSpace(relativePath)
	if displayPath == "" {
		return nil, errors.New("proposal fixture path is required")
	}
	if filepath.IsAbs(displayPath) || filepath.VolumeName(displayPath) != "" {
		return nil, fmt.Errorf("proposal fixture path %q must be relative to the consumer repo", displayPath)
	}
	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	candidatePath, err := filepath.Abs(filepath.Join(rootPath, filepath.Clean(filepath.FromSlash(displayPath))))
	if err != nil || !withinRoot(rootPath, candidatePath) {
		return nil, fmt.Errorf("proposal fixture path %q escapes the consumer repo", displayPath)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q has an invalid consumer repo root: %w", displayPath, err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be resolved: %w", displayPath, err)
	}
	if !withinRoot(resolvedRoot, resolvedCandidate) {
		return nil, fmt.Errorf("proposal fixture path %q escapes the consumer repo", displayPath)
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be read: %w", displayPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("proposal fixture path %q is not a regular file", displayPath)
	}
	raw, err := os.ReadFile(resolvedCandidate)
	if err != nil {
		return nil, fmt.Errorf("proposal fixture path %q cannot be read: %w", displayPath, err)
	}
	return raw, nil
}

func materializeSoftwareDevContract(artifactRoot, repoRoot string, compiled *compiledExecutableContract, proposal *parsedPlannerProposal, rawProposal []byte) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		return err
	}
	stageRoot, err := os.MkdirTemp(rootPath, ".software-dev-compile-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageRoot)

	if err := writeJSONFile(filepath.Join(stageRoot, "request.json"), mapValue(compiled.Plan["request"])); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "plan.json"), compiled.Plan); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "task-graph.json"), compiled.TaskGraph); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageRoot, "proposal", "metadata.json"), compiled.ProposalMetadata); err != nil {
		return err
	}
	if proposal != nil {
		if err := writeJSONFile(filepath.Join(stageRoot, "proposal", "normalized.json"), proposal.Declaration); err != nil {
			return err
		}
		extension := ".yaml"
		if proposal.Format == "json" {
			extension = ".json"
		}
		if err := os.WriteFile(filepath.Join(stageRoot, "proposal", "raw"+extension), rawProposal, 0o644); err != nil {
			return err
		}
	}
	for _, rawTask := range compiled.TaskArtifacts {
		task := mapValue(rawTask)
		taskID := stringValue(task["id"])
		if taskID == "" {
			return errors.New("compiled software.dev task artifact has no id")
		}
		taskDir := filepath.Join(stageRoot, "tasks", taskID)
		if err := writeJSONFile(filepath.Join(taskDir, "task.json"), task); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(taskDir, "prompt.md"), []byte(renderCompiledSoftwareDevPrompt(compiled.Plan, task, repoRoot)), 0o644); err != nil {
			return err
		}
	}

	for _, name := range []string{"request.json", "plan.json", "task-graph.json", "tasks", "proposal"} {
		target := filepath.Join(rootPath, name)
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stageRoot, name), target); err != nil {
			return err
		}
	}
	return nil
}

func renderCompiledSoftwareDevPrompt(plan, task map[string]any, repoRoot string) string {
	lines := []string{}
	if startup := strings.TrimSpace(stringValue(plan["startup_prompt"])); startup != "" {
		lines = append(lines, startup, "")
	}
	lines = append(lines,
		"You are one bounded worker in a governed software-development task graph.",
		"",
		"Task id: "+stringValue(task["id"]),
		"Agent role: "+fallbackString(stringValue(task["role"]), stringValue(task["agent"])),
		"Goal: "+stringValue(task["goal"]),
		"Expected output: "+stringValue(task["expected_output"]),
	)
	if brief := strings.TrimSpace(stringValue(task["brief"])); brief != "" {
		lines = append(lines, "", "Brief:", brief)
	}
	if prompt := strings.TrimSpace(stringValue(task["prompt"])); prompt != "" {
		lines = append(lines, "", "Task instructions:", prompt)
	}
	if constraints := stringList(task["constraints"]); len(constraints) > 0 {
		lines = append(lines, "", "Constraints:")
		for _, constraint := range constraints {
			lines = append(lines, "- "+constraint)
		}
	}
	if contextPaths := stringList(task["context_paths"]); len(contextPaths) > 0 {
		lines = append(lines, "", "Context briefing paths:")
		for _, contextPath := range contextPaths {
			lines = append(lines, "- "+contextPath)
		}
	}
	access := mapValue(task["access"])
	if len(stringList(access["read"]))+len(stringList(access["write"]))+len(stringList(access["deny"])) > 0 {
		lines = append(lines, "", "Access policy:")
		for _, mode := range []string{"read", "write", "deny"} {
			paths := stringList(access[mode])
			if len(paths) == 0 {
				continue
			}
			lines = append(lines, mode+":")
			for _, path := range paths {
				lines = append(lines, "- "+path)
			}
		}
	}
	if boolAny(plan["include_agents_md"]) {
		if rawAgents, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			lines = append(lines, "", "AGENTS.md context:", "", strings.TrimRight(string(rawAgents), "\n"))
		}
	}
	lines = append(lines,
		"",
		"Rules:",
		"- Execute only this compiled task and respect its dependencies.",
		"- Treat access, deny, budget, approval, apply, publish, and sync policy as fixed authority.",
		"- Return the requested artifact content directly without narrating commands or orchestration mechanics.",
	)
	if contract := renderMaterializeOutputContract(listValue(task["materialize_outputs"])); contract != "" {
		lines = append(lines, "", contract)
	}
	return strings.Join(lines, "\n") + "\n"
}

func cleanupSoftwareDevExecutableArtifacts(artifactRoot string) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	for _, name := range []string{"plan.json", "task-graph.json", "tasks"} {
		if err := os.RemoveAll(filepath.Join(rootPath, name)); err != nil {
			return err
		}
	}
	return nil
}

func recordRejectedSoftwareDevProposal(artifactRoot string, raw []byte) error {
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := os.RemoveAll(proposalDir); err != nil {
		return err
	}
	if err := os.MkdirAll(proposalDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(proposalDir, "rejected.txt"), raw, 0o644)
}

func softwareDevArtifactRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("software.dev artifact root is required")
	}
	rootPath, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid software.dev artifact root: %w", err)
	}
	if filepath.Dir(rootPath) == rootPath {
		return "", fmt.Errorf("software.dev artifact root cannot be a filesystem root: %s", rootPath)
	}
	return rootPath, nil
}

type softwareDevPromotionIdentity struct {
	TaskPackPath string
	StepID       string
	SiblingRoles string
}

func evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, taskPackStepID, artifactRoot string) error {
	repoTaskPack, identity, err := loadSoftwareDevPromotionIdentity(repoRoot, taskPackPath, taskPackStepID)
	if err != nil {
		return err
	}
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := requirePromotionDirectory(rootPath, proposalDir, "proposal artifact directory"); err != nil {
		return err
	}
	if err := requirePromotionDirectory(rootPath, filepath.Join(rootPath, "verify"), "verification artifact directory"); err != nil {
		return err
	}
	candidatePath := filepath.Join(proposalDir, "promotion-candidate.json")

	rawProposal, rawRelativePath, err := readPromotionRawProposal(rootPath)
	if err != nil {
		return err
	}
	parsed, err := parsePlannerProposal(rawProposal)
	if err != nil {
		return fmt.Errorf("promotion source proposal is invalid: %w", err)
	}
	normalized, err := readPromotionJSONMap(rootPath, "proposal/normalized.json")
	if err != nil {
		return err
	}
	metadata, err := readPromotionJSONMap(rootPath, "proposal/metadata.json")
	if err != nil {
		return err
	}
	verification, err := readPromotionJSONMap(rootPath, "verify/result.json")
	if err != nil {
		return err
	}
	if mustJSON(parsed.Declaration, nil) != mustJSON(normalized, nil) {
		return errors.New("promotion proposal raw and normalized artifacts are inconsistent")
	}
	if err := validatePromotionMetadata(parsed, metadata, identity); err != nil {
		return err
	}
	if err := validatePromotionVerificationArtifact(verification); err != nil {
		return err
	}

	candidate := evaluateSoftwareDevPromotion(repoTaskPack, normalized, metadata, verification, identity, rawRelativePath)
	if err := addPromotionCandidateTargetDigests(candidate, repoRoot, identity); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(candidatePath, candidate); err != nil {
		return fmt.Errorf("write promotion candidate: %w", err)
	}
	return nil
}

func loadSoftwareDevPromotionIdentity(repoRoot, taskPackPath, taskPackStepID string) (map[string]any, softwareDevPromotionIdentity, error) {
	displayPath := strings.TrimSpace(taskPackPath)
	canonicalPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(displayPath)))
	if displayPath == "" {
		return nil, softwareDevPromotionIdentity{}, errors.New("promotion task pack path is required")
	}
	if filepath.IsAbs(displayPath) || filepath.VolumeName(displayPath) != "" {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q must be relative to the consumer repo", displayPath)
	}
	if canonicalPath == "." || displayPath != canonicalPath {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q is not an exact canonical repo-relative identity", displayPath)
	}
	stepID := strings.TrimSpace(taskPackStepID)
	if stepID == "" || stepID != taskPackStepID {
		return nil, softwareDevPromotionIdentity{}, errors.New("promotion task pack step id must be an exact non-empty identity")
	}
	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion consumer repo root is invalid: %w", err)
	}
	candidatePath, err := filepath.Abs(filepath.Join(rootPath, filepath.FromSlash(canonicalPath)))
	if err != nil || !withinRoot(rootPath, candidatePath) {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q escapes the consumer repo", displayPath)
	}
	if err := rejectSymlinkPath(rootPath, candidatePath, "promotion task pack"); err != nil {
		return nil, softwareDevPromotionIdentity{}, err
	}
	info, err := os.Stat(candidatePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q is not a readable regular file", displayPath)
	}
	repoTaskPack, err := loadTaskPack(rootPath, canonicalPath, stepID)
	if err != nil {
		return nil, softwareDevPromotionIdentity{}, err
	}

	siblingRelative := ""
	siblingPath := filepath.Join(filepath.Dir(candidatePath), "agents.yml")
	if siblingInfo, statErr := os.Lstat(siblingPath); statErr == nil {
		if siblingInfo.Mode()&os.ModeSymlink != 0 || !siblingInfo.Mode().IsRegular() {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q is not a repo-owned regular file", displayPath)
		}
		if err := rejectSymlinkPath(rootPath, siblingPath, "promotion sibling agents file"); err != nil {
			return nil, softwareDevPromotionIdentity{}, err
		}
		sibling := readYAMLMap(siblingPath)
		if _, ok := mapDeclaration(sibling["agents"]); !ok {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q has no agents mapping", displayPath)
		}
		relative, relErr := filepath.Rel(rootPath, siblingPath)
		if relErr != nil || strings.HasPrefix(relative, "..") {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q escapes the consumer repo", displayPath)
		}
		siblingRelative = filepath.ToSlash(relative)
	} else if !os.IsNotExist(statErr) {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("inspect promotion sibling agents path: %w", statErr)
	}

	return repoTaskPack, softwareDevPromotionIdentity{
		TaskPackPath: canonicalPath,
		StepID:       stepID,
		SiblingRoles: siblingRelative,
	}, nil
}

func rejectSymlinkPath(rootPath, targetPath, label string) error {
	relative, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s escapes the consumer repo", label)
	}
	current := rootPath
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return fmt.Errorf("%s cannot be inspected: %w", label, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s cannot use a symlinked identity: %s", label, filepath.ToSlash(relative))
		}
	}
	return nil
}

func requirePromotionDirectory(rootPath, directory, label string) error {
	if !withinRoot(rootPath, directory) {
		return fmt.Errorf("%s escapes the run artifact root", label)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("%s is missing: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a regular directory", label)
	}
	return nil
}

func readPromotionRawProposal(rootPath string) ([]byte, string, error) {
	type candidate struct {
		path     string
		relative string
	}
	found := []candidate{}
	for _, relative := range []string{"proposal/raw.yaml", "proposal/raw.json"} {
		path := filepath.Join(rootPath, filepath.FromSlash(relative))
		if _, err := os.Lstat(path); err == nil {
			found = append(found, candidate{path: path, relative: relative})
		} else if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("inspect %s: %w", relative, err)
		}
	}
	if len(found) != 1 {
		return nil, "", fmt.Errorf("promotion requires exactly one raw proposal artifact; found %d", len(found))
	}
	raw, err := readPromotionRegularFile(rootPath, found[0].path, found[0].relative)
	return raw, found[0].relative, err
}

func readPromotionJSONMap(rootPath, relative string) (map[string]any, error) {
	path := filepath.Join(rootPath, filepath.FromSlash(relative))
	raw, err := readPromotionRegularFile(rootPath, path, relative)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("promotion artifact %s is malformed JSON: %w", relative, err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("promotion artifact %s must be a non-empty JSON object", relative)
	}
	return payload, nil
}

func readPromotionRegularFile(rootPath, path, relative string) ([]byte, error) {
	if !withinRoot(rootPath, path) {
		return nil, fmt.Errorf("promotion artifact %s escapes the run artifact root", relative)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("promotion artifact %s is missing: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("promotion artifact %s is not a regular file", relative)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promotion artifact %s cannot be read: %w", relative, err)
	}
	return raw, nil
}

func validatePromotionMetadata(proposal *parsedPlannerProposal, metadata map[string]any, identity softwareDevPromotionIdentity) error {
	if !boolAny(metadata["present"]) || !boolAny(metadata["selected_graph"]) {
		return errors.New("promotion metadata does not identify a successfully selected proposal graph")
	}
	if stringValue(metadata["source_format"]) != proposal.Format {
		return errors.New("promotion metadata source format does not match the raw proposal")
	}
	taskPack := mapValue(metadata["task_pack"])
	if stringValue(taskPack["path"]) != identity.TaskPackPath || stringValue(taskPack["step_id"]) != identity.StepID {
		return errors.New("promotion metadata task-pack identity does not match the selected repo surface")
	}

	orchestration := contractOrchestration(proposal.Declaration)
	roleIDs := []string{}
	for id := range mapValue(orchestration["agents"]) {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	taskIDs := []string{}
	dependencies := []any{}
	outputs := []any{}
	for _, rawTask := range listValue(orchestration["tasks"]) {
		task := mapValue(rawTask)
		id := stringValue(task["id"])
		taskIDs = append(taskIDs, id)
		dependencies = append(dependencies, map[string]any{"task_id": id, "depends_on": anySlice(stringList(task["depends_on"]))})
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			outputs = append(outputs, map[string]any{"task_id": id, "path": stringValue(mapValue(rawOutput)["path"])})
		}
	}
	expected := map[string]any{
		"role_ids":            anySlice(roleIDs),
		"constraints":         anySlice(contractConstraints(proposal.Declaration)),
		"task_ids":            anySlice(taskIDs),
		"dependencies":        dependencies,
		"materialize_outputs": outputs,
	}
	for _, key := range []string{"role_ids", "constraints", "task_ids", "dependencies", "materialize_outputs"} {
		if mustJSON(metadata[key], []any{}) != mustJSON(expected[key], []any{}) {
			return fmt.Errorf("promotion metadata %s does not match the selected proposal graph", key)
		}
	}
	return nil
}

func validatePromotionVerificationArtifact(verification map[string]any) error {
	for _, key := range []string{"status", "failure_class"} {
		value, ok := verification[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("promotion verification %s must be a non-empty string", key)
		}
	}
	issues, ok := proposalArray(verification["issues"])
	if !ok {
		return errors.New("promotion verification issues must be an array of strings")
	}
	for _, issue := range issues {
		if _, ok := issue.(string); !ok {
			return errors.New("promotion verification issues must be an array of strings")
		}
	}
	valueBar, ok := mapDeclaration(verification["value_bar"])
	if !ok {
		return errors.New("promotion verification value_bar must be an object")
	}
	if verdict, ok := valueBar["verdict"].(string); !ok || strings.TrimSpace(verdict) == "" {
		return errors.New("promotion verification value_bar.verdict must be a non-empty string")
	}
	switch valueBar["overall_score"].(type) {
	case int, int64, float32, float64, json.Number:
	default:
		return errors.New("promotion verification value_bar.overall_score must be a number")
	}
	baseline, ok := mapDeclaration(verification["direct_worker_baseline"])
	if !ok {
		return errors.New("promotion verification direct_worker_baseline must be an object")
	}
	if verdict, ok := baseline["verdict"].(string); !ok || strings.TrimSpace(verdict) == "" {
		return errors.New("promotion verification direct_worker_baseline.verdict must be a non-empty string")
	}
	return nil
}

func evaluateSoftwareDevPromotion(repoTaskPack, proposal, metadata, verification map[string]any, identity softwareDevPromotionIdentity, rawRelativePath string) map[string]any {
	delta, meaningful := promotionSoftLayerDelta(repoTaskPack, proposal)
	reasons := []any{}
	status := strings.TrimSpace(stringValue(verification["status"]))
	if status != "pass" {
		reasons = append(reasons, promotionReason("verification_not_passed", "Verification status must be pass."))
	}
	if len(stringList(verification["issues"])) > 0 {
		reasons = append(reasons, promotionReason("verification_has_issues", "Verification issues must be empty."))
	}
	if failureClass := strings.TrimSpace(stringValue(verification["failure_class"])); failureClass != "none" {
		reasons = append(reasons, promotionReason("verification_failure_class", "Verification failure_class must be none."))
	}
	valueBar := mapValue(verification["value_bar"])
	valueVerdict := stringValue(valueBar["verdict"])
	if valueVerdict != "strong_orchestration_value" && valueVerdict != "mixed_orchestration_value" {
		reasons = append(reasons, promotionReason("weak_or_missing_value_bar", "Value-bar evidence must be mixed or strong."))
	}
	baseline := mapValue(verification["direct_worker_baseline"])
	baselineVerdict := stringValue(baseline["verdict"])
	if baselineVerdict != "orchestration_adds_value" && baselineVerdict != "direct_worker_likely_sufficient" {
		reasons = append(reasons, promotionReason("lower_or_missing_direct_worker_baseline", "Direct-worker evidence must not prefer the direct worker."))
	}
	if !meaningful {
		reasons = append(reasons, promotionReason("no_reusable_soft_layer_delta", "The proposal adds no meaningful reusable soft-layer guidance."))
	}
	eligibilityStatus := "eligible"
	if len(reasons) > 0 {
		eligibilityStatus = "ineligible"
	} else {
		reasons = append(reasons,
			promotionReason("selected_proposal_graph_confirmed", "Proposal metadata matches the selected compiled graph."),
			promotionReason("verification_and_value_bar_passed", "Verification, value-bar, and direct-worker gates passed."),
			promotionReason("reusable_soft_layer_delta_found", "At least one reusable soft-layer delta is reviewable."),
		)
	}

	targets := []any{
		map[string]any{
			"kind":    "selected_task_pack_step",
			"path":    identity.TaskPackPath,
			"step_id": identity.StepID,
		},
	}
	if len(mapValue(delta["roles"])) > 0 && identity.SiblingRoles != "" {
		targets = append(targets, map[string]any{
			"kind": "owned_sibling_agents",
			"path": identity.SiblingRoles,
		})
	}

	return map[string]any{
		"contract_version": "software.dev.promotion-candidate/v1",
		"eligibility": map[string]any{
			"status":  eligibilityStatus,
			"reasons": reasons,
		},
		"source_evidence": map[string]any{
			"proposal": map[string]any{
				"raw":            rawRelativePath,
				"normalized":     "proposal/normalized.json",
				"metadata":       "proposal/metadata.json",
				"source_format":  metadata["source_format"],
				"selected_graph": metadata["selected_graph"],
			},
			"verification": map[string]any{
				"path":                    "verify/result.json",
				"status":                  status,
				"failure_class":           verification["failure_class"],
				"value_bar_verdict":       valueVerdict,
				"value_bar_overall_score": valueBar["overall_score"],
				"direct_worker_verdict":   baselineVerdict,
			},
		},
		"mutable_surface": map[string]any{
			"task_pack_path": identity.TaskPackPath,
			"step_id":        identity.StepID,
		},
		"repo_owned_target_surfaces":     targets,
		"promotable_soft_layer_delta":    delta,
		"excluded_session_only_fields":   promotionSessionExclusions(),
		"excluded_hard_authority_fields": promotionHardAuthorityExclusions(),
		"workspace_mutation": map[string]any{
			"performed":    false,
			"confirmation": "Evaluation wrote only proposal/promotion-candidate.json under the existing run artifact root.",
		},
	}
}

func promotionSoftLayerDelta(repoTaskPack, proposal map[string]any) (map[string]any, bool) {
	repoOrchestration := contractOrchestration(repoTaskPack)
	proposalOrchestration := contractOrchestration(proposal)
	meaningful := false
	step := map[string]any{}
	if startup := strings.TrimSpace(stringValue(proposal["startup_prompt"])); startup != "" && startup != strings.TrimSpace(stringValue(repoTaskPack["startup_prompt"])) {
		step["startup_prompt"] = startup
		meaningful = true
	}
	rootConstraints := promotionStringAdditions(stringList(repoTaskPack["constraints"]), stringList(proposal["constraints"]))
	if len(rootConstraints) > 0 {
		step["constraints"] = anySlice(rootConstraints)
		meaningful = true
	}

	orchestration := map[string]any{}
	orchestrationConstraints := promotionStringAdditions(stringList(repoOrchestration["constraints"]), stringList(proposalOrchestration["constraints"]))
	if len(orchestrationConstraints) > 0 {
		orchestration["constraints"] = anySlice(orchestrationConstraints)
		meaningful = true
	}
	plan := promotionPlanDelta(mapValue(repoOrchestration["plan"]), mapValue(proposalOrchestration["plan"]))
	if len(plan) > 0 {
		orchestration["plan"] = plan
		meaningful = true
	}
	for _, key := range []string{"merge", "verify"} {
		difference := promotionScalarMapDelta(mapValue(repoOrchestration[key]), mapValue(proposalOrchestration[key]))
		if len(difference) > 0 {
			orchestration[key] = difference
			meaningful = true
		}
	}
	required := promotionStringAdditions(
		stringList(mapValue(repoOrchestration["apply"])["required_artifacts"]),
		stringList(mapValue(proposalOrchestration["apply"])["required_artifacts"]),
	)
	if len(required) > 0 {
		orchestration["apply"] = map[string]any{"required_artifacts": anySlice(required)}
		meaningful = true
	}
	if len(orchestration) > 0 {
		step["orchestration"] = orchestration
	}

	roles := map[string]any{}
	repoRoles := mapValue(repoOrchestration["agents"])
	proposalRoles := mapValue(proposalOrchestration["agents"])
	roleIDs := make([]string, 0, len(proposalRoles))
	for id := range proposalRoles {
		roleIDs = append(roleIDs, id)
	}
	sort.Strings(roleIDs)
	for _, id := range roleIDs {
		proposalRole := mapValue(proposalRoles[id])
		repoRole := mapValue(repoRoles[id])
		roleDelta := map[string]any{}
		if guidance := strings.TrimSpace(stringValue(proposalRole["role"])); guidance != "" && guidance != strings.TrimSpace(stringValue(repoRole["role"])) {
			roleDelta["role"] = guidance
		}
		constraints := promotionStringAdditions(stringList(repoRole["constraints"]), stringList(proposalRole["constraints"]))
		if len(constraints) > 0 {
			roleDelta["constraints"] = anySlice(constraints)
		}
		if len(roleDelta) > 0 {
			roles[id] = roleDelta
			meaningful = true
		}
	}

	return map[string]any{
		"task_pack_step": step,
		"roles":          roles,
	}, meaningful
}

func promotionPlanDelta(repo, proposal map[string]any) map[string]any {
	out := map[string]any{}
	if goal := strings.TrimSpace(stringValue(proposal["goal"])); goal != "" && goal != strings.TrimSpace(stringValue(repo["goal"])) {
		out["goal"] = goal
	}
	steps := promotionStringAdditions(stringList(repo["steps"]), stringList(proposal["steps"]))
	if len(steps) > 0 {
		out["steps"] = anySlice(steps)
	}
	constraints := promotionStringAdditions(stringList(repo["constraints"]), stringList(proposal["constraints"]))
	if len(constraints) > 0 {
		out["constraints"] = anySlice(constraints)
	}
	return out
}

func promotionScalarMapDelta(repo, proposal map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range proposal {
		if isContractScalar(value) && mustJSON(value, nil) != mustJSON(repo[key], nil) {
			out[key] = value
		}
	}
	return out
}

func promotionStringAdditions(existing, proposed []string) []string {
	out := []string{}
	for _, value := range proposed {
		value = strings.TrimSpace(value)
		if value != "" && !containsString(existing, value) {
			out = appendStableContractStrings(out, value)
		}
	}
	return out
}

func promotionReason(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

func promotionSessionExclusions() []any {
	return []any{
		map[string]any{"fields": []any{"orchestration.request"}, "reason": "The exact request is per-run context."},
		map[string]any{"fields": []any{"orchestration.tasks", "orchestration.tasks[].goal", "orchestration.tasks[].brief", "orchestration.tasks[].prompt"}, "reason": "Exact task decomposition and task prompts are session-only."},
		map[string]any{"fields": []any{"orchestration.tasks[].depends_on"}, "reason": "The exact dependency graph is session-only."},
		map[string]any{"fields": []any{"orchestration.agents[].worker", "orchestration.agents[].worker_type", "orchestration.agents[].work_mode", "orchestration.agents[].worker_policy", "orchestration.agents[].model", "orchestration.agents[].model_policy"}, "reason": "Selected workers, providers, models, and lanes are session-only."},
		map[string]any{"fields": []any{"orchestration.tasks[].materialize_outputs"}, "reason": "One-off inferred outputs remain session-only; only explicit required-artifact floor additions may be proposed."},
		map[string]any{"fields": []any{"verification.recommended_rerun_tasks", "verification.root_cause_task", "verification.next_action", "verification.issues"}, "reason": "Repair plans and validation failures are session-only evidence."},
	}
}

func promotionHardAuthorityExclusions() []any {
	return []any{
		map[string]any{"fields": []any{"access", "orchestration.access", "orchestration.agents[].access", "orchestration.agents[].accessible_paths"}, "reason": "Mounts and access policy are hard runtime authority."},
		map[string]any{"fields": []any{"access.deny", "orchestration.access.deny", "orchestration.agents[].access.deny", "remove_deny"}, "reason": "Deny-rule changes are never promotable."},
		map[string]any{"fields": []any{"auth", "secrets"}, "reason": "Authentication and secrets are never promotable."},
		map[string]any{"fields": []any{"cloud_budget", "max_cloud_tokens", "orchestration.cloud_budget", "orchestration.agents[].cloud_budget", "orchestration.agents[].max_cloud_tokens"}, "reason": "Budgets and token ceilings are package-owned authority."},
		map[string]any{"fields": []any{"orchestration.apply.require_approval", "orchestration.agents[].require_approval"}, "reason": "Approval settings are package-owned authority."},
		map[string]any{"fields": []any{"orchestration.apply.target_root"}, "reason": "The repo-selected apply target cannot be changed by promotion."},
		map[string]any{"fields": []any{"publish", "sync", "orchestration.publish", "orchestration.sync", "orchestration.agents[].publish", "orchestration.agents[].sync"}, "reason": "Publish and sync behavior are package-owned authority."},
		map[string]any{"fields": []any{"destructive_action_policy"}, "reason": "Destructive-action policy is never promotable."},
	}
}

func writeJSONFileAtomic(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("atomic JSON target cannot be a symlink: %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".promotion-candidate-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(append(raw, '\n')); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

// normalizeTaskPackContract compiles package defaults, one loaded repository task-pack agent
// declaration, and an optional per-run planner proposal into a deterministic, data-only contract.
// It deliberately does not materialize artifacts or execute tasks; the two-phase compiler owns
// those later boundaries.
func normalizeTaskPackContract(packageDefaults, repoTaskPack, proposal map[string]any) (map[string]any, error) {
	layers := []struct {
		name string
		data map[string]any
	}{
		{name: "package", data: packageDefaults},
		{name: "repo", data: repoTaskPack},
		{name: "proposal", data: proposal},
	}
	problems := []string{}

	packageAccess := normalizedContractAccess(contractAccess(packageDefaults), "package.access", &problems)
	effectiveAccess := copyMap(packageAccess)
	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		effectiveAccess = narrowContractAccess(effectiveAccess, contractAccess(layer.data), layer.name, &problems)
	}

	packageBudget := normalizedContractBudget(contractBudget(packageDefaults), "package.cloud_budget", &problems)
	effectiveBudget := copyMap(packageBudget)
	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		effectiveBudget = narrowContractBudget(effectiveBudget, contractBudget(layer.data), layer.name, &problems)
	}

	for _, layer := range layers[1:] {
		if len(layer.data) == 0 {
			continue
		}
		if contractBool(layer.data, "publish") {
			problems = append(problems, layer.name+".publish cannot enable package-owned publish")
		}
		if contractBool(layer.data, "sync") {
			problems = append(problems, layer.name+".sync cannot enable package-owned sync")
		}
		if value, ok := contractApply(layer.data)["require_approval"]; ok && !boolAny(value) {
			problems = append(problems, layer.name+".apply.require_approval cannot disable mandatory package approval")
		}
	}

	soft := map[string]any{}
	for _, key := range []string{"startup_prompt", "include_agents_md"} {
		for _, layer := range layers {
			if value, ok := layer.data[key]; ok && isContractScalar(value) {
				soft[key] = value
			}
		}
	}
	for _, key := range []string{"request", "plan", "merge", "verify"} {
		merged := map[string]any{}
		for _, layer := range layers {
			mergeContractScalars(merged, mapValue(contractOrchestration(layer.data)[key]))
		}
		soft[key] = merged
	}

	constraints := []string{}
	for _, layer := range layers {
		constraints = appendStableContractStrings(constraints, contractConstraints(layer.data)...)
	}

	baseFloor := []string{}
	for _, layer := range layers[:2] {
		for _, rawPath := range stringList(contractApply(layer.data)["required_artifacts"]) {
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s.apply.required_artifacts output path %q is invalid: %v", layer.name, rawPath, err))
				continue
			}
			baseFloor = appendStableContractStrings(baseFloor, path)
		}
	}
	requiredOutputs := append([]string{}, baseFloor...)
	proposalApply := contractApply(proposal)
	if rawProposalFloor, present := proposalApply["required_artifacts"]; present {
		proposalFloor := []string{}
		for _, rawPath := range stringList(rawProposalFloor) {
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				problems = append(problems, fmt.Sprintf("proposal.apply.required_artifacts output path %q is invalid: %v", rawPath, err))
				continue
			}
			proposalFloor = appendStableContractStrings(proposalFloor, path)
		}
		for _, floor := range baseFloor {
			if !containsString(proposalFloor, floor) {
				problems = append(problems, fmt.Sprintf("proposal.apply.required_artifacts cannot remove or rename required output %q", floor))
			}
		}
		requiredOutputs = appendStableContractStrings(requiredOutputs, proposalFloor...)
	}

	applyTarget := strings.TrimSpace(stringValue(contractApply(packageDefaults)["target_root"]))
	if repoTarget := strings.TrimSpace(stringValue(contractApply(repoTaskPack)["target_root"])); repoTarget != "" {
		applyTarget = repoTarget
	}
	if proposalTarget := strings.TrimSpace(stringValue(proposalApply["target_root"])); proposalTarget != "" && proposalTarget != applyTarget {
		problems = append(problems, fmt.Sprintf("proposal.apply.target_root %q cannot change repo-selected target %q", proposalTarget, applyTarget))
	}

	roles := normalizeContractRoles(packageDefaults, repoTaskPack, proposal, packageAccess, effectiveAccess, effectiveBudget, &problems)
	taskLayerName, rawTasks := selectContractTasks(packageDefaults, repoTaskPack, proposal)
	tasks := normalizeContractTasks(rawTasks, taskLayerName, effectiveAccess, effectiveBudget, &problems)
	if maxTasks := intFromAny(effectiveBudget["max_tasks"]); maxTasks > 0 && len(tasks) > maxTasks {
		problems = append(problems, fmt.Sprintf("%s.tasks count %d exceeds cloud_budget.max_tasks ceiling %d", taskLayerName, len(tasks), maxTasks))
	}
	validateContractTaskRoles(tasks, roles, taskLayerName, &problems)
	validateContractGraph(tasks, requiredOutputs, &problems)

	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid normalized orchestration contract: %s", strings.Join(problems, "; "))
	}

	return map[string]any{
		"startup_prompt":    soft["startup_prompt"],
		"include_agents_md": soft["include_agents_md"],
		"request":           soft["request"],
		"plan":              soft["plan"],
		"merge":             soft["merge"],
		"verify":            soft["verify"],
		"access":            effectiveAccess,
		"cloud_budget":      effectiveBudget,
		"constraints":       anySlice(constraints),
		"required_outputs":  anySlice(requiredOutputs),
		"apply_target":      applyTarget,
		"approval_required": true,
		"publish":           false,
		"sync":              false,
		"roles":             roles,
		"tasks":             tasks,
	}, nil
}

func contractOrchestration(layer map[string]any) map[string]any {
	return mapValue(layer["orchestration"])
}

func contractAccess(layer map[string]any) map[string]any {
	if access, ok := mapDeclaration(layer["access"]); ok {
		return access
	}
	return mapValue(contractOrchestration(layer)["access"])
}

func contractBudget(layer map[string]any) map[string]any {
	if budget, ok := mapDeclaration(layer["cloud_budget"]); ok {
		return budget
	}
	return mapValue(contractOrchestration(layer)["cloud_budget"])
}

func contractApply(layer map[string]any) map[string]any {
	return mapValue(contractOrchestration(layer)["apply"])
}

func contractBool(layer map[string]any, key string) bool {
	if value, ok := layer[key]; ok {
		return boolAny(value)
	}
	return boolAny(contractOrchestration(layer)[key])
}

func contractConstraints(layer map[string]any) []string {
	out := []string{}
	out = appendStableContractStrings(out, stringList(layer["constraints"])...)
	orchestration := contractOrchestration(layer)
	out = appendStableContractStrings(out, stringList(orchestration["constraints"])...)
	out = appendStableContractStrings(out, stringList(mapValue(orchestration["plan"])["constraints"])...)
	return out
}

func isContractScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, int64, float64, json.Number:
		return true
	default:
		return false
	}
}

func mergeContractScalars(target, source map[string]any) {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "constraints" || !isContractScalar(source[key]) {
			continue
		}
		target[key] = source[key]
	}
}

func appendStableContractStrings(values []string, additions ...string) []string {
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition != "" && !containsString(values, addition) {
			values = append(values, addition)
		}
	}
	return values
}

func normalizedContractAccess(access map[string]any, field string, problems *[]string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"read", "write", "deny"} {
		paths := []string{}
		for _, rawPath := range stringList(access[key]) {
			path, err := normalizeContractAuthorityPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.%s path %q is invalid: %v", field, key, rawPath, err))
				continue
			}
			paths = appendStableContractStrings(paths, path)
		}
		out[key] = anySlice(paths)
	}
	return out
}

func narrowContractAccess(current, requested map[string]any, layer string, problems *[]string) map[string]any {
	out := copyMap(current)
	if removed := stringList(requested["remove_deny"]); len(removed) > 0 {
		for _, path := range removed {
			*problems = append(*problems, fmt.Sprintf("%s.access.remove_deny cannot remove package deny rule %q", layer, path))
		}
	}
	for _, key := range []string{"read", "write"} {
		raw, present := requested[key]
		if !present {
			continue
		}
		ceiling := stringList(current[key])
		narrowed := []string{}
		for _, rawPath := range stringList(raw) {
			path, err := normalizeContractAuthorityPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.access.%s path %q is invalid: %v", layer, key, rawPath, err))
				continue
			}
			if !contractPathWithinAny(path, ceiling) {
				*problems = append(*problems, fmt.Sprintf("%s.access.%s path %q is outside the current package authority ceiling", layer, key, path))
				continue
			}
			narrowed = appendStableContractStrings(narrowed, path)
		}
		out[key] = anySlice(narrowed)
	}
	denied := stringList(current["deny"])
	for _, rawPath := range stringList(requested["deny"]) {
		path, err := normalizeContractAuthorityPath(rawPath)
		if err != nil {
			*problems = append(*problems, fmt.Sprintf("%s.access.deny path %q is invalid: %v", layer, rawPath, err))
			continue
		}
		denied = appendStableContractStrings(denied, path)
	}
	out["deny"] = anySlice(denied)
	return out
}

func normalizeContractAuthorityPath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return "", errors.New("path is empty")
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path escapes its authority root")
	}
	return cleaned, nil
}

func contractPathWithinAny(candidate string, ceilings []string) bool {
	for _, ceiling := range ceilings {
		if ceiling == "." && candidate != ".." && !strings.HasPrefix(candidate, "../") && !strings.HasPrefix(candidate, "/") {
			return true
		}
		if candidate == ceiling || ceiling == "/" || strings.HasPrefix(candidate, strings.TrimSuffix(ceiling, "/")+"/") {
			return true
		}
	}
	return false
}

func intersectContractAccess(packageRoleAccess, runAccess map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"read", "write"} {
		paths := []string{}
		for _, runPath := range stringList(runAccess[key]) {
			for _, rolePath := range stringList(packageRoleAccess[key]) {
				switch {
				case contractPathWithinAny(runPath, []string{rolePath}):
					paths = appendStableContractStrings(paths, runPath)
				case contractPathWithinAny(rolePath, []string{runPath}):
					paths = appendStableContractStrings(paths, rolePath)
				}
			}
		}
		out[key] = anySlice(paths)
	}
	denied := appendStableContractStrings(nil, stringList(packageRoleAccess["deny"])...)
	denied = appendStableContractStrings(denied, stringList(runAccess["deny"])...)
	out["deny"] = anySlice(denied)
	return out
}

func normalizedContractBudget(budget map[string]any, field string, problems *[]string) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		if raw, ok := budget[key]; ok {
			value := intFromAny(raw)
			if value < 0 {
				*problems = append(*problems, fmt.Sprintf("%s.%s must be non-negative", field, key))
				continue
			}
			out[key] = value
		}
	}
	return out
}

func narrowContractBudget(current, requested map[string]any, layer string, problems *[]string) map[string]any {
	out := copyMap(current)
	for _, key := range []string{"max_total_tokens", "max_task_tokens", "max_tasks"} {
		raw, present := requested[key]
		if !present {
			continue
		}
		value := intFromAny(raw)
		ceilingRaw, governed := current[key]
		ceiling := intFromAny(ceilingRaw)
		if value < 0 {
			*problems = append(*problems, fmt.Sprintf("%s.cloud_budget.%s must be non-negative", layer, key))
			continue
		}
		if !governed || value > ceiling {
			*problems = append(*problems, fmt.Sprintf("%s.cloud_budget.%s value %d exceeds package ceiling %d", layer, key, value, ceiling))
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeContractRoles(packageDefaults, repoTaskPack, proposal map[string]any, packageAccess, runAccess, budget map[string]any, problems *[]string) []any {
	layers := []struct {
		name  string
		roles map[string]any
	}{
		{name: "package", roles: mapValue(contractOrchestration(packageDefaults)["agents"])},
		{name: "repo", roles: mapValue(contractOrchestration(repoTaskPack)["agents"])},
		{name: "proposal", roles: mapValue(contractOrchestration(proposal)["agents"])},
	}
	ids := []string{}
	for _, layer := range layers {
		for id := range layer.roles {
			ids = appendStableContractStrings(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]any, 0, len(ids))
	hardKeys := map[string]bool{"access": true, "authority": true, "max_cloud_tokens": true, "cloud_budget": true, "require_approval": true, "publish": true, "sync": true}
	for _, id := range ids {
		role := map[string]any{"id": id}
		constraints := []string{}
		packageRole := mapValue(layers[0].roles[id])
		roleAccess := copyMap(packageAccess)
		if access, ok := mapDeclaration(packageRole["access"]); ok {
			roleAccess = narrowContractAccess(roleAccess, access, fmt.Sprintf("package.roles[%q]", id), problems)
		}
		roleAccess = intersectContractAccess(roleAccess, runAccess)
		roleBudget := intFromAny(budget["max_task_tokens"])
		if raw, ok := packageRole["max_cloud_tokens"]; ok && intFromAny(raw) < roleBudget {
			roleBudget = intFromAny(raw)
		}
		for _, layer := range layers {
			definition := mapValue(layer.roles[id])
			if len(definition) == 0 {
				continue
			}
			keys := make([]string, 0, len(definition))
			for key := range definition {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "constraints" || hardKeys[key] {
					continue
				}
				role[key] = definition[key]
			}
			constraints = appendStableContractStrings(constraints, stringList(definition["constraints"])...)
			if layer.name == "package" {
				if authority, ok := definition["authority"]; ok {
					role["authority"] = authority
				}
				continue
			}
			if access, ok := mapDeclaration(definition["access"]); ok {
				roleAccess = narrowContractAccess(roleAccess, access, fmt.Sprintf("%s.roles[%q]", layer.name, id), problems)
			}
			if raw, ok := definition["max_cloud_tokens"]; ok {
				value := intFromAny(raw)
				if value > roleBudget {
					*problems = append(*problems, fmt.Sprintf("%s.roles[%q].max_cloud_tokens value %d exceeds package ceiling %d", layer.name, id, value, roleBudget))
				} else {
					roleBudget = value
				}
			}
			if authority, ok := definition["authority"]; ok && mustJSON(authority, nil) != mustJSON(role["authority"], nil) {
				*problems = append(*problems, fmt.Sprintf("%s.roles[%q].authority cannot replace package authority", layer.name, id))
			}
			for _, key := range []string{"require_approval", "publish", "sync"} {
				if boolAny(definition[key]) || (key == "require_approval" && definition[key] != nil && !boolAny(definition[key])) {
					*problems = append(*problems, fmt.Sprintf("%s.roles[%q].%s is package-owned authority", layer.name, id, key))
				}
			}
		}
		role["constraints"] = anySlice(constraints)
		role["access"] = roleAccess
		if roleBudget > 0 {
			role["max_cloud_tokens"] = roleBudget
		}
		out = append(out, role)
	}
	return out
}

func selectContractTasks(packageDefaults, repoTaskPack, proposal map[string]any) (string, []any) {
	if tasks, present := contractOrchestration(proposal)["tasks"]; present {
		return "proposal", listValue(tasks)
	}
	if tasks, present := contractOrchestration(repoTaskPack)["tasks"]; present {
		return "repo", listValue(tasks)
	}
	return "package", listValue(contractOrchestration(packageDefaults)["tasks"])
}

func normalizeContractTasks(rawTasks []any, layer string, access, budget map[string]any, problems *[]string) []any {
	out := make([]any, 0, len(rawTasks))
	maxTaskTokens := intFromAny(budget["max_task_tokens"])
	for index, raw := range rawTasks {
		source := mapValue(raw)
		id := strings.TrimSpace(stringValue(source["id"]))
		field := fmt.Sprintf("%s.tasks[%d]", layer, index)
		if id != "" {
			field = fmt.Sprintf("%s.tasks[%q]", layer, id)
		}
		task := copyMap(source)
		task["id"] = id
		dependsOn := appendStableContractStrings(nil, stringList(source["depends_on"])...)
		task["depends_on"] = anySlice(dependsOn)
		constraints := appendStableContractStrings(nil, stringList(source["constraints"])...)
		task["constraints"] = anySlice(constraints)

		outputs := []any{}
		for _, rawOutput := range listValue(source["materialize_outputs"]) {
			output := mapValue(rawOutput)
			rawPath := stringValue(output["path"])
			if len(output) == 0 {
				rawPath = stringValue(rawOutput)
				output = map[string]any{}
			}
			path, err := normalizeContractOutputPath(rawPath)
			if err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.materialize_outputs output path %q is invalid: %v", field, rawPath, err))
				continue
			}
			output = copyMap(output)
			output["path"] = path
			outputs = append(outputs, output)
		}
		task["materialize_outputs"] = outputs

		if taskAccess, ok := mapDeclaration(source["access"]); ok {
			task["access"] = narrowContractAccess(access, taskAccess, field, problems)
		}
		if raw, present := source["max_cloud_tokens"]; present {
			value := intFromAny(raw)
			if maxTaskTokens == 0 || value > maxTaskTokens {
				*problems = append(*problems, fmt.Sprintf("%s.max_cloud_tokens value %d exceeds package ceiling %d", field, value, maxTaskTokens))
			}
		}
		if contractBool(source, "publish") {
			*problems = append(*problems, field+".publish cannot enable package-owned publish")
		}
		if contractBool(source, "sync") {
			*problems = append(*problems, field+".sync cannot enable package-owned sync")
		}
		out = append(out, task)
	}
	return out
}

func validateContractTaskRoles(tasks, roles []any, layer string, problems *[]string) {
	rolesByID := map[string]map[string]any{}
	for _, rawRole := range roles {
		role := mapValue(rawRole)
		rolesByID[stringValue(role["id"])] = role
	}
	for _, rawTask := range tasks {
		task := mapValue(rawTask)
		agentID := strings.TrimSpace(stringValue(task["agent"]))
		if agentID == "" {
			continue
		}
		role, exists := rolesByID[agentID]
		if !exists {
			*problems = append(*problems, fmt.Sprintf("%s.tasks[%q].agent references unknown normalized role %q", layer, stringValue(task["id"]), agentID))
			continue
		}
		taskBudget := intFromAny(task["max_cloud_tokens"])
		roleBudget := intFromAny(role["max_cloud_tokens"])
		if taskBudget > 0 && roleBudget > 0 && taskBudget > roleBudget {
			*problems = append(*problems, fmt.Sprintf("%s.tasks[%q].max_cloud_tokens value %d exceeds normalized role %q ceiling %d", layer, stringValue(task["id"]), taskBudget, agentID, roleBudget))
		}
	}
}

func normalizeContractOutputPath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return "", errors.New("path is empty")
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return "", errors.New("path must be relative")
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path escapes the materialized output root")
	}
	return cleaned, nil
}

func validateContractGraph(tasks []any, requiredOutputs []string, problems *[]string) {
	firstTask := map[string]int{}
	orderedIDs := []string{}
	for index, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		if id == "" {
			*problems = append(*problems, fmt.Sprintf("tasks[%d].id is required", index))
			continue
		}
		if first, duplicate := firstTask[id]; duplicate {
			*problems = append(*problems, fmt.Sprintf("tasks[%q].id is duplicate (first declared at index %d)", id, first))
			continue
		}
		firstTask[id] = index
		orderedIDs = append(orderedIDs, id)
	}
	for _, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		for _, dependency := range stringList(task["depends_on"]) {
			if _, ok := firstTask[dependency]; !ok {
				*problems = append(*problems, fmt.Sprintf("tasks[%q].depends_on dependency %q does not exist", id, dependency))
			}
		}
	}

	state := map[string]int{}
	stack := []string{}
	cycleSeen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		state[id] = 1
		stack = append(stack, id)
		task := mapValue(tasks[firstTask[id]])
		for _, dependency := range stringList(task["depends_on"]) {
			if _, exists := firstTask[dependency]; !exists {
				continue
			}
			if state[dependency] == 0 {
				visit(dependency)
				continue
			}
			if state[dependency] == 1 {
				start := 0
				for stack[start] != dependency {
					start++
				}
				cycle := append(append([]string{}, stack[start:]...), dependency)
				signature := strings.Join(cycle, " -> ")
				if !cycleSeen[signature] {
					cycleSeen[signature] = true
					*problems = append(*problems, fmt.Sprintf("tasks[%q].depends_on dependency %q creates cycle %s", id, dependency, signature))
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
	}
	for _, id := range orderedIDs {
		if state[id] == 0 {
			visit(id)
		}
	}

	producers := map[string]string{}
	for _, raw := range tasks {
		task := mapValue(raw)
		id := stringValue(task["id"])
		for _, rawOutput := range listValue(task["materialize_outputs"]) {
			path := stringValue(mapValue(rawOutput)["path"])
			if first, exists := producers[path]; exists {
				*problems = append(*problems, fmt.Sprintf("materialized output %q has duplicate producers tasks[%q] and tasks[%q]", path, first, id))
				continue
			}
			producers[path] = id
		}
	}
	for _, path := range requiredOutputs {
		if _, ok := producers[path]; !ok {
			*problems = append(*problems, fmt.Sprintf("required output floor %q has no producer", path))
		}
	}
}

func emitTaskEnv(path string, stdout io.Writer) error {
	task := readJSONMap(path)
	lane := mapValue(task["lane"])
	modelPolicy := mapValue(task["model_policy"])
	mapping := map[string]string{
		"TASK_BASE_ID":                             fallbackString(stringValue(task["base_id"]), stringValue(task["id"])),
		"TASK_WORKER_PROFILE":                      stringValue(task["worker"]),
		"TASK_WORKER_POLICY_MODE":                  workerPolicyMode(task),
		"TASK_WORK_MODE":                           stringValue(task["work_mode"]),
		"TASK_OUTPUT_PATH":                         stringValue(task["output_path"]),
		"TASK_COMPARISON_JSON":                     mustJSON(task["comparison"], map[string]any{"enabled": false}),
		"TASK_RESOLVER_HINT":                       fallbackString(stringValue(task["resolver_hint"]), "auto"),
		"TASK_REQUESTED_RESOLVER_HINT":             fallbackString(stringValue(task["requested_resolver_hint"]), fallbackString(stringValue(task["resolver_hint"]), "auto")),
		"TASK_LANE_JSON":                           mustJSON(lane, map[string]any{}),
		"TASK_LANE_ID":                             stringValue(lane["lane_id"]),
		"TASK_LANE_AVAILABLE":                      strconv.FormatBool(boolAny(lane["available"])),
		"TASK_LANE_MISSING_COMMANDS_JSON":          mustJSON(lane["missing_commands"], []any{}),
		"TASK_LANE_SETUP_HINT":                     stringValue(lane["setup_hint"]),
		"TASK_LANE_AUTH_HINT":                      stringValue(lane["auth_hint"]),
		"TASK_GOAL":                                stringValue(task["goal"]),
		"TASK_EXPECTED_OUTPUT":                     stringValue(task["expected_output"]),
		"TASK_CONTEXT_PATHS_JSON":                  mustJSON(task["context_paths"], []any{}),
		"TASK_CLAIMS_JSON":                         mustJSON(task["claims"], []any{}),
		"TASK_CITATIONS_JSON":                      mustJSON(fallbackAny(task["citations"], task["context_paths"]), []any{}),
		"TASK_MATERIALIZE_OUTPUTS_JSON":            mustJSON(task["materialize_outputs"], []any{}),
		"TASK_MAX_CLOUD_TOKENS":                    strconv.Itoa(intFromAny(task["max_cloud_tokens"])),
		"TASK_MODEL_JSON":                          mustJSON(task["model"], map[string]any{}),
		"TASK_MODEL_POLICY_JSON":                   mustJSON(modelPolicy, map[string]any{}),
		"TASK_MODEL_POLICY_EXECUTION_MODE":         strings.ToLower(strings.TrimSpace(stringValue(modelPolicy["execution_mode"]))),
		"TASK_PROVIDER_POOL_ROLE":                  fallbackString(stringValue(modelPolicy["role"]), "workflow"),
		"TASK_PROVIDER_POOL_SESSION_SCOPE":         fallbackString(stringValue(modelPolicy["session_scope"]), "node"),
		"TASK_PROVIDER_POOL_MAX_ACTIVE":            strconv.Itoa(intFromAny(modelPolicy["max_active"])),
		"TASK_PROVIDER_POOL_QUEUE_TIMEOUT_SECONDS": strconv.Itoa(intFromAny(modelPolicy["queue_timeout_seconds"])),
		"TASK_DEPENDS_ON_JSON":                     mustJSON(task["depends_on"], []any{}),
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

func emitProviderPoolResponseEnv(responsePath, responseMarkdownPath string, stdout io.Writer) error {
	payload := readJSONMap(responsePath)
	text := stringValue(payload["text"])
	if strings.TrimSpace(text) != "" {
		if err := os.WriteFile(responseMarkdownPath, []byte(text), 0o644); err != nil {
			return err
		}
	}
	metadata := mapValue(payload["metadata"])
	providerSessionID := fallbackString(stringValue(metadata["provider_session_id"]), stringValue(metadata["session_id"]))
	mapping := map[string]string{
		"PROVIDER_POOL_STATE":               stringValue(payload["state"]),
		"PROVIDER_POOL_STATUS":              stringValue(payload["status"]),
		"PROVIDER_POOL_EXIT_CODE":           strconv.Itoa(intFromAny(payload["exit_code"])),
		"PROVIDER_POOL_TEXT_BYTES":          strconv.Itoa(len([]byte(text))),
		"PROVIDER_POOL_USED_LIVE_MODEL":     strconv.FormatBool(strings.EqualFold(stringValue(payload["state"]), "ready") && strings.TrimSpace(text) != ""),
		"PROVIDER_POOL_PROVIDER_SESSION_ID": providerSessionID,
		"PROVIDER_POOL_WORKER_ID":           stringValue(metadata["worker_id"]),
		"PROVIDER_POOL_PROMPT_TURN_ID":      stringValue(metadata["prompt_turn_id"]),
		"PROVIDER_POOL_METADATA_JSON":       mustJSON(metadata, map[string]any{}),
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
	if sessionID := strings.TrimSpace(env["provider_session_id"]); sessionID != "" {
		mode := "trace_only"
		if strings.TrimSpace(env["provider_pool_metadata_json"]) != "" {
			mode = "provider_pool"
		}
		payload["worker_session"] = map[string]any{
			"provider":   env["provider"],
			"session_id": sessionID,
			"mode":       mode,
		}
	}
	if metadata := strings.TrimSpace(env["provider_pool_metadata_json"]); metadata != "" {
		payload["provider_pool"] = decodeJSONAny(metadata, map[string]any{})
	}
	return writeJSONFile(path, payload)
}

func materializeTaskOutputs(responsePath, taskDir, outputsJSON, resultPath, repoRoot, mountEnv string) error {
	outputs := listValue(decodeJSONAny(outputsJSON, []any{}))
	if len(outputs) == 0 {
		return writeJSONFile(resultPath, map[string]any{
			"status": "skipped",
			"files":  []any{},
		})
	}
	raw, err := os.ReadFile(responsePath)
	if err != nil {
		return err
	}
	blocks := parseMaterializedBlocks(string(raw))
	materializedRoot := filepath.Join(taskDir, "materialized")
	type pendingMaterializedOutput struct {
		clean   string
		content string
	}
	pending := []pendingMaterializedOutput{}
	for _, rawOutput := range outputs {
		output := mapValue(rawOutput)
		rel := strings.TrimSpace(stringValue(output["path"]))
		if rel == "" {
			return errors.New("materialize_outputs entry missing path")
		}
		clean, err := cleanMaterializedRelPath(rel)
		if err != nil {
			return err
		}
		content, ok := blocks[filepath.ToSlash(clean)]
		if !ok {
			id := strings.TrimSpace(stringValue(output["id"]))
			if id != "" {
				content, ok = blocks[id]
			}
		}
		if !ok {
			return fmt.Errorf("missing materialized output block for %s", rel)
		}
		if durableOutputNeedsPolicy(clean) {
			content, err = normalizeDurableOutput(content, repoRoot, mountEnv)
			if err != nil {
				return fmt.Errorf("%s: %w", filepath.ToSlash(clean), err)
			}
		}
		pending = append(pending, pendingMaterializedOutput{clean: clean, content: content})
	}
	files := []map[string]any{}
	for _, output := range pending {
		target := filepath.Join(materializedRoot, filepath.FromSlash(output.clean))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(strings.TrimRight(output.content, "\r\n")+"\n"), 0o644); err != nil {
			return err
		}
		files = append(files, map[string]any{
			"path":     filepath.ToSlash(output.clean),
			"artifact": filepath.ToSlash(filepath.Join("materialized", filepath.FromSlash(output.clean))),
		})
	}
	return writeJSONFile(resultPath, map[string]any{
		"status": "materialized",
		"files":  files,
	})
}

func durableOutputNeedsPolicy(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".yml", ".yaml":
		return true
	default:
		return false
	}
}

func normalizeDurableOutput(content, repoRoot, mountEnv string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil || strings.TrimSpace(repoRoot) == "" {
		return "", errors.New("durable output policy requires an explicit consumer repository root")
	}
	guestRoots := []string{"/work", "/DesignNotes"}
	seenRoots := map[string]bool{"/work": true, "/DesignNotes": true}
	for _, line := range strings.Split(mountEnv, "\n") {
		_, guest, ok := parseContainerMountSpec(line)
		if !ok {
			continue
		}
		guest = cleanGuestPath(guest)
		if guest != "" && !seenRoots[guest] {
			seenRoots[guest] = true
			guestRoots = append(guestRoots, guest)
		}
	}
	sort.Slice(guestRoots, func(i, j int) bool {
		if len(guestRoots[i]) != len(guestRoots[j]) {
			return len(guestRoots[i]) > len(guestRoots[j])
		}
		return guestRoots[i] < guestRoots[j]
	})
	parts := make([]string, 0, len(guestRoots))
	for _, guest := range guestRoots {
		parts = append(parts, regexp.QuoteMeta(guest))
	}
	guestPattern := regexp.MustCompile(`(?:` + strings.Join(parts, "|") + `)(?:/[A-Za-z0-9._~@%+,\-]+)*`)
	normalized, err := rewriteDurableGuestReferences(content, guestPattern, func(reference string) (string, error) {
		return repoNativeGuestReference(root, reference, mountEnv)
	})
	if err != nil {
		return "", err
	}
	normalized, err = rewriteDurableHostReferences(normalized, reDurableHostPath, func(reference string) (string, error) {
		return repoNativeHostReference(root, reference)
	})
	if err != nil {
		return "", err
	}
	normalized, err = rewriteDurableHostReferences(normalized, reDurableHostToken, func(reference string) (string, error) {
		return repoNativeHostReference(root, reference)
	})
	if err != nil {
		return "", err
	}
	for _, forbidden := range durableOutputForbiddenTerms {
		if match := forbidden.pattern.FindString(normalized); match != "" {
			return "", fmt.Errorf("durable output contains orchestration-only terminology %q (%s)", match, forbidden.label)
		}
	}
	return normalized, nil
}

func rewriteDurableGuestReferences(content string, pattern *regexp.Regexp, rewrite func(string) (string, error)) (string, error) {
	return rewriteDurableReferencesWhere(content, pattern, func(text string, start, end int) bool {
		if start > 0 && isDurablePathCharacter(text[start-1]) {
			return false
		}
		return end >= len(text) || !isDurablePathCharacter(text[end])
	}, rewrite)
}

func rewriteDurableHostReferences(content string, pattern *regexp.Regexp, rewrite func(string) (string, error)) (string, error) {
	return rewriteDurableReferencesWhere(content, pattern, func(text string, start, end int) bool {
		if strings.Contains(text[start:end], "://") {
			return false
		}
		tokenStart := strings.LastIndexAny(text[:start], " \t\r\n`(<[\"'")
		return !strings.Contains(text[tokenStart+1:start], "://")
	}, rewrite)
}

func rewriteDurableReferencesWhere(content string, pattern *regexp.Regexp, include func(string, int, int) bool, rewrite func(string) (string, error)) (string, error) {
	allMatches := pattern.FindAllStringIndex(content, -1)
	matches := make([][]int, 0, len(allMatches))
	for _, match := range allMatches {
		if include(content, match[0], match[1]) {
			matches = append(matches, match)
		}
	}
	if len(matches) == 0 {
		return content, nil
	}
	var out strings.Builder
	start := 0
	for _, match := range matches {
		reference := content[match[0]:match[1]]
		replacement, err := rewrite(reference)
		if err != nil {
			return "", err
		}
		out.WriteString(content[start:match[0]])
		out.WriteString(replacement)
		start = match[1]
	}
	out.WriteString(content[start:])
	return out.String(), nil
}

func isDurablePathCharacter(value byte) bool {
	return value == '/' || value == '\\' || value == '-' || value == '_' || value == '.' || value == '~' || value == '@' || value == '%' || value == '+' || value == ',' ||
		(value >= '0' && value <= '9') || (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}

func repoNativeGuestReference(repoRoot, reference, mountEnv string) (string, error) {
	type candidate struct {
		guestRoot string
		hostRoot  string
	}
	candidates := []candidate{{guestRoot: "/work", hostRoot: repoRoot}}
	for _, line := range strings.Split(mountEnv, "\n") {
		host, guest, ok := parseContainerMountSpec(line)
		if !ok {
			continue
		}
		guest = cleanGuestPath(guest)
		if guest != "" {
			candidates = append(candidates, candidate{guestRoot: guest, hostRoot: host})
		}
	}
	cleanReference := cleanGuestPath(reference)
	bestLength := -1
	resolved := map[string]bool{}
	for _, item := range candidates {
		if !guestPathContains(item.guestRoot, cleanReference) {
			continue
		}
		if len(item.guestRoot) < bestLength {
			continue
		}
		if len(item.guestRoot) > bestLength {
			bestLength = len(item.guestRoot)
			resolved = map[string]bool{}
		}
		hostRoot, err := filepath.Abs(item.hostRoot)
		if err != nil {
			return "", fmt.Errorf("runtime path reference %q has an invalid source mapping", reference)
		}
		rel := strings.TrimPrefix(cleanReference, item.guestRoot)
		rel = strings.TrimPrefix(rel, "/")
		hostTarget, err := filepath.Abs(filepath.Join(hostRoot, filepath.FromSlash(rel)))
		if err != nil || !withinRoot(hostRoot, hostTarget) {
			return "", fmt.Errorf("runtime path reference %q escapes its explicit source mapping", reference)
		}
		resolved[filepath.Clean(hostTarget)] = true
	}
	if len(resolved) != 1 {
		return "", fmt.Errorf("runtime path reference %q is ambiguous; no unique repo-native source mapping exists", reference)
	}
	var hostTarget string
	for value := range resolved {
		hostTarget = value
	}
	if !withinRoot(repoRoot, hostTarget) {
		return "", fmt.Errorf("runtime path reference %q maps outside the consumer repository", reference)
	}
	rel, err := filepath.Rel(repoRoot, hostTarget)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("runtime path reference %q does not prove a specific repo-native reference", reference)
	}
	return filepath.ToSlash(rel), nil
}

func repoNativeHostReference(repoRoot, reference string) (string, error) {
	if runtime.GOOS != "windows" && regexp.MustCompile(`^[A-Za-z]:[\\/]`).MatchString(reference) {
		return "", fmt.Errorf("machine host path %q cannot be mapped on this host", reference)
	}
	hostPath, err := filepath.Abs(reference)
	if err != nil || !withinRoot(repoRoot, hostPath) {
		return "", fmt.Errorf("machine host path %q is not a repo-native reference", reference)
	}
	rel, err := filepath.Rel(repoRoot, hostPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("machine host path %q does not prove a specific repo-native reference", reference)
	}
	return filepath.ToSlash(rel), nil
}

func renderMaterializeOutputContract(outputs []any) string {
	if len(outputs) == 0 {
		return ""
	}
	paths := []string{}
	for _, rawOutput := range outputs {
		path := strings.TrimSpace(stringValue(mapValue(rawOutput)["path"]))
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return ""
	}
	lines := []string{
		"DorkPipe materialized output contract:",
		"- Your response must contain one exact block for each required output path.",
		"- Do not wrap these blocks in markdown fences.",
		"- Do not use YAML bundle/list wrappers around the file blocks.",
		"- Required output paths: " + strings.Join(paths, ", "),
		"",
		"Use this exact block format for every required path:",
	}
	for _, path := range paths {
		lines = append(lines,
			"",
			fmt.Sprintf(`<!-- dorkpipe:file path="%s" -->`, path),
			"file content here",
			"<!-- /dorkpipe:file -->",
		)
	}
	return strings.Join(lines, "\n")
}

func parseMaterializedBlocks(text string) map[string]string {
	blocks := map[string]string{}
	re := regexp.MustCompile(`(?ms)<!--\s*dorkpipe:file\s+([^>]+?)\s*-->\s*\r?\n?(.*?)\r?\n?<!--\s*/dorkpipe:file\s*-->`)
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		key := materializedBlockKey(match[1])
		if key == "" {
			continue
		}
		blocks[key] = match[2]
	}
	return blocks
}

func materializedBlockKey(attrs string) string {
	attrs = strings.TrimSpace(attrs)
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?:^|\s)path\s*=\s*"([^"]+)"`),
		regexp.MustCompile(`(?:^|\s)path\s*=\s*'([^']+)'`),
		regexp.MustCompile(`(?:^|\s)id\s*=\s*"([^"]+)"`),
		regexp.MustCompile(`(?:^|\s)id\s*=\s*'([^']+)'`),
	} {
		if match := re.FindStringSubmatch(attrs); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	fields := strings.Fields(attrs)
	if len(fields) > 0 {
		return strings.TrimSpace(fields[0])
	}
	return ""
}

func cleanMaterializedRelPath(path string) (string, error) {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	if normalized == "" || strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", fmt.Errorf("materialized output path must be relative: %s", path)
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized)))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("materialized output path escapes task directory: %s", path)
	}
	return cleaned, nil
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
			"status":                  stringValue(raw["status"]),
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
	issues := []string{}
	_ = json.Unmarshal([]byte(issuesJSON), &issues)
	outputs, err := resolveApplyOutputs(rootPath, artifactRootPath, applyCfg)
	if err != nil {
		issues = append(issues, err.Error())
		fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote("fail"))
		fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
		return nil
	}
	if len(outputs) == 0 {
		fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote("pass"))
		fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
		return nil
	}
	stage, err := stageApplyOutputs(rootPath, artifactRootPath, outputs)
	if err != nil {
		issues = append(issues, err.Error())
		fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote("fail"))
		fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
		return nil
	}
	defer os.RemoveAll(stage.TempRoot)
	coherenceIssues := []string{}
	for _, item := range stage.Files {
		switch strings.ToLower(filepath.Ext(item.TargetPath)) {
		case ".md":
			coherenceIssues = append(coherenceIssues, verifyMarkdownTargets(stage, item)...)
			if strings.EqualFold(filepath.Base(item.TargetPath), "validation.md") {
				coherenceIssues = append(coherenceIssues, verifyValidationClaims(stage, item)...)
			}
		case ".yml", ".yaml":
			coherenceIssues = append(coherenceIssues, verifyYAMLTargets(stage, item)...)
		}
	}
	issues = append(issues, coherenceIssues...)
	status := "pass"
	if len(coherenceIssues) > 0 {
		status = "fail"
	}
	fmt.Fprintf(stdout, "VERIFY_APPLY_STATUS=%s\n", shellQuote(status))
	fmt.Fprintf(stdout, "VERIFY_APPLY_ISSUES=%s\n", shellQuote(mustJSON(issues, []string{})))
	return nil
}

func buildVerifyResult(outPath, planPath, graphPath, mergePath, usagePath, haltPath, status, confidenceRaw, issuesJSON, nextAction string, env map[string]string) error {
	plan := readJSONMapOptional(planPath)
	graph := readJSONMapOptional(graphPath)
	merge := readJSONMapOptional(mergePath)
	usage := readJSONMapOptional(usagePath)
	halt := readJSONMapOptional(haltPath)
	issues := []string{}
	_ = json.Unmarshal([]byte(issuesJSON), &issues)
	confidence := floatFromString(confidenceRaw, floatDefault(merge["average_confidence"], 0.6))
	if status == "" {
		status = "pass"
	}
	valueBar := evaluateValueBar(plan, graph, merge, usage, halt)
	graphLint := lintGraphShape(graph, merge, plan)
	failureClass := classifyVerifyFailure(status, issues, graphLint, valueBar, halt)
	recommended := recommendRerunTasks(graph, merge, issues, graphLint, failureClass)
	baseline := directWorkerBaseline(graph, merge, valueBar, graphLint)
	allIssues := append([]string{}, issues...)
	for _, warning := range stringList(graphLint["warnings"]) {
		allIssues = append(allIssues, warning)
	}
	if stringValue(baseline["verdict"]) == "direct_worker_likely_better" {
		allIssues = append(allIssues, "orchestration may not beat one strong direct worker for this graph shape")
		if status == "pass" {
			status = "review"
		}
	}
	if len(allIssues) > 0 && status == "pass" {
		status = "review"
	}
	payload := map[string]any{
		"status":                  status,
		"confidence":              confidence,
		"issues":                  allIssues,
		"failure_class":           failureClass,
		"root_cause_task":         rootCauseTask(merge, allIssues, recommended),
		"recommended_rerun_tasks": recommended,
		"value_bar":               valueBar,
		"graph_lint":              graphLint,
		"direct_worker_baseline":  baseline,
		"cloud_usage_artifact":    usagePath,
		"halt_artifact":           haltPath,
		"next_action":             nextAction,
	}
	if followUp := mapValue(plan["follow_up"]); len(followUp) > 0 {
		payload["follow_up"] = followUp
	}
	return writeJSONFile(outPath, payload)
}

func evaluateValueBar(plan, graph, merge, usage, halt map[string]any) map[string]any {
	tasks := workerGraphTasks(graph)
	mergeTasks := append(listValue(merge["planning_tasks"]), listValue(merge["tasks"])...)
	providers := map[string]bool{}
	liveCount := 0
	fallbackCount := 0
	cloudTokens := intAny(usage["total_estimated_tokens"])
	for _, raw := range mergeTasks {
		task := mapValue(raw)
		provider := stringValue(task["provider_actual"])
		if provider != "" {
			providers[provider] = true
		}
		if boolAny(task["used_live_model"]) {
			liveCount++
		} else {
			fallbackCount++
		}
	}
	outputs := listValue(mapValue(plan["apply"])["outputs"])
	parallelWidth := maxParallelWidth(tasks)
	hasValidation := hasWorkerType(tasks, "validation") || hasWorkerType(tasks, "verify")
	hasLocalExtraction := hasProvider(tasks, "ollama") || hasWorkerType(tasks, "extraction") || hasWorkerType(tasks, "inventory")
	hasApply := len(outputs) > 0
	hasApproval := boolDefault(mapValue(plan["apply"])["require_approval"], true)
	hasFollowUp := len(mapValue(graph["follow_up"])) > 0 || len(mapValue(plan["follow_up"])) > 0
	hasComparison := false
	for _, task := range tasks {
		if boolAny(mapValue(task["comparison"])["enabled"]) {
			hasComparison = true
			break
		}
	}
	dims := map[string]any{
		"breadth": map[string]any{
			"score":  scoreBool(len(tasks) >= 3 || parallelWidth > 1 || len(providers) > 1 || hasComparison),
			"reason": breadthReason(len(tasks), parallelWidth, len(providers), hasComparison),
		},
		"safety": map[string]any{
			"score":  scoreBool(hasApply && hasApproval),
			"reason": safetyReason(hasApply, hasApproval),
		},
		"cost": map[string]any{
			"score":  scoreBool(hasLocalExtraction || cloudTokens == 0),
			"reason": costReason(hasLocalExtraction, cloudTokens),
		},
		"validation": map[string]any{
			"score":  scoreBool(hasValidation),
			"reason": validationReason(hasValidation),
		},
		"rerunability": map[string]any{
			"score":  scoreBool(hasFollowUp || len(tasks) > 1),
			"reason": rerunReason(hasFollowUp, len(tasks)),
		},
		"traceability": map[string]any{
			"score":  1.0,
			"reason": "request, graph, task, merge, verify, usage, halt, and approval artifacts are materialized by DorkPipe",
		},
	}
	if len(halt) > 0 {
		dims["cost"].(map[string]any)["score"] = 0.25
		dims["cost"].(map[string]any)["reason"] = "cloud budget halt was triggered; cost control worked but the run needs review"
	}
	total := 0.0
	for _, raw := range dims {
		total += floatAny(mapValue(raw)["score"])
	}
	overall := total / float64(len(dims))
	return map[string]any{
		"overall_score": round2(overall),
		"verdict":       valueBarVerdict(overall),
		"dimensions":    dims,
	}
}

func lintGraphShape(graph, merge, plan map[string]any) map[string]any {
	tasks := workerGraphTasks(graph)
	warnings := []string{}
	serial := isMostlySerial(tasks)
	distinctProviders := map[string]bool{}
	distinctTypes := map[string]bool{}
	outputPaths := map[string]bool{}
	for _, task := range tasks {
		if provider := stringValue(task["provider"]); provider != "" {
			distinctProviders[provider] = true
		}
		if workerType := stringValue(task["worker_type"]); workerType != "" {
			distinctTypes[workerType] = true
		}
		if output := stringValue(task["output_path"]); output != "" {
			outputPaths[output] = true
		}
	}
	if serial && len(tasks) > 3 && len(distinctProviders) <= 1 {
		warnings = append(warnings, "graph is mostly serial with one provider; consider collapsing to one strong direct worker")
	}
	if len(tasks) > 4 && len(distinctTypes) <= 1 {
		warnings = append(warnings, "graph has many tasks with the same worker_type; split value is unclear")
	}
	if len(tasks) > 3 && len(outputPaths) <= 1 && len(listValue(mapValue(plan["apply"])["outputs"])) <= 1 {
		warnings = append(warnings, "graph has many workers but little output separation; orchestration may add handoff cost")
	}
	live := 0
	for _, raw := range append(listValue(merge["planning_tasks"]), listValue(merge["tasks"])...) {
		if boolAny(mapValue(raw)["used_live_model"]) {
			live++
		}
	}
	if len(tasks) > 0 && live == 0 {
		warnings = append(warnings, "no worker used a live model; output is fallback-only")
	}
	return map[string]any{
		"status":            ternaryString(len(warnings) == 0, "pass", "review"),
		"warnings":          warnings,
		"worker_task_count": len(tasks),
		"parallel_width":    maxParallelWidth(tasks),
		"provider_count":    len(distinctProviders),
		"worker_type_count": len(distinctTypes),
		"mostly_serial":     serial,
	}
}

func classifyVerifyFailure(status string, issues []string, graphLint, valueBar, halt map[string]any) string {
	if len(halt) > 0 {
		return "budget_halt"
	}
	joined := strings.ToLower(strings.Join(issues, " "))
	switch {
	case strings.Contains(joined, "fallback"):
		return "fallback_only"
	case strings.Contains(joined, "yaml"):
		return "schema_or_yaml"
	case strings.Contains(joined, "markdown link") || strings.Contains(joined, "reference target"):
		return "broken_references"
	case strings.Contains(joined, "artifact shape") || strings.Contains(joined, "preamble") || strings.Contains(joined, "narrated"):
		return "artifact_shape"
	case stringValue(graphLint["status"]) != "pass":
		return "low_value_graph"
	case stringValue(valueBar["verdict"]) == "weak_orchestration_value":
		return "weak_value_bar"
	case status != "pass":
		return "verification_review"
	default:
		return "none"
	}
}

func recommendRerunTasks(graph, merge map[string]any, issues []string, graphLint map[string]any, failureClass string) []string {
	tasks := workerGraphTasks(graph)
	known := map[string]bool{}
	for _, task := range tasks {
		known[stringValue(task["id"])] = true
	}
	recommended := map[string]bool{}
	for _, issue := range issues {
		first := strings.SplitN(issue, ":", 2)[0]
		first = strings.TrimSpace(first)
		if known[first] {
			recommended[first] = true
		}
	}
	if len(recommended) == 0 {
		for _, raw := range append(listValue(merge["planning_tasks"]), listValue(merge["tasks"])...) {
			task := mapValue(raw)
			status := strings.ToLower(stringValue(task["status"]))
			if status == "failed" || boolAny(task["budget_halt"]) || !boolAny(task["used_live_model"]) {
				id := stringValue(task["task_id"])
				if known[id] {
					recommended[id] = true
				}
			}
		}
	}
	if len(recommended) == 0 && (failureClass == "low_value_graph" || failureClass == "weak_value_bar") {
		for _, task := range tasks {
			id := stringValue(task["id"])
			if id != "" {
				recommended[id] = true
				break
			}
		}
	}
	return sortedTaskIDsFromSet(recommended)
}

func rootCauseTask(merge map[string]any, issues []string, recommended []string) string {
	if len(recommended) > 0 {
		return recommended[0]
	}
	for _, raw := range append(listValue(merge["planning_tasks"]), listValue(merge["tasks"])...) {
		task := mapValue(raw)
		if !boolAny(task["used_live_model"]) || strings.EqualFold(stringValue(task["status"]), "failed") {
			return stringValue(task["task_id"])
		}
	}
	if len(issues) > 0 {
		return "verify_final"
	}
	return ""
}

func directWorkerBaseline(graph, merge, valueBar, graphLint map[string]any) map[string]any {
	tasks := workerGraphTasks(graph)
	providers := map[string]bool{}
	for _, task := range tasks {
		if provider := stringValue(task["provider"]); provider != "" {
			providers[provider] = true
		}
	}
	serial := boolAny(graphLint["mostly_serial"])
	weakValue := stringValue(valueBar["verdict"]) == "weak_orchestration_value"
	verdict := "orchestration_adds_value"
	reason := "DorkPipe adds governance, traceability, apply safety, or rerunable task artifacts."
	if len(tasks) <= 2 && len(providers) <= 1 {
		verdict = "direct_worker_likely_sufficient"
		reason = "The graph is small and uses a single lane family; one strong worker may be simpler."
	}
	if len(tasks) > 3 && serial && len(providers) <= 1 && (weakValue || stringValue(graphLint["status"]) != "pass") {
		verdict = "direct_worker_likely_better"
		reason = "The graph is mostly serial, same-lane, and does not show enough value-bar benefit."
	}
	return map[string]any{
		"baseline": "one strong Codex or Claude worker reading the same declared sources and producing the requested output",
		"verdict":  verdict,
		"reason":   reason,
	}
}

func workerGraphTasks(graph map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, raw := range listValue(graph["tasks"]) {
		task := mapValue(raw)
		workerType := stringValue(task["worker_type"])
		if workerType == "merge" || workerType == "verify" {
			continue
		}
		if stringValue(task["id"]) == "" {
			continue
		}
		out = append(out, task)
	}
	return out
}

func maxParallelWidth(tasks []map[string]any) int {
	if len(tasks) == 0 {
		return 0
	}
	widthByDepth := map[int]int{}
	depthByID := map[string]int{}
	for len(depthByID) < len(tasks) {
		changed := false
		for _, task := range tasks {
			id := stringValue(task["id"])
			if id == "" || depthByID[id] != 0 {
				continue
			}
			deps := stringList(task["depends_on"])
			depth := 1
			ready := true
			for _, dep := range deps {
				if depDepth, ok := depthByID[dep]; ok {
					depth = maxInt(depth, depDepth+1)
					continue
				}
				if graphHasTask(tasks, dep) {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			depthByID[id] = depth
			widthByDepth[depth]++
			changed = true
		}
		if !changed {
			break
		}
	}
	maxWidth := 1
	for _, width := range widthByDepth {
		maxWidth = maxInt(maxWidth, width)
	}
	return maxWidth
}

func graphHasTask(tasks []map[string]any, id string) bool {
	for _, task := range tasks {
		if stringValue(task["id"]) == id {
			return true
		}
	}
	return false
}

func isMostlySerial(tasks []map[string]any) bool {
	if len(tasks) <= 1 {
		return true
	}
	return maxParallelWidth(tasks) <= 1
}

func hasWorkerType(tasks []map[string]any, workerType string) bool {
	for _, task := range tasks {
		if strings.EqualFold(stringValue(task["worker_type"]), workerType) {
			return true
		}
	}
	return false
}

func hasProvider(tasks []map[string]any, provider string) bool {
	for _, task := range tasks {
		if strings.EqualFold(stringValue(task["provider"]), provider) {
			return true
		}
	}
	return false
}

func hasCloudAuthorityTask(tasks []map[string]any) bool {
	for _, task := range tasks {
		provider := strings.ToLower(stringValue(task["provider"]))
		if provider != "codex" && provider != "claude" {
			continue
		}
		switch strings.ToLower(stringValue(task["worker_type"])) {
		case "architecture", "routing", "validation", "authoring", "repair":
			return true
		}
	}
	return false
}

func scoreBool(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

func breadthReason(taskCount, parallelWidth, providerCount int, comparison bool) string {
	parts := []string{fmt.Sprintf("%d worker task(s)", taskCount), fmt.Sprintf("parallel width %d", parallelWidth), fmt.Sprintf("%d provider(s)", providerCount)}
	if comparison {
		parts = append(parts, "comparison lanes enabled")
	}
	return strings.Join(parts, "; ")
}

func safetyReason(hasApply, hasApproval bool) string {
	if hasApply && hasApproval {
		return "durable writes are approval-gated apply outputs"
	}
	if hasApply {
		return "apply outputs exist but approval is not required"
	}
	return "no approval-gated apply outputs were declared"
}

func costReason(hasLocalExtraction bool, cloudTokens int) string {
	if hasLocalExtraction && cloudTokens > 0 {
		return fmt.Sprintf("cheap/local lanes were used before governed cloud spend (%d estimated cloud tokens)", cloudTokens)
	}
	if hasLocalExtraction {
		return "cheap/local lanes were used and no cloud token spend was recorded"
	}
	if cloudTokens > 0 {
		return fmt.Sprintf("cloud spend recorded without an obvious cheap/local extraction lane (%d estimated cloud tokens)", cloudTokens)
	}
	return "no cloud token spend was recorded"
}

func validationReason(hasValidation bool) string {
	if hasValidation {
		return "graph includes an explicit validation or verify stage"
	}
	return "no explicit validation worker was present beyond mechanical verification"
}

func rerunReason(hasFollowUp bool, taskCount int) string {
	if hasFollowUp {
		return "follow-up mode records selected rerun tasks"
	}
	if taskCount > 1 {
		return "multi-node graph can rerun targeted failed tasks"
	}
	return "single worker graph has little node-level rerun value"
}

func valueBarVerdict(score float64) string {
	switch {
	case score >= 0.75:
		return "strong_orchestration_value"
	case score >= 0.5:
		return "mixed_orchestration_value"
	default:
		return "weak_orchestration_value"
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func ternaryString(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
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

func resolveApplyOutputs(rootPath, artifactRootPath string, applyCfg map[string]any) ([]any, error) {
	outputs := listValue(applyCfg["outputs"])
	if len(outputs) > 0 {
		return outputs, nil
	}
	targetRoot := strings.TrimSpace(stringValue(applyCfg["target_root"]))
	if targetRoot == "" {
		return nil, nil
	}
	return inferApplyOutputs(rootPath, artifactRootPath, targetRoot, listValue(applyCfg["required_artifacts"]))
}

func inferApplyOutputs(rootPath, artifactRootPath, targetRoot string, required []any) ([]any, error) {
	_, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	artifactRoot, err := filepath.Abs(artifactRootPath)
	if err != nil {
		return nil, err
	}
	tasksRoot := filepath.Join(artifactRoot, "tasks")
	if info, statErr := os.Stat(tasksRoot); statErr != nil || !info.IsDir() {
		return nil, errors.New("no materialized apply outputs inferred from tasks/*/materialized")
	}
	type inferredApplyOutput struct {
		source string
		target string
	}
	byRel := map[string]inferredApplyOutput{}
	if err := filepath.WalkDir(tasksRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		marker := string(filepath.Separator) + "materialized" + string(filepath.Separator)
		idx := strings.Index(path, marker)
		if idx < 0 {
			return nil
		}
		rel := path[idx+len(marker):]
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
			return nil
		}
		sourceRel, err := filepath.Rel(artifactRoot, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if existing, exists := byRel[key]; exists {
			return fmt.Errorf("inferred apply artifact %s is duplicated between %s and %s", key, existing.source, filepath.ToSlash(sourceRel))
		}
		byRel[key] = inferredApplyOutput{
			source: filepath.ToSlash(sourceRel),
			target: filepath.ToSlash(filepath.Join(targetRoot, rel)),
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if len(byRel) == 0 {
		return nil, errors.New("no materialized apply outputs inferred from tasks/*/materialized")
	}
	for _, raw := range required {
		rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(fmt.Sprint(raw))))
		if rel == "" || rel == "." {
			continue
		}
		if _, ok := byRel[rel]; !ok {
			return nil, fmt.Errorf("required apply artifact is missing: %s", rel)
		}
	}
	keys := make([]string, 0, len(byRel))
	for key := range byRel {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	outputs := make([]any, 0, len(keys))
	for _, key := range keys {
		item := byRel[key]
		outputs = append(outputs, map[string]any{
			"source": item.source,
			"path":   item.target,
		})
	}
	return outputs, nil
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
	return applyResultsWithVerify(rootPath, artifactRootPath, planPath, approvalPath, resultPath, "", false)
}

func applyResultsWithVerify(rootPath, artifactRootPath, planPath, approvalPath, resultPath, verifyPath string, allowReviewApply bool) error {
	root, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	artifactRoot, err := filepath.Abs(artifactRootPath)
	if err != nil {
		return err
	}
	verifyStatus := ""
	if strings.TrimSpace(verifyPath) != "" {
		verifyStatus = strings.TrimSpace(stringValue(readJSONMapOptional(verifyPath)["status"]))
		if verifyStatus != "" && !strings.EqualFold(verifyStatus, "pass") && !strings.EqualFold(verifyStatus, "review") && !allowReviewApply {
			return writeJSONFile(resultPath, map[string]any{
				"status":        "skipped",
				"reason":        "verify status is " + verifyStatus + "; apply is blocked unless explicitly overridden",
				"verify_status": verifyStatus,
				"applied":       []any{},
			})
		}
	}
	plan := readJSONMap(planPath)
	applyCfg := mapValue(plan["apply"])
	outputs, err := resolveApplyOutputs(rootPath, artifactRootPath, applyCfg)
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
	if err != nil {
		return fail(err.Error())
	}
	if len(outputs) == 0 {
		return fail("no apply outputs declared or inferred")
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
	result := map[string]any{
		"status":  "applied",
		"applied": applied,
	}
	if verifyStatus != "" {
		result["verify_status"] = verifyStatus
	}
	if strings.EqualFold(verifyStatus, "review") {
		result["requires_human_review"] = true
		result["publish_allowed"] = false
		result["reason"] = "verify status is review; applied to workspace for concrete diff inspection only"
	}
	return writeJSONFile(resultPath, result)
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

func mountedGuestRootNotes(mountEnv string) []string {
	notes := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(mountEnv, "\n") {
		_, guest, ok := parseContainerMountSpec(line)
		if !ok {
			continue
		}
		guest = cleanGuestPath(guest)
		if guest == "" || seen[guest] {
			continue
		}
		seen[guest] = true
		switch guest {
		case "/work":
			continue
		case "/DesignNotes":
			notes = append(notes,
				fmt.Sprintf("- `%s` is a stable source-packet label for an external design corpus, not a repo-native path.", guest),
				"- Use it only while gathering evidence. Durable docs must cite a repo-native reference proven by an explicit source mapping or omit the path.",
			)
		default:
			notes = append(notes,
				fmt.Sprintf("- `%s` is a stable source-packet label, not a repo-native path.", guest),
				"- Use it only while gathering evidence. Durable docs must cite a repo-native reference proven by an explicit source mapping or omit the path.",
			)
		}
	}
	return notes
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
	agentsCfg := loadAgentsConfig(workflowPath)
	for key, value := range mapValue(orchestration["agents"]) {
		agentsCfg[key] = value
	}
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

	shared = orderedNativeGuidanceShared(shared)
	baselineContextPath := nativeGuidanceBaselineContextPath(shared)
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
			"require_approval":   boolDefault(apply["require_approval"], true),
			"outputs":            listValue(apply["outputs"]),
			"target_root":        stringValue(apply["target_root"]),
			"required_artifacts": listValue(apply["required_artifacts"]),
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
			"setup_hint":       stringValue(lane["setup_hint"]),
			"auth_hint":        stringValue(mapValue(lane["availability"])["auth_hint"]),
			"capabilities":     listValue(lane["capabilities"]),
			"context_window":   intFromAny(lane["context_window"]),
		})
	}
	for _, rawTask := range tasks {
		task, err := resolveAgentTask(mapValue(rawTask), agentsCfg, env)
		if err != nil {
			return fmt.Errorf("%s: %w", workflowPath, err)
		}
		task, err = applyTaskWorkerProfile(task, env)
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
			contextPaths := orderNativeGuidanceTaskContext(taskContextPaths(task), baselineContextPath)
			resolvedContextPaths, err := resolveScopeList(anySlice(contextPaths))
			if err != nil {
				return err
			}
			localLane := boolAny(laneSelection["local"])
			promptBrief := map[string]any{}
			promptBriefText := ""
			if localLane {
				promptBrief, promptBriefText, err = materializePromptBrief(taskDir, resolvedContextPaths, stringValue(laneSelection["provider"]), root, artifactRoot, sharedDir, inlineInputContext, inlineInputMaxBytes, inlineInputTotalMaxBytes)
				if err != nil {
					return fmt.Errorf("%s: task %q prompt brief: %w", workflowPath, taskID, err)
				}
			}
			sourcePacket := map[string]any{}
			sourcePacketText := ""
			if localLane {
				sourceRoots, err := resolveScopeList(listValue(mapValue(task["context"])["source_roots"]))
				if err != nil {
					return err
				}
				if len(sourceRoots) > 0 {
					sourcePacketText, err = renderSourcePacket(root, sourceRoots, taskAccess["read"], taskAccess["deny"], env["DOCKPIPE_CONTAINER_MOUNTS"])
					if err != nil {
						return fmt.Errorf("%s: task %q source packet: %w", workflowPath, taskID, err)
					}
					sourcePacketPath := filepath.Join(taskDir, "source-packet.md")
					if err := os.WriteFile(sourcePacketPath, []byte(sourcePacketText), 0o644); err != nil {
						return err
					}
					sourcePacket = map[string]any{
						"path":         filepath.ToSlash(filepath.Join("tasks", taskID, "source-packet.md")),
						"source_roots": sourceRoots,
						"authority":    "access.read",
					}
				}
			}
			dependsOn := []string{}
			for _, depRaw := range listValue(task["depends_on"]) {
				dep := fmt.Sprint(depRaw)
				if variant["compare_provider"] != "" && taskHasComparison(tasks, dep, compareProviders, compareScope) {
					dep = comparisonTaskID(dep, variant["compare_provider"])
				}
				dependsOn = append(dependsOn, dep)
			}
			effectiveTaskModel := taskModelForLane(taskModel, laneSelection)
			taskPayload := map[string]any{
				"id":                      taskID,
				"base_id":                 variant["base_task_id"],
				"reuse_existing":          reuseExisting,
				"agent":                   stringValue(task["agent"]),
				"role":                    stringValue(task["role"]),
				"authority":               mapValue(task["authority"]),
				"worker":                  stringValue(task["worker"]),
				"worker_policy":           mapValue(task["worker_policy"]),
				"comparison":              laneSelection["comparison"],
				"goal":                    stringValue(task["goal"]),
				"brief":                   stringValue(task["brief"]),
				"context":                 mapValue(task["context"]),
				"context_paths":           resolvedContextPaths,
				"prompt_brief":            promptBrief,
				"source_packet":           sourcePacket,
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
				"citations":               mustResolveScopeCitations(task, resolvedContextPaths, resolveScopeList),
				"materialize_outputs":     listValue(task["materialize_outputs"]),
				"startup_prompt":          startupPrompt,
				"include_agents_md":       includeAgentsMD,
				"accessible_paths":        accessiblePaths,
				"access":                  taskAccess,
				"model":                   effectiveTaskModel,
				"model_policy":            taskPolicy,
				"task_class": map[string]any{
					"name":      class.Name,
					"authority": class.Authority,
				},
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
					"Agent role: " + fallbackString(stringValue(taskPayload["role"]), stringValue(taskPayload["agent"])),
					"Goal: " + stringValue(taskPayload["goal"]),
					"Expected output: " + stringValue(taskPayload["expected_output"]),
				}
				if brief := strings.TrimSpace(stringValue(taskPayload["brief"])); brief != "" {
					lines = append(lines, "", "Brief:", brief)
				}
				if len(contextPaths) > 0 {
					lines = append(lines, "", "Context briefing paths:")
					for _, path := range contextPaths {
						lines = append(lines, "- "+path)
					}
				}
				lines = append(lines,
					"",
					"Rules:",
					"- Treat this as one bounded task, not the whole request.",
					"- Treat context briefing paths as starting context, not the full source boundary.",
					"- Access policy and mounted roots define what else you may inspect.",
					"- Ground substantive claims in exact source paths.",
					"- Keep durable target docs target-agnostic unless the task explicitly asks for workflow, runtime, provider, or model details.",
					"- Do not turn orchestration lane choices, provider names, model names, or mount mechanics into repo policy unless the task explicitly requires that and the cited sources support it.",
					"- Return concise markdown suitable for downstream merge.",
					"- Return the requested artifact content directly; do not narrate your tool workflow.",
					"- Call out uncertainty explicitly.",
				)
				if variant["compare_provider"] != "" {
					lines = append(lines, "", "Comparison mode:", fmt.Sprintf("- You are the %s fork for base task `%s`.", variant["compare_provider"], variant["base_task_id"]), "- Produce an independent answer for later side-by-side evaluation.")
				}
				prompt = strings.Join(lines, "\n") + "\n"
			}
			if laneContext := renderExecutionLanePromptContext(laneSelection, taskPayload); laneContext != "" {
				prompt = laneContext + "\n\n" + strings.TrimLeft(prompt, "\n")
			}
			outputContract := []string{}
			if boolString(fallbackString(env["DORKPIPE_ORCH_STRICT_OUTPUT_CONTRACT"], "true")) {
				outputContract = append(outputContract,
					"DorkPipe worker output contract:",
					"- Return only the requested artifact content.",
					"- Answer the task directly in the requested format.",
					"- Do not describe files you wrote, commands you ran, validation steps, source-control status, or container behavior.",
					"- Do not say you completed the task; produce the task answer itself.",
					"- Do not create or describe task.json, lane-selection.json, result.json, merge artifacts, or example artifacts in the response.",
					"- Do not write an execution plan, checklist, final report, or sample output unless the task explicitly asks for one.",
					"- Do not write or modify source files unless the task explicitly asks for edits.",
					"- Do not restate DorkPipe workflow/runtime/provider/model choices as durable target-repo policy unless the task explicitly asks for those details and the cited sources make them canonical.",
					"- Use the same output standard regardless of provider or model lane.",
				)
			}
			if materializedContract := renderMaterializeOutputContract(listValue(taskPayload["materialize_outputs"])); materializedContract != "" {
				if len(outputContract) > 0 {
					outputContract = append(outputContract, "")
				}
				outputContract = append(outputContract, materializedContract)
			}
			if variant["compare_provider"] != "" {
				outputContract = append(outputContract, "", "Comparison mode:", fmt.Sprintf("- You are the %s fork for base task `%s`.", variant["compare_provider"], variant["base_task_id"]), "- Produce an independent answer for side-by-side evaluation.", "- Do not mention that you are in a competition or compare yourself to other lanes.")
			}
			if len(outputContract) > 0 {
				prompt = strings.Join(outputContract, "\n") + "\n\n" + strings.TrimLeft(prompt, "\n")
			}
			briefingContext := promptBriefText
			if !localLane {
				briefingContext = renderBriefingContext(resolvedContextPaths, stringValue(laneSelection["provider"]), root, artifactRoot, sharedDir, inlineInputContext, inlineInputMaxBytes, inlineInputTotalMaxBytes)
			}
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
			if mountNotes := mountedGuestRootNotes(env["DOCKPIPE_CONTAINER_MOUNTS"]); len(mountNotes) > 0 {
				prefix = append(prefix, "", "Mounted source roots:")
				prefix = append(prefix, mountNotes...)
			}
			if includeAgentsMD {
				agentsPath := filepath.Join(root, "AGENTS.md")
				if rawAgents, err := os.ReadFile(agentsPath); err == nil {
					prefix = append(prefix, "", "AGENTS.md context:", "", strings.TrimRight(string(rawAgents), "\n"))
				}
			}
			if briefingContext != "" && !localLane {
				prefix = append(prefix, "", strings.TrimRight(briefingContext, "\n"))
			}
			if len(prefix) > 0 {
				if localLane {
					prompt = strings.TrimRight(prompt, "\n") + "\n\n" + strings.TrimSpace(strings.Join(prefix, "\n")) + "\n"
				} else {
					prompt = strings.TrimRight(strings.Join(prefix, "\n"), "\n") + "\n\n" + strings.TrimLeft(prompt, "\n")
				}
			}
			if briefingContext != "" && localLane {
				prompt = strings.TrimRight(prompt, "\n") + "\n\nReference context excerpts:\n\n" + strings.TrimRight(briefingContext, "\n") + "\n"
			}
			if localLane && sourcePacketText != "" {
				prompt = strings.TrimRight(prompt, "\n") + "\n\nDeterministic source packet:\n\n" + strings.TrimRight(sourcePacketText, "\n") + "\n"
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
	} else if requested == "auto" && workerMode == "prefer" && workerPreferred != "" && strings.EqualFold(stringValue(task["worker_type"]), "planning") {
		// Planning scouts are intentionally cheap, bounded preparation lanes.
		// Their declared worker preference remains authoritative unless a caller
		// explicitly overrides it above; downstream fanout can still escalate.
		requested = workerPreferred
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
	laneProvider := fallbackString(stringValue(lane["provider"]), fallbackString(stringValue(lane["resolver_hint"]), requested))
	taskModelProvider := expandEnv(stringValue(taskModel["provider"]), env)
	if model := stringValue(taskModel["model"]); model != "" && (taskModelProvider == "" || strings.EqualFold(taskModelProvider, laneProvider)) {
		lane["model"] = expandEnv(model, env)
	}
	return map[string]any{
		"task_id":              stringValue(task["id"]),
		"worker":               stringValue(task["worker"]),
		"worker_policy_mode":   workerMode,
		"worker_preference":    workerPreferred,
		"requested":            requested,
		"lane_id":              stringValue(lane["id"]),
		"provider":             laneProvider,
		"resolver_hint":        fallbackString(stringValue(lane["resolver_hint"]), fallbackString(stringValue(lane["provider"]), requested)),
		"model":                stringValue(lane["model"]),
		"cloud":                boolAny(lane["cloud"]),
		"local":                boolAny(lane["local"]),
		"available":            boolAny(lane["available"]),
		"missing_commands":     listValue(lane["missing_commands"]),
		"setup_hint":           stringValue(lane["setup_hint"]),
		"auth_hint":            stringValue(mapValue(lane["availability"])["auth_hint"]),
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

func taskModelForLane(taskModel, laneSelection map[string]any) map[string]any {
	laneProvider := stringValue(laneSelection["provider"])
	laneModel := stringValue(laneSelection["model"])
	laneContext := intFromAny(laneSelection["context_window"])
	if len(taskModel) > 0 {
		taskProvider := stringValue(taskModel["provider"])
		if taskProvider == "" || strings.EqualFold(taskProvider, laneProvider) {
			out := copyMap(taskModel)
			if stringValue(out["provider"]) == "" {
				out["provider"] = laneProvider
			}
			if stringValue(out["model"]) == "" {
				out["model"] = laneModel
			}
			if _, ok := out["num_ctx"]; !ok && laneContext > 0 {
				out["num_ctx"] = laneContext
			}
			return out
		}
	}
	out := map[string]any{
		"provider": laneProvider,
		"model":    laneModel,
	}
	if laneContext > 0 {
		out["num_ctx"] = laneContext
	}
	return out
}

func renderExecutionLanePromptContext(laneSelection, taskPayload map[string]any) string {
	if len(laneSelection) == 0 {
		return ""
	}
	lines := []string{"Execution lane (operational run metadata):"}
	if requested := stringValue(laneSelection["requested"]); requested != "" {
		lines = append(lines, "- Requested lane: "+requested)
	}
	if laneID := stringValue(laneSelection["lane_id"]); laneID != "" {
		lines = append(lines, "- Selected lane: "+laneID)
	}
	if provider := stringValue(laneSelection["provider"]); provider != "" {
		lines = append(lines, "- Provider: "+provider)
	}
	if model := stringValue(laneSelection["model"]); model != "" {
		lines = append(lines, "- Model: "+model)
	}
	if taskClass := mapValue(taskPayload["task_class"]); len(taskClass) > 0 {
		if name := stringValue(taskClass["name"]); name != "" {
			lines = append(lines, "- Work class: "+name)
		}
		if authority := stringValue(taskClass["authority"]); authority != "" {
			lines = append(lines, "- Authority: "+authority)
		}
	}
	if reasons := stringList(laneSelection["reasons"]); len(reasons) > 0 {
		lines = append(lines, "- Selection rationale: "+strings.Join(reasons, "; "))
	}
	if policy := mapValue(taskPayload["model_policy"]); len(policy) > 0 {
		lines = append(lines, "- Model policy: `"+mustJSON(policy, map[string]any{})+"`")
	}
	lines = append(lines,
		"- This metadata describes the current run only; do not turn it into durable repository policy.",
		"- Do not substitute lane selection for source evidence; follow the task scope and report uncertainty when evidence is unavailable.",
	)
	return strings.Join(lines, "\n")
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
	case "example_brain_baseline":
		path := strings.TrimSpace(env["DORKPIPE_ORCH_EXAMPLE_BRAIN_BASELINE"])
		if path == "" {
			return "", errors.New("example_brain_baseline collector requires DORKPIPE_ORCH_EXAMPLE_BRAIN_BASELINE")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read example brain baseline: %w", err)
		}
		return string(raw), nil
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
		startingPoints := stringList(entry["starting_points"])
		if len(startingPoints) > 0 {
			lines = append(lines, "", "## Starting Points", "")
			for _, rel := range startingPoints {
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

func orderedNativeGuidanceShared(shared []any) []any {
	ordered := make([]any, 0, len(shared))
	for _, raw := range shared {
		if stringValue(mapValue(raw)["collector"]) == "example_brain_baseline" {
			ordered = append(ordered, raw)
		}
	}
	for _, raw := range shared {
		if stringValue(mapValue(raw)["collector"]) != "example_brain_baseline" {
			ordered = append(ordered, raw)
		}
	}
	return ordered
}

func nativeGuidanceBaselineContextPath(shared []any) string {
	for _, raw := range shared {
		entry := mapValue(raw)
		if stringValue(entry["collector"]) != "example_brain_baseline" {
			continue
		}
		if rel := strings.TrimSpace(stringValue(entry["path"])); rel != "" {
			return filepath.ToSlash(filepath.Join("shared", rel))
		}
	}
	return ""
}

func orderNativeGuidanceTaskContext(values []string, baseline string) []string {
	if baseline == "" || !containsString(values, baseline) {
		return append([]string{}, values...)
	}
	return prependUniqueString(values, baseline)
}

func prependUniqueString(values []string, first string) []string {
	out := []string{first}
	for _, value := range values {
		if value != first {
			out = append(out, value)
		}
	}
	return out
}

const (
	sourcePacketMaxFiles     = 32
	sourcePacketMaxFileBytes = 1200
	sourcePacketMaxBytes     = 6000
)

type sourcePacketRoot struct {
	hostPath    string
	displayPath string
}

type sourcePacketFile struct {
	hostPath    string
	displayPath string
}

var sourcePacketExtensions = map[string]bool{
	".bash": true, ".c": true, ".cfg": true, ".conf": true, ".cpp": true, ".cs": true,
	".css": true, ".go": true, ".h": true, ".hcl": true, ".hpp": true, ".html": true,
	".ini": true, ".java": true, ".js": true, ".json": true, ".jsx": true, ".md": true,
	".mjs": true, ".pipe": true, ".ps1": true, ".py": true, ".rb": true, ".rs": true,
	".scss": true, ".sh": true, ".sql": true, ".toml": true, ".ts": true, ".tsx": true,
	".txt": true, ".xml": true, ".yaml": true, ".yml": true,
}

var sourcePacketIgnoredDirs = map[string]bool{
	".dockpipe": true, ".git": true, ".staging": true, "bin": true, "build": true,
	"coverage": true, "dist": true, "node_modules": true,
}

// renderSourcePacket produces a bounded, deterministic evidence artifact for a local lane. Source
// roots are ordered by the task; each root is walked lexically and may only yield text files that are
// within access.read and outside access.deny. Guest roots resolve through declared mounts before any
// filesystem walk, so a packet cannot silently widen a worker's declared source authority.
func renderSourcePacket(root string, sourceRoots, readRoots, denyRoots []string, mountEnv string) (string, error) {
	if len(readRoots) == 0 {
		return "", errors.New("context.source_roots requires access.read for local source packets")
	}
	resolvedReads, err := resolveSourcePacketPaths(root, readRoots, mountEnv)
	if err != nil {
		return "", fmt.Errorf("resolve access.read: %w", err)
	}
	resolvedDenies, err := resolveSourcePacketPaths(root, denyRoots, mountEnv)
	if err != nil {
		return "", fmt.Errorf("resolve access.deny: %w", err)
	}
	resolvedSources, err := resolveSourcePacketRoots(root, sourceRoots, mountEnv)
	if err != nil {
		return "", fmt.Errorf("resolve context.source_roots: %w", err)
	}
	for _, source := range resolvedSources {
		if !pathWithinAny(source.hostPath, resolvedReads) {
			return "", fmt.Errorf("source root is outside access.read: %s", source.displayPath)
		}
	}

	files := []sourcePacketFile{}
	seen := map[string]bool{}
	for _, source := range resolvedSources {
		if len(files) >= sourcePacketMaxFiles {
			break
		}
		walkErr := filepath.WalkDir(source.hostPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if pathWithinAny(path, resolvedDenies) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				if sourcePacketIgnoredDirs[strings.ToLower(entry.Name())] {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !sourcePacketExtensions[strings.ToLower(filepath.Ext(entry.Name()))] || !pathWithinAny(path, resolvedReads) {
				return nil
			}
			if !seen[path] {
				seen[path] = true
				files = append(files, sourcePacketFile{
					hostPath:    path,
					displayPath: sourcePacketChildDisplayPath(source, path),
				})
			}
			if len(files) >= sourcePacketMaxFiles {
				return filepath.SkipDir
			}
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}
	}

	lines := []string{
		"# Deterministic Source Packet",
		"",
		"- Authority: read-only evidence from `access.read`; paths under `access.deny` are excluded.",
		fmt.Sprintf("- Bounds: at most %d text files, %d bytes per file, and %d source bytes total.", sourcePacketMaxFiles, sourcePacketMaxFileBytes, sourcePacketMaxBytes),
		"- Ordering: declared source-root order, then lexical path order within each root.",
		"- Text filter: source and documentation extensions only; generated/cache directories and symlinks are excluded.",
		"",
		"## Source roots",
		"",
	}
	for _, source := range resolvedSources {
		lines = append(lines, "- `"+source.displayPath+"` (allowed by `access.read`)")
	}
	remaining := sourcePacketMaxBytes
	included := 0
	for _, file := range files {
		if remaining <= 0 {
			break
		}
		content, err := readSourcePacketFile(file.hostPath, minInt(sourcePacketMaxFileBytes, remaining))
		if err != nil {
			return "", err
		}
		if len(content) == 0 || !isSourcePacketText(content) {
			continue
		}
		content = truncateUTF8(content, minInt(sourcePacketMaxFileBytes, remaining))
		remaining -= len(content)
		included++
		text := strings.ReplaceAll(strings.TrimRight(string(content), "\n"), "```", "``\\`")
		lines = append(lines, "", "## "+file.displayPath, "", "```text", text, "```")
	}
	if included == 0 {
		lines = append(lines, "", "No readable text files matched the declared roots and packet policy.")
	} else if remaining == 0 || len(files) >= sourcePacketMaxFiles {
		lines = append(lines, "", "Packet bounds reached; additional allowed files were not included.")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func resolveSourcePacketRoots(root string, paths []string, mountEnv string) ([]sourcePacketRoot, error) {
	resolved := []sourcePacketRoot{}
	seen := map[string]bool{}
	for _, raw := range paths {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		hostPath, err := resolveSourcePacketPath(root, value, mountEnv)
		if err != nil {
			return nil, err
		}
		if seen[hostPath] {
			continue
		}
		displayPath, err := sourcePacketRootDisplayPath(root, value, hostPath, mountEnv)
		if err != nil {
			return nil, err
		}
		seen[hostPath] = true
		resolved = append(resolved, sourcePacketRoot{hostPath: hostPath, displayPath: displayPath})
	}
	return resolved, nil
}

func resolveSourcePacketPaths(root string, paths []string, mountEnv string) ([]string, error) {
	resolved := []string{}
	seen := map[string]bool{}
	for _, raw := range paths {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		path, err := resolveSourcePacketPath(root, value, mountEnv)
		if err != nil {
			return nil, err
		}
		if !seen[path] {
			seen[path] = true
			resolved = append(resolved, path)
		}
	}
	return resolved, nil
}

func resolveSourcePacketPath(root, value, mountEnv string) (string, error) {
	if strings.HasPrefix(filepath.ToSlash(value), "/") {
		if path, _, ok := resolveGuestMountTarget(value, mountEnv); ok {
			return path, nil
		}
		if path, _, ok := resolvePrimaryWorkTarget(root, value); ok {
			return path, nil
		}
	}
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return abs, nil
}

func pathWithinAny(path string, roots []string) bool {
	for _, root := range roots {
		if withinRoot(root, path) {
			return true
		}
	}
	return false
}

func sourcePacketRootDisplayPath(root, declared, hostPath, mountEnv string) (string, error) {
	declaredSlash := filepath.ToSlash(declared)
	if strings.HasPrefix(declaredSlash, "/") {
		if _, _, ok := resolveGuestMountTarget(declared, mountEnv); ok {
			return cleanGuestPath(declared), nil
		}
		if _, _, ok := resolvePrimaryWorkTarget(root, declared); ok {
			return cleanGuestPath(declared), nil
		}
	}
	if !filepath.IsAbs(declared) {
		return filepath.ToSlash(filepath.Clean(declared)), nil
	}
	if guestPath, ok := sourcePacketGuestPathForHost(hostPath, mountEnv); ok {
		return guestPath, nil
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, hostPath)
	if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return filepath.ToSlash(rel), nil
	}
	return "", fmt.Errorf("external source root must use a declared guest mount")
}

func sourcePacketGuestPathForHost(hostPath, mountEnv string) (string, bool) {
	bestHostRoot := ""
	bestGuestRoot := ""
	for _, line := range strings.Split(mountEnv, "\n") {
		host, guest, ok := parseContainerMountSpec(line)
		if !ok {
			continue
		}
		hostRoot, err := filepath.Abs(host)
		if err != nil || !withinRoot(hostRoot, hostPath) || len(hostRoot) <= len(bestHostRoot) {
			continue
		}
		bestHostRoot = hostRoot
		bestGuestRoot = cleanGuestPath(guest)
	}
	if bestHostRoot == "" || bestGuestRoot == "" {
		return "", false
	}
	rel, err := filepath.Rel(bestHostRoot, hostPath)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return bestGuestRoot, true
	}
	return strings.TrimRight(bestGuestRoot, "/") + "/" + filepath.ToSlash(rel), true
}

func sourcePacketChildDisplayPath(root sourcePacketRoot, hostPath string) string {
	rel, err := filepath.Rel(root.hostPath, hostPath)
	if err != nil || rel == "." {
		return root.displayPath
	}
	rel = filepath.ToSlash(rel)
	if root.displayPath == "." {
		return rel
	}
	return strings.TrimRight(root.displayPath, "/") + "/" + rel
}

func readSourcePacketFile(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, int64(limit)))
}

func isSourcePacketText(content []byte) bool {
	for _, value := range content {
		if value == 0 {
			return false
		}
	}
	return true
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

func renderBriefingContext(contextPaths []string, provider, root, artifactRoot, sharedDir string, enabled bool, maxBytes, totalMaxBytes int) string {
	if !enabled || len(contextPaths) == 0 || totalMaxBytes <= 0 {
		return ""
	}
	remaining := totalMaxBytes
	sections := []string{
		"Briefing context excerpts:",
		"",
		"Use these excerpts as required briefing for this task. They are not a source boundary; cite exact paths in prose when useful, but keep the final answer compact.",
	}
	included := 0
	ordered := append([]string{}, contextPaths...)
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

// materializePromptBrief keeps the bounded context assembled for a local lane as a run artifact as
// well as prompt evidence. It reuses the same deterministic path ordering and byte limits as inline
// briefing context, so it never creates a durable normalized copy of repository documentation.
func materializePromptBrief(taskDir string, contextPaths []string, provider, root, artifactRoot, sharedDir string, enabled bool, maxBytes, totalMaxBytes int) (map[string]any, string, error) {
	text := renderBriefingContext(contextPaths, provider, root, artifactRoot, sharedDir, enabled, maxBytes, totalMaxBytes)
	if text == "" {
		return map[string]any{}, "", nil
	}
	path := filepath.Join(taskDir, "prompt-brief.md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return nil, "", err
	}
	return map[string]any{
		"path":          filepath.ToSlash(filepath.Join("tasks", filepath.Base(taskDir), "prompt-brief.md")),
		"context_paths": contextPaths,
		"authority":     "context briefing paths",
	}, text, nil
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

func mustResolveScopeCitations(task map[string]any, contextPaths []string, resolve func([]any) ([]string, error)) []string {
	paths, err := resolve(listValue(fallbackAny(task["citations"], anySlice(contextPaths))))
	if err != nil {
		return contextPaths
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

func resolveAgentTask(task, agents map[string]any, env map[string]string) (map[string]any, error) {
	agentID := strings.TrimSpace(expandEnv(stringValue(task["agent"]), env))
	if agentID == "" {
		return task, nil
	}
	def := mapValue(agents[agentID])
	if len(def) == 0 {
		return nil, fmt.Errorf("task %q references unknown agent %q", stringValue(task["id"]), agentID)
	}
	out := copyMap(def)
	out["agent"] = agentID
	out["id"] = task["id"]
	for _, key := range []string{"constraints"} {
		merged := append(listValue(def[key]), listValue(task[key])...)
		if len(merged) > 0 {
			out[key] = merged
		}
	}
	for key, value := range task {
		if key == "constraints" {
			continue
		}
		out[key] = value
	}
	if stringValue(out["role"]) == "" {
		out["role"] = agentID
	}
	if len(mapValue(out["context"])) == 0 {
		out["context"] = map[string]any{}
	}
	return out, nil
}

func loadAgentsConfig(workflowPath string) map[string]any {
	workflowDir := filepath.Dir(workflowPath)
	searchRoot := agentsConfigSearchRoot(workflowDir)
	for dir := workflowDir; ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, "agents.yml")
		if _, err := os.Stat(path); err == nil {
			payload := readYAMLMapOptional(path)
			if agents := mapValue(payload["agents"]); len(agents) > 0 {
				return agents
			}
			return mapValue(mapValue(payload["agent"])["agents"])
		}
		if dir == searchRoot {
			break
		}
	}
	return map[string]any{}
}

func agentsConfigSearchRoot(workflowDir string) string {
	workflowRoot := ""
	for dir := workflowDir; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "package.yml")); err == nil {
			return dir
		}
		if workflowRoot == "" && filepath.Base(dir) == "workflows" {
			workflowRoot = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	if workflowRoot != "" {
		return workflowRoot
	}
	return workflowDir
}

func taskContextPaths(task map[string]any) []string {
	context := mapValue(task["context"])
	values := []string{}
	for _, key := range []string{"required_artifacts", "seed_paths", "source_roots"} {
		for _, raw := range listValue(context[key]) {
			value := strings.TrimSpace(fmt.Sprint(raw))
			if value != "" && !containsString(values, value) {
				values = append(values, value)
			}
		}
	}
	return values
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
