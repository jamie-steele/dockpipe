package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

func main() {
	if len(os.Args) < 3 {
		fatalf("usage: test-assert <mode> <path>")
	}
	mode := os.Args[1]
	target := os.Args[2]
	switch mode {
	case "normalize-findings":
		assertNormalizeFindings(target)
	case "normalize-gosec":
		assertNormalizeGosec(target)
	case "insights-main":
		assertInsightsMain(target)
	case "insights-category":
		assertInsightsCategory(target)
	case "orchestration-lanes-initial":
		assertOrchestrationLanesInitial(target)
	case "orchestration-lanes-snapshot":
		assertOrchestrationLanesSnapshot(target)
	case "orchestration-lanes-followup":
		assertOrchestrationLanesFollowup(target)
	case "orchestration-force-codex":
		assertOrchestrationForceCodex(target)
	case "orchestration-brain-codex":
		assertOrchestrationBrainCodex(target)
	case "orchestration-compare":
		assertOrchestrationCompare(target)
	case "orchestration-cloud-usage":
		assertOrchestrationCloudUsage(target)
	default:
		fatalf("unknown assertion mode: %s", mode)
	}
}

func assertNormalizeFindings(path string) {
	data := readJSON(path)
	assert(data["schema_version"] == "1.0", "unexpected schema_version: %#v", data["schema_version"])
	_, ok := data["findings"].([]any)
	assert(ok, "findings is not an array")
}

func assertNormalizeGosec(path string) {
	data := readJSON(path)
	_, ok := data["Issues"].([]any)
	assert(ok, "Issues is not an array")
}

func assertInsightsMain(path string) {
	data := readJSON(path)
	assert(data["kind"] == "dockpipe_user_insights", "unexpected kind: %#v", data["kind"])
	insights := asArray(data["insights"], "insights")
	assert(len(insights) == 2, "expected 2 insights, got %d", len(insights))
	categories := map[string]bool{}
	statuses := map[string]bool{}
	for _, item := range insights {
		obj := asObject(item, "insight")
		categories[asString(obj["category"], "category")] = true
		statuses[asString(obj["status"], "status")] = true
	}
	assert(categories["compliance"] && categories["convention"] && len(categories) == 2, "unexpected categories: %#v", categories)
	assert(statuses["accepted"] && statuses["pending"] && len(statuses) == 2, "unexpected statuses: %#v", statuses)
}

func assertInsightsCategory(path string) {
	data := readJSONAny(path)
	items := asArray(data, "category export")
	assert(len(items) >= 1, "expected at least one category item")
}

func assertOrchestrationLanesInitial(root string) {
	lanePlan := readJSON(filepath.Join(root, "lanes", "plan.json"))
	tasks := asArray(lanePlan["tasks"], "lane tasks")
	assert(len(tasks) == 6, "expected 6 planned tasks, got %d", len(tasks))
	providers := map[string]string{}
	for _, task := range tasks {
		obj := asObject(task, "lane task")
		providers[asString(obj["task_id"], "task_id")] = asString(obj["provider"], "provider")
	}
	for _, taskID := range []string{"contract_brain", "workflow_brain", "planner_brain", "repo_shape", "package_contracts", "safety_model"} {
		assert(providers[taskID] == "ollama", "unexpected provider for %s: %s", taskID, providers[taskID])
	}
	explicitLocal := map[string]bool{"contract_brain": true, "workflow_brain": true, "planner_brain": true, "repo_shape": true}
	for _, task := range tasks {
		obj := asObject(task, "lane task")
		taskID := asString(obj["task_id"], "task_id")
		gated, _ := obj["gated_by_baseline"].(bool)
		assert(gated || explicitLocal[taskID], "task should be gated or explicitly local: %s", taskID)
	}
	metrics := readJSONLines(filepath.Join(root, "training", "metrics.jsonl"))
	assert(len(metrics) == 6, "expected 6 training metrics, got %d", len(metrics))
	for _, metric := range metrics {
		assert(metric["used_live_model"] == false, "metric used live model: %#v", metric)
		assert(metric["training_mode"] == "observe", "unexpected training mode: %#v", metric)
		_, hasTokens := metric["estimated_total_tokens"]
		assert(hasTokens, "missing estimated_total_tokens")
		assert(asString(metric["started_at"], "started_at") != "", "missing started_at")
		assert(asString(metric["finished_at"], "finished_at") != "", "missing finished_at")
		assert(isJSONNumber(metric["duration_ms"]), "duration_ms is not numeric")
	}
	for taskID := range providers {
		result := readJSON(filepath.Join(root, "tasks", taskID, "result.json"))
		assert(asString(result["lane_id"], "lane_id") != "", "missing lane_id for %s", taskID)
		selection := asObject(result["lane_selection"], "lane_selection")
		assert(selection["task_id"] == taskID, "unexpected lane selection for %s", taskID)
		assert(asString(result["started_at"], "started_at") != "", "missing started_at for %s", taskID)
		assert(asString(result["finished_at"], "finished_at") != "", "missing finished_at for %s", taskID)
		assert(isJSONNumber(result["duration_ms"]), "duration_ms is not numeric for %s", taskID)
	}
	prompt := readText(filepath.Join(root, "tasks", "package_contracts", "prompt.md"))
	assertContains(prompt, "Dependency context from completed upstream tasks:")
	assertContains(prompt, "### planner_brain")
	assertNotContains(prompt, "### contract_brain")
	assertNotContains(prompt, "### workflow_brain")
	assertContains(prompt, "AGENTS.md context:")
	assertContains(prompt, "DockPipe Root Router")
	graphTasks := taskMap(readJSON(filepath.Join(root, "task-graph.json")), "id")
	assert(graphTasks["contract_brain"]["worker_type"] == "planning", "unexpected contract_brain worker_type")
	assert(graphTasks["workflow_brain"]["worker_type"] == "planning", "unexpected workflow_brain worker_type")
	assert(graphTasks["planner_brain"]["worker_type"] == "planning", "unexpected planner_brain worker_type")
	assertStringArrayEqual(asStringArray(graphTasks["planner_brain"]["depends_on"], "depends_on"), []string{"contract_brain", "workflow_brain"}, "planner_brain depends_on")
	assertStringArrayContains(asStringArray(graphTasks["package_contracts"]["depends_on"], "depends_on"), "planner_brain", "package_contracts depends_on")
}

func assertOrchestrationLanesSnapshot(root string) {
	snapshot := map[string]string{
		"package_contracts": strconv.FormatInt(statNano(filepath.Join(root, "tasks", "package_contracts", "result.json")), 10),
		"repo_shape":        strconv.FormatInt(statNano(filepath.Join(root, "tasks", "repo_shape", "result.json")), 10),
		"safety_model":      strconv.FormatInt(statNano(filepath.Join(root, "tasks", "safety_model", "result.json")), 10),
	}
	writeJSON(filepath.Join(root, "before-followup.json"), snapshot)
}

func assertOrchestrationLanesFollowup(root string) {
	beforeRaw := readJSON(filepath.Join(root, "before-followup.json"))
	before := map[string]int64{}
	for key, value := range beforeRaw {
		parsed, err := strconv.ParseInt(asString(value, key), 10, 64)
		if err != nil {
			fatalf("parse before-followup %s: %v", key, err)
		}
		before[key] = parsed
	}
	request := readJSON(filepath.Join(root, "request.json"))
	followUp := asObject(request["follow_up"], "follow_up")
	assert(followUp["enabled"] == true, "follow_up not enabled")
	assert(asString(followUp["request"], "request") != "", "missing follow_up request")
	assert(asString(followUp["goal"], "goal") != "", "missing follow_up goal")
	assertStringArrayEqual(asStringArray(followUp["selected_tasks"], "selected_tasks"), []string{"package_contracts"}, "selected_tasks")
	assertStringArrayEqual(asStringArray(followUp["rerun_tasks"], "rerun_tasks"), []string{"package_contracts"}, "rerun_tasks")
	graphTasks := taskMap(readJSON(filepath.Join(root, "task-graph.json")), "id")
	assert(graphTasks["package_contracts"]["reuse_existing"] == false, "package_contracts should rerun")
	assert(graphTasks["repo_shape"]["reuse_existing"] == true, "repo_shape should be reused")
	assert(graphTasks["safety_model"]["reuse_existing"] == true, "safety_model should be reused")
	assert(statNano(filepath.Join(root, "tasks", "package_contracts", "result.json")) > before["package_contracts"], "package_contracts result was not refreshed")
	assert(statNano(filepath.Join(root, "tasks", "repo_shape", "result.json")) == before["repo_shape"], "repo_shape result changed")
	assert(statNano(filepath.Join(root, "tasks", "safety_model", "result.json")) == before["safety_model"], "safety_model result changed")
	prompt := readText(filepath.Join(root, "tasks", "package_contracts", "prompt.md"))
	assertContains(prompt, "Follow-up repair mode:")
	assertContains(prompt, "Follow-up request:")
	assertContains(prompt, "Follow-up goal:")
}

func assertOrchestrationForceCodex(root string) {
	tasks := taskMap(readJSON(filepath.Join(root, "lanes", "plan.json")), "task_id")
	for _, taskID := range []string{"contract_brain", "workflow_brain", "planner_brain", "repo_shape", "package_contracts", "safety_model"} {
		task := tasks[taskID]
		assert(task["requested"] == "codex", "unexpected requested provider for %s", taskID)
		assert(task["provider"] == "codex", "unexpected provider for %s", taskID)
		assert(task["lane_id"] == "codex.cli.default", "unexpected lane for %s", taskID)
	}
	assert(tasks["contract_brain"]["worker_preference"] == "ollama", "contract_brain worker_preference changed")
	assert(tasks["repo_shape"]["worker_preference"] == "ollama", "repo_shape worker_preference changed")
	request := readJSON(filepath.Join(root, "request.json"))
	assert(request["force_provider"] == "codex", "unexpected force_provider")
	assert(request["force_provider_scope"] == "auto", "unexpected force_provider_scope")
}

func assertOrchestrationBrainCodex(root string) {
	tasks := taskMap(readJSON(filepath.Join(root, "task-graph.json")), "id")
	assert(tasks["contract_brain"]["provider"] == "codex", "contract_brain provider")
	assert(tasks["workflow_brain"]["provider"] == "ollama", "workflow_brain provider")
	assert(tasks["planner_brain"]["provider"] == "codex", "planner_brain provider")
	assert(tasks["repo_shape"]["provider"] == "ollama", "repo_shape provider")
	assert(tasks["package_contracts"]["provider"] == "ollama", "package_contracts provider")
	assert(tasks["safety_model"]["provider"] == "ollama", "safety_model provider")
	assertStringArrayEqual(asStringArray(tasks["planner_brain"]["depends_on"], "depends_on"), []string{"contract_brain", "workflow_brain"}, "planner_brain depends_on")
	for _, taskID := range []string{"repo_shape", "package_contracts", "safety_model"} {
		assertStringArrayContains(asStringArray(tasks[taskID]["depends_on"], "depends_on"), "planner_brain", taskID+" depends_on")
	}
}

func assertOrchestrationCompare(root string) {
	tasks := taskMap(readJSON(filepath.Join(root, "lanes", "plan.json")), "task_id")
	expected := map[string]string{
		"contract_brain__ollama":    "ollama",
		"contract_brain__codex":     "codex",
		"contract_brain__claude":    "claude",
		"workflow_brain__ollama":    "ollama",
		"workflow_brain__codex":     "codex",
		"workflow_brain__claude":    "claude",
		"planner_brain__ollama":     "ollama",
		"planner_brain__codex":      "codex",
		"planner_brain__claude":     "claude",
		"repo_shape__ollama":        "ollama",
		"repo_shape__codex":         "codex",
		"repo_shape__claude":        "claude",
		"package_contracts__ollama": "ollama",
		"package_contracts__codex":  "codex",
		"package_contracts__claude": "claude",
		"safety_model__ollama":      "ollama",
		"safety_model__codex":       "codex",
		"safety_model__claude":      "claude",
	}
	validBaseTasks := map[string]bool{"contract_brain": true, "workflow_brain": true, "planner_brain": true, "repo_shape": true, "package_contracts": true, "safety_model": true}
	for taskID, provider := range expected {
		task := tasks[taskID]
		assert(task["provider"] == provider, "unexpected lane provider for %s", taskID)
		comparison := asObject(task["comparison"], "comparison")
		assert(comparison["enabled"] == true, "comparison not enabled for %s", taskID)
		assert(validBaseTasks[asString(task["base_task_id"], "base_task_id")], "unexpected base_task_id for %s", taskID)
	}
	graph := readJSON(filepath.Join(root, "task-graph.json"))
	graphTasks := taskMap(graph, "id")
	for taskID, provider := range expected {
		task := graphTasks[taskID]
		assert(task["provider"] == provider, "unexpected graph provider for %s", taskID)
		model := asString(task["model"], "model")
		assert(model != "", "missing model for %s", taskID)
		if provider == "codex" {
			assert(model == "test-codex-model", "unexpected codex model for %s", taskID)
		}
		if provider == "claude" {
			assert(model == "test-claude-model", "unexpected claude model for %s", taskID)
		}
		if provider == "ollama" {
			assert(model == "test-ollama-model", "unexpected ollama model for %s", taskID)
		}
		prompt := readText(filepath.Join(root, "tasks", taskID, "prompt.md"))
		assertContains(prompt, "DorkPipe worker output contract:")
		assertContains(prompt, "Return only the requested artifact content.")
		assertContains(prompt, "Do not create or describe task.json")
		assertContains(prompt, "AGENTS.md context:")
		assertContains(prompt, "DockPipe Root Router")
		assertContains(prompt, "Briefing context excerpts:")
		assertContains(prompt, "shared/repo-map.md")
		if provider == "ollama" {
			assert(hasPrefix(prompt, "DorkPipe worker output contract:"), "ollama prompt should start with output contract")
			assertContains(prompt, "Local model lane guidance:")
		}
	}
	concurrency := asObject(graph["concurrency"], "concurrency")
	assert(asFloat(concurrency["max_workers"], "max_workers") >= 4, "max_workers too low")
	assert(asFloat(concurrency["max_local_workers"], "max_local_workers") >= 2, "max_local_workers too low")
	assert(asFloat(concurrency["max_cloud_workers"], "max_cloud_workers") >= 2, "max_cloud_workers too low")
	request := readJSON(filepath.Join(root, "request.json"))
	assertStringArrayEqual(asStringArray(request["compare_providers"], "compare_providers"), []string{"ollama", "codex", "claude"}, "compare_providers")
	assert(request["compare_scope"] == "auto", "unexpected compare_scope")
}

func assertOrchestrationCloudUsage(root string) {
	usage := readJSON(filepath.Join(root, "cloud-usage.json"))
	assert(asFloat(usage["cloud_task_count"], "cloud_task_count") == 3, "cloud_task_count")
	assert(asFloat(usage["total_estimated_input_tokens"], "total_estimated_input_tokens") == 165, "input tokens")
	assert(asFloat(usage["total_estimated_output_tokens"], "total_estimated_output_tokens") == 85, "output tokens")
	assert(asFloat(usage["total_estimated_tokens"], "total_estimated_tokens") == 250, "total tokens")
	assert(asFloat(usage["total_duration_ms"], "total_duration_ms") == 2400, "total duration")
	providers := asObject(usage["providers"], "providers")
	codex := asObject(providers["codex"], "codex")
	claude := asObject(providers["claude"], "claude")
	assert(asFloat(codex["task_count"], "codex.task_count") == 2, "codex task count")
	assert(asFloat(codex["estimated_tokens"], "codex.estimated_tokens") == 200, "codex tokens")
	assert(asFloat(codex["duration_ms"], "codex.duration_ms") == 2000, "codex duration")
	assert(asFloat(claude["task_count"], "claude.task_count") == 1, "claude task count")
	assert(asFloat(claude["estimated_tokens"], "claude.estimated_tokens") == 50, "claude tokens")
	assert(asFloat(claude["duration_ms"], "claude.duration_ms") == 400, "claude duration")
}

func readJSON(path string) map[string]any {
	value := readJSONAny(path)
	return asObject(value, path)
}

func readJSONAny(path string) any {
	bytes, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	var value any
	if err := json.Unmarshal(bytes, &value); err != nil {
		fatalf("parse %s: %v", path, err)
	}
	return value
}

func readJSONLines(path string) []map[string]any {
	bytes, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	var result []map[string]any
	for _, line := range splitLines(string(bytes)) {
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			fatalf("parse %s line: %v", path, err)
		}
		result = append(result, item)
	}
	return result
}

func readText(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	return string(bytes)
}

func writeJSON(path string, value any) {
	bytes, err := json.Marshal(value)
	if err != nil {
		fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func taskMap(parent map[string]any, key string) map[string]map[string]any {
	items := asArray(parent["tasks"], "tasks")
	result := map[string]map[string]any{}
	for _, item := range items {
		obj := asObject(item, "task")
		result[asString(obj[key], key)] = obj
	}
	return result
}

func statNano(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		fatalf("stat %s: %v", path, err)
	}
	return info.ModTime().UnixNano()
}

func asObject(value any, label string) map[string]any {
	obj, ok := value.(map[string]any)
	assert(ok, "%s is not an object: %#v", label, value)
	return obj
}

func asArray(value any, label string) []any {
	items, ok := value.([]any)
	assert(ok, "%s is not an array: %#v", label, value)
	return items
}

func asString(value any, label string) string {
	text, ok := value.(string)
	assert(ok, "%s is not a string: %#v", label, value)
	return text
}

func asFloat(value any, label string) float64 {
	number, ok := value.(float64)
	assert(ok, "%s is not a JSON number: %#v", label, value)
	return number
}

func asStringArray(value any, label string) []string {
	items := asArray(value, label)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, asString(item, label))
	}
	return result
}

func isJSONNumber(value any) bool {
	_, ok := value.(float64)
	return ok
}

func assertStringArrayEqual(actual, expected []string, label string) {
	assert(reflect.DeepEqual(actual, expected), "%s mismatch: got %#v want %#v", label, actual, expected)
}

func assertStringArrayContains(items []string, expected string, label string) {
	for _, item := range items {
		if item == expected {
			return
		}
	}
	fatalf("%s missing %s in %#v", label, expected, items)
}

func assertContains(text, needle string) {
	assert(contains(text, needle), "expected text to contain %q", needle)
}

func assertNotContains(text, needle string) {
	assert(!contains(text, needle), "expected text not to contain %q", needle)
}

func assert(condition bool, format string, args ...any) {
	if condition {
		return
	}
	fatalf(format, args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "test-assert: "+format+"\n", args...)
	os.Exit(1)
}

func contains(text, needle string) bool {
	return len(needle) == 0 || (len(text) >= len(needle) && index(text, needle) >= 0)
}

func hasPrefix(text, prefix string) bool {
	return len(text) >= len(prefix) && text[:len(prefix)] == prefix
}

func index(text, needle string) int {
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			line := text[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
