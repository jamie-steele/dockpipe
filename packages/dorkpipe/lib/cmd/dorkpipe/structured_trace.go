package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const editArtifactVersion = "v2"

type editStructuredTarget struct {
	Kind        string `json:"kind,omitempty"`
	SymbolName  string `json:"symbol_name,omitempty"`
	SymbolKind  string `json:"symbol_kind,omitempty"`
	Container   string `json:"container,omitempty"`
	Description string `json:"description,omitempty"`
}

type editStructuredRange struct {
	StartLine    int `json:"start_line,omitempty"`
	OldLineCount int `json:"old_line_count,omitempty"`
	NewLineCount int `json:"new_line_count,omitempty"`
}

type editStructuredEdit struct {
	ID             string                `json:"id,omitempty"`
	Op             string                `json:"op"`
	Source         string                `json:"source,omitempty"`
	Language       string                `json:"language,omitempty"`
	TargetFile     string                `json:"target_file"`
	Description    string                `json:"description,omitempty"`
	Target         *editStructuredTarget `json:"target,omitempty"`
	Range          *editStructuredRange  `json:"range,omitempty"`
	OldText        string                `json:"old_text,omitempty"`
	NewText        string                `json:"new_text,omitempty"`
	Content        string                `json:"content,omitempty"`
	Preconditions  []string              `json:"preconditions,omitempty"`
	Postconditions []string              `json:"postconditions,omitempty"`
	FallbackNotes  []string              `json:"fallback_notes,omitempty"`
}

type editTraceEvent struct {
	ContractVersion string         `json:"contract_version"`
	ArtifactVersion string         `json:"artifact_version"`
	RequestID       string         `json:"request_id"`
	ParentRequestID string         `json:"parent_request_id,omitempty"`
	Phase           string         `json:"phase,omitempty"`
	EventType       string         `json:"event_type,omitempty"`
	Label           string         `json:"label,omitempty"`
	Status          string         `json:"status,omitempty"`
	Progress        float64        `json:"progress,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ArtifactDir     string         `json:"artifact_dir,omitempty"`
	Timestamp       string         `json:"timestamp"`
}

type activeTraceRecorder struct {
	root            string
	artifactDir     string
	tracePath       string
	phase           string
	parentRequestID string
}

type unifiedDiffFile struct {
	oldPath string
	newPath string
	hunks   []unifiedDiffHunk
}

type unifiedDiffHunk struct {
	startOld int
	countOld int
	startNew int
	countNew int
	lines    []string
}

var currentTraceRecorder *activeTraceRecorder

func beginArtifactTrace(root, artifactDir, phase, parentRequestID string) {
	if strings.TrimSpace(artifactDir) == "" {
		currentTraceRecorder = nil
		return
	}
	currentTraceRecorder = &activeTraceRecorder{
		root:            root,
		artifactDir:     artifactDir,
		tracePath:       filepath.Join(artifactDir, "trace.jsonl"),
		phase:           phase,
		parentRequestID: strings.TrimSpace(parentRequestID),
	}
}

func endArtifactTrace() {
	currentTraceRecorder = nil
}

func recordTraceEvent(ev editEvent) {
	if currentTraceRecorder == nil {
		return
	}
	label := strings.TrimSpace(ev.DisplayText)
	if label == "" {
		label = strings.TrimSpace(ev.UserMessage)
	}
	if label == "" && ev.Error != nil {
		label = strings.TrimSpace(ev.Error.UserMessage)
	}
	traceEvent := editTraceEvent{
		ContractVersion: ev.ContractVersion,
		ArtifactVersion: editArtifactVersion,
		RequestID:       ev.RequestID,
		ParentRequestID: currentTraceRecorder.parentRequestID,
		Phase:           currentTraceRecorder.phase,
		EventType:       ev.Type,
		Label:           label,
		Status:          ev.Status,
		Progress:        ev.Progress,
		Metadata:        cloneAnyMap(ev.Metadata),
		ArtifactDir:     relativeTo(currentTraceRecorder.root, currentTraceRecorder.artifactDir),
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	appendTraceRecord(currentTraceRecorder.tracePath, traceEvent)
}

func appendTraceRecord(path string, value editTraceEvent) {
	if strings.TrimSpace(path) == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(value)
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func prepareArtifactForStorage(root string, artifact *editModelArtifact) *editModelArtifact {
	if artifact == nil {
		return nil
	}
	prepared := cloneEditArtifact(artifact)
	prepared.ArtifactVersion = editArtifactVersion
	prepared.Patch = normalizeGeneratedPatch(prepared.Patch)
	if len(prepared.StructuredEdits) == 0 {
		prepared.StructuredEdits = deriveStructuredEdits(root, prepared)
	}
	return prepared
}

func cloneEditArtifact(artifact *editModelArtifact) *editModelArtifact {
	if artifact == nil {
		return nil
	}
	cloned := &editModelArtifact{
		ArtifactVersion: strings.TrimSpace(artifact.ArtifactVersion),
		Summary:         artifact.Summary,
		TargetFiles:     append([]string{}, artifact.TargetFiles...),
		Patch:           artifact.Patch,
		Validations:     append([]string{}, artifact.Validations...),
	}
	if artifact.HelperScript != nil {
		helper := *artifact.HelperScript
		cloned.HelperScript = &helper
	}
	if len(artifact.CreatedFiles) > 0 {
		cloned.CreatedFiles = make(map[string]string, len(artifact.CreatedFiles))
		for key, value := range artifact.CreatedFiles {
			cloned.CreatedFiles[key] = value
		}
	}
	if len(artifact.StructuredEdits) > 0 {
		cloned.StructuredEdits = append([]editStructuredEdit{}, artifact.StructuredEdits...)
	}
	return cloned
}

func writePreparedArtifactBundle(root, artifactsDir string, artifact *editModelArtifact, verifyText string) (*editModelArtifact, string, error) {
	prepared := prepareArtifactForStorage(root, artifact)
	if err := validateEditArtifact(prepared); err != nil {
		return nil, "", err
	}
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), prepared)
	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(prepared.Patch), 0o644); err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(verifyText) != "" {
		_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyText), 0o644)
	}
	return prepared, patchPath, nil
}

func deriveStructuredEdits(root string, artifact *editModelArtifact) []editStructuredEdit {
	if artifact == nil {
		return nil
	}
	var edits []editStructuredEdit
	if len(artifact.CreatedFiles) > 0 {
		keys := make([]string, 0, len(artifact.CreatedFiles))
		for rel := range artifact.CreatedFiles {
			keys = append(keys, rel)
		}
		sort.Strings(keys)
		for _, rel := range keys {
			edits = append(edits, editStructuredEdit{
				ID:          makeStructuredEditID(rel, "create_file", len(edits)+1),
				Op:          "create_file",
				Source:      "deterministic",
				Language:    inferStructuredLanguage(rel),
				TargetFile:  rel,
				Description: fmt.Sprintf("Create %s.", rel),
				Content:     artifact.CreatedFiles[rel],
				Postconditions: []string{
					fmt.Sprintf("%s exists with the requested content.", rel),
				},
			})
		}
	}
	diffEdits := deriveStructuredEditsFromPatch(root, artifact.Patch)
	if len(diffEdits) == 0 {
		return edits
	}
	seen := map[string]bool{}
	for _, item := range edits {
		seen[item.TargetFile+"::"+item.Op] = true
	}
	for _, item := range diffEdits {
		key := item.TargetFile + "::" + item.Op
		if item.Op == "create_file" && seen[key] {
			continue
		}
		edits = append(edits, item)
	}
	return edits
}

func deriveStructuredEditsFromPatch(root, patch string) []editStructuredEdit {
	files := parseUnifiedDiffFiles(patch)
	if len(files) == 0 {
		return nil
	}
	var edits []editStructuredEdit
	for _, file := range files {
		targetFile := file.newPath
		if targetFile == "/dev/null" {
			targetFile = file.oldPath
		}
		if targetFile == "" {
			continue
		}
		language := inferStructuredLanguage(targetFile)
		switch {
		case file.oldPath == "/dev/null":
			content := ""
			for _, hunk := range file.hunks {
				content += newTextForHunk(hunk)
			}
			edits = append(edits, editStructuredEdit{
				ID:          makeStructuredEditID(targetFile, "create_file", len(edits)+1),
				Op:          "create_file",
				Source:      "patch-derived",
				Language:    language,
				TargetFile:  targetFile,
				Description: fmt.Sprintf("Create %s from the prepared artifact.", targetFile),
				Content:     content,
				Postconditions: []string{
					fmt.Sprintf("%s exists with the prepared content.", targetFile),
				},
				FallbackNotes: []string{"Fall back to applying the prepared unified diff if direct creation is unsafe."},
			})
		case file.newPath == "/dev/null":
			edits = append(edits, editStructuredEdit{
				ID:          makeStructuredEditID(targetFile, "delete_file", len(edits)+1),
				Op:          "delete_file",
				Source:      "patch-derived",
				Language:    language,
				TargetFile:  targetFile,
				Description: fmt.Sprintf("Delete %s.", targetFile),
				Postconditions: []string{
					fmt.Sprintf("%s no longer exists.", targetFile),
				},
				FallbackNotes: []string{"Fall back to applying the prepared unified diff if direct deletion is unsafe."},
			})
		default:
			for _, hunk := range file.hunks {
				oldText := oldTextForHunk(hunk)
				newText := newTextForHunk(hunk)
				target := detectStructuredTarget(root, targetFile, hunk.startOld)
				edits = append(edits, editStructuredEdit{
					ID:          makeStructuredEditID(targetFile, "replace_range", len(edits)+1),
					Op:          "replace_range",
					Source:      "patch-derived",
					Language:    language,
					TargetFile:  targetFile,
					Description: describeStructuredRangeEdit(targetFile, target, hunk),
					Target:      target,
					Range: &editStructuredRange{
						StartLine:    hunk.startOld,
						OldLineCount: hunk.countOld,
						NewLineCount: hunk.countNew,
					},
					OldText: oldText,
					NewText: newText,
					Preconditions: []string{
						"The expected pre-edit text is still present near the recorded line range.",
					},
					Postconditions: []string{
						"The file matches the prepared post-edit text for this range.",
					},
					FallbackNotes: []string{"Fall back to applying the prepared unified diff if the recorded range has drifted."},
				})
			}
		}
	}
	return edits
}

func parseUnifiedDiffFiles(patch string) []unifiedDiffFile {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	var files []unifiedDiffFile
	var current *unifiedDiffFile
	var currentHunk *unifiedDiffHunk
	flushHunk := func() {
		if current == nil || currentHunk == nil {
			return
		}
		current.hunks = append(current.hunks, *currentHunk)
		currentHunk = nil
	}
	flushFile := func() {
		if current == nil {
			return
		}
		flushHunk()
		if current.oldPath != "" || current.newPath != "" || len(current.hunks) > 0 {
			files = append(files, *current)
		}
		current = nil
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			oldPath, newPath := parseDiffGitPaths(line)
			current = &unifiedDiffFile{oldPath: oldPath, newPath: newPath}
		case current != nil && strings.HasPrefix(line, "--- "):
			current.oldPath = parseDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
		case current != nil && strings.HasPrefix(line, "+++ "):
			current.newPath = parseDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
		case current != nil && strings.HasPrefix(line, "@@ "):
			flushHunk()
			hunk, ok := parseUnifiedDiffHunk(line)
			if !ok {
				continue
			}
			currentHunk = &hunk
		default:
			if currentHunk != nil {
				currentHunk.lines = append(currentHunk.lines, line)
			}
		}
	}
	flushFile()
	return files
}

func parseDiffGitPaths(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return "", ""
	}
	return parseDiffPath(fields[2]), parseDiffPath(fields[3])
}

func parseDiffPath(raw string) string {
	switch {
	case raw == "/dev/null":
		return raw
	case strings.HasPrefix(raw, "a/"), strings.HasPrefix(raw, "b/"):
		return raw[2:]
	default:
		return raw
	}
}

var unifiedDiffHeaderPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func parseUnifiedDiffHunk(header string) (unifiedDiffHunk, bool) {
	match := unifiedDiffHeaderPattern.FindStringSubmatch(header)
	if len(match) == 0 {
		return unifiedDiffHunk{}, false
	}
	startOld, _ := strconv.Atoi(match[1])
	countOld := 1
	if match[2] != "" {
		countOld, _ = strconv.Atoi(match[2])
	}
	startNew, _ := strconv.Atoi(match[3])
	countNew := 1
	if match[4] != "" {
		countNew, _ = strconv.Atoi(match[4])
	}
	return unifiedDiffHunk{
		startOld: startOld,
		countOld: countOld,
		startNew: startNew,
		countNew: countNew,
	}, true
}

func oldTextForHunk(hunk unifiedDiffHunk) string {
	var lines []string
	for _, line := range hunk.lines {
		if strings.HasPrefix(line, `\ No newline at end of file`) {
			continue
		}
		if strings.HasPrefix(line, "+") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-") {
			lines = append(lines, line[1:])
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func newTextForHunk(hunk unifiedDiffHunk) string {
	var lines []string
	for _, line := range hunk.lines {
		if strings.HasPrefix(line, `\ No newline at end of file`) {
			continue
		}
		if strings.HasPrefix(line, "-") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "+") {
			lines = append(lines, line[1:])
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func inferStructuredLanguage(relPath string) string {
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".json":
		return "json"
	case ".yml", ".yaml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return ""
	}
}

func detectStructuredTarget(root, relPath string, startLine int) *editStructuredTarget {
	language := inferStructuredLanguage(relPath)
	if language != "javascript" && language != "typescript" {
		return &editStructuredTarget{Kind: "range"}
	}
	abs := filepath.Join(root, relPath)
	body, err := os.ReadFile(abs)
	if err != nil {
		return &editStructuredTarget{Kind: "range"}
	}
	lines := strings.Split(string(body), "\n")
	idx := startLine - 1
	if idx < 0 {
		idx = 0
	}
	if idx < len(lines) {
		for i := 0; i <= idx && i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "export {") {
				return &editStructuredTarget{
					Kind:        "import_block",
					Description: "Top-level import/export block",
				}
			}
			break
		}
	}
	functionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z0-9_$]+)\s*\(`),
		regexp.MustCompile(`^\s*(?:export\s+)?const\s+([A-Za-z0-9_$]+)\s*=\s*(?:async\s*)?\(`),
		regexp.MustCompile(`^\s*(?:export\s+)?class\s+([A-Za-z0-9_$]+)\b`),
	}
	for i := minStructuredInt(idx, len(lines)-1); i >= 0 && i >= idx-120; i-- {
		line := lines[i]
		for _, pattern := range functionPatterns {
			match := pattern.FindStringSubmatch(line)
			if len(match) > 1 {
				kind := "function"
				if strings.Contains(line, "class ") {
					kind = "class"
				}
				return &editStructuredTarget{
					Kind:       kind,
					SymbolKind: kind,
					SymbolName: match[1],
				}
			}
		}
	}
	return &editStructuredTarget{Kind: "range"}
}

func describeStructuredRangeEdit(targetFile string, target *editStructuredTarget, hunk unifiedDiffHunk) string {
	if target != nil && target.SymbolName != "" {
		return fmt.Sprintf("Update %s `%s` in %s.", emptyFallback(target.SymbolKind, target.Kind), target.SymbolName, targetFile)
	}
	if target != nil && target.Kind == "import_block" {
		return fmt.Sprintf("Update the import block in %s.", targetFile)
	}
	if hunk.countOld == 0 {
		return fmt.Sprintf("Insert content into %s near line %d.", targetFile, maxInt(hunk.startOld, 1))
	}
	return fmt.Sprintf("Replace a tracked range in %s starting near line %d.", targetFile, maxInt(hunk.startOld, 1))
}

func makeStructuredEditID(targetFile, op string, ordinal int) string {
	base := strings.NewReplacer("/", "-", ".", "-", " ", "-").Replace(strings.TrimSpace(targetFile))
	if base == "" {
		base = "artifact"
	}
	return fmt.Sprintf("%s-%s-%d", op, base, ordinal)
}

func validateStructuredEdits(edits []editStructuredEdit) error {
	for _, item := range edits {
		if strings.TrimSpace(item.Op) == "" {
			return fmt.Errorf("structured edit op is empty")
		}
		if strings.TrimSpace(item.TargetFile) == "" {
			return fmt.Errorf("structured edit target_file is empty")
		}
		if strings.HasPrefix(item.TargetFile, "/") || strings.Contains(item.TargetFile, "..") {
			return fmt.Errorf("unsafe structured edit target %q", item.TargetFile)
		}
		switch item.Op {
		case "replace_range":
			if item.Range == nil {
				return fmt.Errorf("replace_range for %s is missing range metadata", item.TargetFile)
			}
			if item.Range.StartLine < 0 {
				return fmt.Errorf("replace_range for %s has invalid start line", item.TargetFile)
			}
			if strings.TrimSpace(item.OldText) == "" || strings.TrimSpace(item.NewText) == "" {
				return fmt.Errorf("replace_range for %s must include concrete old_text and new_text", item.TargetFile)
			}
			if looksLikePlaceholderStructuredText(item.OldText) || looksLikePlaceholderStructuredText(item.NewText) {
				return fmt.Errorf("replace_range for %s contains placeholder text", item.TargetFile)
			}
		case "create_file":
			if item.Content == "" && item.NewText == "" {
				return fmt.Errorf("create_file for %s is missing content", item.TargetFile)
			}
			if looksLikePlaceholderStructuredText(item.Content) || looksLikePlaceholderStructuredText(item.NewText) {
				return fmt.Errorf("create_file for %s contains placeholder text", item.TargetFile)
			}
		case "delete_file":
		default:
			return fmt.Errorf("unsupported structured edit op %q", item.Op)
		}
	}
	return nil
}

func looksLikePlaceholderStructuredText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	for _, placeholder := range []string{
		"before text",
		"after text",
		"old text",
		"new text",
		"replace this",
		"updated text here",
	} {
		if normalized == placeholder || normalized == placeholder+`\n` {
			return true
		}
	}
	return false
}

func applyStructuredEdits(root string, artifact *editModelArtifact) (string, error) {
	if artifact == nil || len(artifact.StructuredEdits) == 0 {
		return "", fmt.Errorf("no structured edits are available")
	}
	grouped := map[string][]editStructuredEdit{}
	var createOps []editStructuredEdit
	var deleteOps []editStructuredEdit
	for _, item := range artifact.StructuredEdits {
		switch item.Op {
		case "create_file":
			createOps = append(createOps, item)
		case "delete_file":
			deleteOps = append(deleteOps, item)
		case "replace_range":
			grouped[item.TargetFile] = append(grouped[item.TargetFile], item)
		}
	}
	applied := []string{}
	for _, item := range createOps {
		content := item.Content
		if content == "" {
			content = item.NewText
		}
		abs := filepath.Join(root, item.TargetFile)
		if _, err := os.Stat(abs); err == nil {
			return "", fmt.Errorf("structured create target already exists: %s", item.TargetFile)
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return "", err
		}
		applied = append(applied, fmt.Sprintf("created %s", item.TargetFile))
	}
	targetFiles := make([]string, 0, len(grouped))
	for rel := range grouped {
		targetFiles = append(targetFiles, rel)
	}
	sort.Strings(targetFiles)
	for _, rel := range targetFiles {
		abs := filepath.Join(root, rel)
		body, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		text := string(body)
		ops := append([]editStructuredEdit{}, grouped[rel]...)
		sort.SliceStable(ops, func(i, j int) bool {
			left := ops[i].Range
			right := ops[j].Range
			if left == nil || right == nil {
				return i < j
			}
			if left.StartLine == right.StartLine {
				return left.OldLineCount > right.OldLineCount
			}
			return left.StartLine > right.StartLine
		})
		for _, item := range ops {
			next, err := applyReplaceRangeEdit(text, item)
			if err != nil {
				return "", err
			}
			text = next
		}
		if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
			return "", err
		}
		applied = append(applied, fmt.Sprintf("updated %s", rel))
	}
	for _, item := range deleteOps {
		abs := filepath.Join(root, item.TargetFile)
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		applied = append(applied, fmt.Sprintf("deleted %s", item.TargetFile))
	}
	if len(applied) == 0 {
		return "No structured edits were applied.", nil
	}
	return strings.Join(applied, "\n"), nil
}

func materializeStructuredPatch(root string, artifact *editModelArtifact) (string, error) {
	if artifact == nil || len(artifact.StructuredEdits) == 0 {
		return "", fmt.Errorf("no structured edits are available")
	}
	grouped := map[string][]editStructuredEdit{}
	createOps := map[string]editStructuredEdit{}
	deleteOps := map[string]editStructuredEdit{}
	for _, item := range artifact.StructuredEdits {
		switch item.Op {
		case "create_file":
			createOps[item.TargetFile] = item
		case "delete_file":
			deleteOps[item.TargetFile] = item
		case "replace_range":
			grouped[item.TargetFile] = append(grouped[item.TargetFile], item)
		}
	}
	parts := []string{}
	for _, rel := range sortedStructuredTargets(createOps) {
		item := createOps[rel]
		content := item.Content
		if content == "" {
			content = item.NewText
		}
		parts = append(parts, buildCreateFilePatch(rel, content))
	}
	for _, rel := range sortedGroupedTargets(grouped) {
		beforeBytes, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		after, err := applyStructuredEditsToText(string(beforeBytes), grouped[rel])
		if err != nil {
			return "", err
		}
		parts = append(parts, buildReplaceFilePatch(rel, string(beforeBytes), after))
	}
	for _, rel := range sortedStructuredTargets(deleteOps) {
		beforeBytes, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		parts = append(parts, buildDeleteFilePatch(rel, string(beforeBytes)))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no structured edits could be materialized into a patch")
	}
	return strings.TrimSpace(strings.Join(parts, "\n")) + "\n", nil
}

func applyReplaceRangeEdit(text string, item editStructuredEdit) (string, error) {
	if item.Range == nil {
		return "", fmt.Errorf("structured replace_range for %s is missing range metadata", item.TargetFile)
	}
	start, end, err := rangeOffsetsForText(text, item.Range.StartLine, item.Range.OldLineCount)
	if err != nil {
		return "", err
	}
	expected := item.OldText
	current := text[start:end]
	if expected != "" && current != expected {
		altStart, altEnd, found := findExactReplacementWindow(text, expected, item.Range.StartLine)
		if !found {
			return "", fmt.Errorf("structured replace_range precondition failed for %s near line %d", item.TargetFile, item.Range.StartLine)
		}
		start, end = altStart, altEnd
	}
	return text[:start] + item.NewText + text[end:], nil
}

func applyStructuredEditsToText(text string, ops []editStructuredEdit) (string, error) {
	sorted := append([]editStructuredEdit{}, ops...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := sorted[i].Range
		right := sorted[j].Range
		if left == nil || right == nil {
			return i < j
		}
		if left.StartLine == right.StartLine {
			return left.OldLineCount > right.OldLineCount
		}
		return left.StartLine > right.StartLine
	})
	current := text
	for _, item := range sorted {
		next, err := applyReplaceRangeEdit(current, item)
		if err != nil {
			return "", err
		}
		current = next
	}
	return current, nil
}

func rangeOffsetsForText(text string, startLine, oldLineCount int) (int, int, error) {
	if startLine < 0 {
		return 0, 0, fmt.Errorf("invalid start line %d", startLine)
	}
	if oldLineCount < 0 {
		return 0, 0, fmt.Errorf("invalid old line count %d", oldLineCount)
	}
	if oldLineCount == 0 {
		if startLine <= 0 {
			return 0, 0, nil
		}
		offset, err := lineOffsetForText(text, startLine+1)
		return offset, offset, err
	}
	start, err := lineOffsetForText(text, startLine)
	if err != nil {
		return 0, 0, err
	}
	end, err := lineOffsetForText(text, startLine+oldLineCount)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func lineOffsetForText(text string, lineNumber int) (int, error) {
	if lineNumber <= 1 {
		return 0, nil
	}
	line := 1
	for i, ch := range text {
		if ch == '\n' {
			line++
			if line == lineNumber {
				return i + 1, nil
			}
		}
	}
	if lineNumber == line+1 {
		return len(text), nil
	}
	return len(text), fmt.Errorf("line %d is outside the file", lineNumber)
}

func findExactReplacementWindow(text, expected string, startLine int) (int, int, bool) {
	if expected == "" {
		return 0, 0, false
	}
	expected = strings.ReplaceAll(expected, "\r\n", "\n")
	scanner := bufio.NewScanner(strings.NewReader(text))
	offsets := []int{0}
	currentOffset := 0
	for scanner.Scan() {
		currentOffset += len(scanner.Text()) + 1
		offsets = append(offsets, currentOffset)
	}
	indices := []int{}
	for start := 0; start <= len(text)-len(expected); {
		idx := strings.Index(text[start:], expected)
		if idx < 0 {
			break
		}
		absolute := start + idx
		indices = append(indices, absolute)
		start = absolute + 1
	}
	if len(indices) == 0 {
		return 0, 0, false
	}
	bestStart := indices[0]
	bestDistance := 1 << 30
	for _, actual := range indices {
		actualLine := 1
		for i := 0; i < len(offsets); i++ {
			if offsets[i] > actual {
				break
			}
			actualLine = i + 1
		}
		dist := actualLine - startLine
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDistance {
			bestDistance = dist
			bestStart = actual
		}
	}
	return bestStart, bestStart + len(expected), true
}

func uniqueStructuredEditOps(edits []editStructuredEdit) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range edits {
		op := strings.TrimSpace(item.Op)
		if op == "" || seen[op] {
			continue
		}
		seen[op] = true
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

func minStructuredInt(a, b int) int {
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

func sortedStructuredTargets(items map[string]editStructuredEdit) []string {
	out := make([]string, 0, len(items))
	for rel := range items {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func sortedGroupedTargets(items map[string][]editStructuredEdit) []string {
	out := make([]string, 0, len(items))
	for rel := range items {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}
