package userinsight

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"dorkpipe.orchestrator/statepaths"
)

const (
	schemaVersion  = "1.0"
	timeLayoutUTC  = "2006-01-02T15:04:05Z"
	compactTimeUTC = "20060102T150405Z"
)

var (
	//go:embed rules.json
	rulesFS embed.FS

	severityCritical = regexp.MustCompile(`(?i)\bcritical\b|severity\s*[:=]?\s*1`)
	severityHigh     = regexp.MustCompile(`(?i)\bhigh\b|severity\s*[:=]?\s*2`)
	severityMedium   = regexp.MustCompile(`(?i)\bmedium\b|severity\s*[:=]?\s*3`)
	severityLow      = regexp.MustCompile(`(?i)\blow\b|severity\s*[:=]?\s*4`)

	exportCategories = []string{
		"risk",
		"constraint",
		"convention",
		"architecture_note",
		"compliance",
		"future_work",
		"unknown",
	}
)

type QueueDoc struct {
	SchemaVersion string      `json:"schema_version"`
	Kind          string      `json:"kind"`
	Items         []QueueItem `json:"items"`
}

type QueueItem struct {
	ID           string            `json:"id"`
	RawText      string            `json:"raw_text"`
	CategoryHint string            `json:"category_hint"`
	Source       string            `json:"source"`
	TimestampUTC string            `json:"timestamp_utc"`
	Scope        map[string]string `json:"scope"`
}

type InsightsDoc struct {
	SchemaVersion string            `json:"schema_version"`
	Kind          string            `json:"kind"`
	Separation    map[string]string `json:"separation"`
	Insights      []Insight         `json:"insights"`
	Provenance    *DocProvenance    `json:"provenance,omitempty"`
}

type Insight struct {
	ID             string              `json:"id"`
	QueueItemID    string              `json:"queue_item_id"`
	Category       string              `json:"category"`
	CategoryHint   string              `json:"category_hint"`
	NormalizedText string              `json:"normalized_text"`
	Intent         string              `json:"intent"`
	Severity       string              `json:"severity"`
	Scope          map[string]string   `json:"scope"`
	Provenance     InsightProvenance   `json:"provenance"`
	Confidence     InsightConfidence   `json:"confidence"`
	Status         string              `json:"status"`
	Supersedes     *string             `json:"supersedes"`
	Stale          bool                `json:"stale"`
	History        []InsightHistoryLog `json:"history"`
	RejectionCause string              `json:"rejection_reason,omitempty"`
}

type InsightProvenance struct {
	Source         string              `json:"source"`
	OriginalText   string              `json:"original_text"`
	CapturedAtUTC  string              `json:"captured_at_utc"`
	ProcessedAtUTC string              `json:"processed_at_utc"`
	Classifier     *ClassifierSnapshot `json:"classifier"`
}

type ClassifierSnapshot struct {
	Pattern  string `json:"pattern"`
	Category string `json:"category"`
}

type InsightConfidence struct {
	Role  string   `json:"role"`
	Score *float64 `json:"score"`
	Note  string   `json:"note"`
}

type InsightHistoryLog struct {
	AtUTC  string         `json:"at_utc"`
	Event  string         `json:"event"`
	Detail map[string]any `json:"detail"`
}

type DocProvenance struct {
	LastProcessedAtUTC string `json:"last_processed_at_utc"`
	Generator          string `json:"generator"`
	NewInsightsCount   int    `json:"new_insights_count"`
}

type ClassifierRule struct {
	Pattern     string `json:"pattern"`
	Category    string `json:"category"`
	ForceReview bool   `json:"force_review"`
}

type Rules struct {
	SchemaVersion            string           `json:"schema_version"`
	Description              string           `json:"description"`
	DefaultCategory          string           `json:"default_category"`
	AutoPromoteCategories    []string         `json:"auto_promote_categories"`
	ReviewRequiredCategories []string         `json:"review_required_categories"`
	Classifiers              []ClassifierRule `json:"classifiers"`
}

type ProcessResult struct {
	InsightsPath string
	NewCount     int
}

func Enqueue(workdir, raw, categoryHint, repoPath, component, workflow string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("usage: user-insight-enqueue.sh -m 'insight text' OR pipe text on stdin")
	}
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	if categoryHint == "" {
		categoryHint = "unknown"
	}
	now := time.Now().UTC()
	ts := now.Format(timeLayoutUTC)
	id := "ui-" + now.Format(compactTimeUTC) + "-" + shortHash(raw, ts, randomHex(4))

	queuePath := statepaths.QueuePath(root)
	historyPath := statepaths.AnalysisHistoryPath(root)

	doc, err := loadOrCreateQueue(queuePath)
	if err != nil {
		return "", err
	}
	item := QueueItem{
		ID:           id,
		RawText:      raw,
		CategoryHint: categoryHint,
		Source:       "user",
		TimestampUTC: ts,
		Scope:        compactScope(repoPath, component, workflow),
	}
	doc.Items = append(doc.Items, item)
	if err := writeJSON(queuePath, doc); err != nil {
		return "", err
	}
	if err := appendJSONLine(historyPath, map[string]any{
		"event":         "enqueue",
		"at_utc":        ts,
		"queue_item_id": id,
		"payload":       item,
	}); err != nil {
		return "", err
	}
	return id, nil
}

func Process(workdir string) (ProcessResult, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return ProcessResult{}, err
	}
	rules, err := loadRules()
	if err != nil {
		return ProcessResult{}, err
	}
	queuePath := statepaths.QueuePath(root)
	insightsPath := statepaths.InsightsPath(root)
	historyPath := statepaths.AnalysisHistoryPath(root)

	queueDoc, err := loadQueue(queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ProcessResult{}, fmt.Errorf("no queue at %s (run user-insight-enqueue.sh first)", queuePath)
		}
		return ProcessResult{}, err
	}
	doc, err := loadInsights(insightsPath, true)
	if err != nil {
		return ProcessResult{}, err
	}

	now := time.Now().UTC().Format(timeLayoutUTC)
	seen := map[string]struct{}{}
	for _, insight := range doc.Insights {
		if insight.QueueItemID != "" {
			seen[insight.QueueItemID] = struct{}{}
		}
	}

	newCount := 0
	for _, item := range queueDoc.Items {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		insight, err := buildInsight(item, rules, now)
		if err != nil {
			return ProcessResult{}, err
		}
		doc.Insights = append(doc.Insights, insight)
		newCount++
	}
	doc.Provenance = &DocProvenance{
		LastProcessedAtUTC: now,
		Generator:          "dockpipe-user-insight-process",
		NewInsightsCount:   newCount,
	}
	if err := writeJSON(insightsPath, doc); err != nil {
		return ProcessResult{}, err
	}
	if err := appendJSONLine(historyPath, map[string]any{
		"event":              "process",
		"at_utc":             now,
		"new_insights_count": newCount,
		"provenance":         doc.Provenance,
	}); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{InsightsPath: insightsPath, NewCount: newCount}, nil
}

func ExportByCategory(workdir string) (string, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	insightsPath := statepaths.InsightsPath(root)
	doc, err := loadInsights(insightsPath, false)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("missing %s", insightsPath)
		}
		return "", err
	}
	outDir := statepaths.InsightsByCategoryDir(root)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	for _, category := range exportCategories {
		items := make([]Insight, 0)
		for _, insight := range doc.Insights {
			if insight.Category == category {
				items = append(items, insight)
			}
		}
		if err := writeJSON(filepath.Join(outDir, category+".json"), items); err != nil {
			return "", err
		}
	}
	return outDir, nil
}

func MarkStale(workdir, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("usage: user-insight-mark-stale.sh <insight-or-queue-id>")
	}
	root, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	insightsPath := statepaths.InsightsPath(root)
	historyPath := statepaths.AnalysisHistoryPath(root)
	doc, err := loadInsights(insightsPath, false)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("usage: user-insight-mark-stale.sh <insight-or-queue-id>")
		}
		return err
	}
	now := time.Now().UTC().Format(timeLayoutUTC)
	for i := range doc.Insights {
		if !matchesInsightID(doc.Insights[i], id) {
			continue
		}
		doc.Insights[i].Stale = true
		doc.Insights[i].History = append(doc.Insights[i].History, InsightHistoryLog{
			AtUTC:  now,
			Event:  "mark_stale",
			Detail: map[string]any{},
		})
	}
	if err := writeJSON(insightsPath, doc); err != nil {
		return err
	}
	return appendJSONLine(historyPath, map[string]any{
		"event":               "mark_stale",
		"at_utc":              now,
		"insight_or_queue_id": id,
	})
}

func Review(workdir, action, id, reason string) (string, error) {
	if action != "accept" && action != "reject" {
		return "", fmt.Errorf("usage: user-insight-review.sh accept|reject <insight-or-queue-id> [--reason ...]")
	}
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("user-insight-review: missing insights file or id")
	}
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	insightsPath := statepaths.InsightsPath(root)
	historyPath := statepaths.AnalysisHistoryPath(root)
	doc, err := loadInsights(insightsPath, false)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("user-insight-review: missing insights file or id")
		}
		return "", err
	}
	status := "accepted"
	if action == "reject" {
		status = "rejected"
	}
	now := time.Now().UTC().Format(timeLayoutUTC)
	for i := range doc.Insights {
		if !matchesInsightID(doc.Insights[i], id) {
			continue
		}
		doc.Insights[i].Status = status
		if status == "rejected" && reason != "" {
			doc.Insights[i].RejectionCause = reason
		}
		doc.Insights[i].History = append(doc.Insights[i].History, InsightHistoryLog{
			AtUTC: now,
			Event: "review_" + status,
			Detail: map[string]any{
				"reason": reason,
			},
		})
	}
	if err := writeJSON(insightsPath, doc); err != nil {
		return "", err
	}
	if err := appendJSONLine(historyPath, map[string]any{
		"event":               "review_" + action,
		"at_utc":              now,
		"insight_or_queue_id": id,
		"status":              status,
		"reason":              reason,
	}); err != nil {
		return "", err
	}
	return status, nil
}

func Supersede(workdir, newID, oldID string) error {
	if strings.TrimSpace(newID) == "" || strings.TrimSpace(oldID) == "" {
		return fmt.Errorf("usage: user-insight-supersede.sh <new_insight_id> <old_insight_id>")
	}
	root, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	insightsPath := statepaths.InsightsPath(root)
	historyPath := statepaths.AnalysisHistoryPath(root)
	doc, err := loadInsights(insightsPath, false)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("usage: user-insight-supersede.sh <new_insight_id> <old_insight_id>")
		}
		return err
	}
	now := time.Now().UTC().Format(timeLayoutUTC)
	for i := range doc.Insights {
		switch {
		case matchesInsightID(doc.Insights[i], newID):
			old := oldID
			doc.Insights[i].Supersedes = &old
			doc.Insights[i].History = append(doc.Insights[i].History, InsightHistoryLog{
				AtUTC:  now,
				Event:  "supersede_link",
				Detail: map[string]any{"supersedes": oldID},
			})
		case matchesInsightID(doc.Insights[i], oldID):
			doc.Insights[i].Status = "superseded"
			doc.Insights[i].Stale = true
			doc.Insights[i].History = append(doc.Insights[i].History, InsightHistoryLog{
				AtUTC:  now,
				Event:  "superseded_by",
				Detail: map[string]any{"by": newID},
			})
		}
	}
	if err := writeJSON(insightsPath, doc); err != nil {
		return err
	}
	return appendJSONLine(historyPath, map[string]any{
		"event":  "supersede",
		"at_utc": now,
		"new_id": newID,
		"old_id": oldID,
	})
}

func loadOrCreateQueue(path string) (QueueDoc, error) {
	doc, err := loadQueue(path)
	if err == nil {
		return doc, nil
	}
	if !os.IsNotExist(err) {
		return QueueDoc{}, err
	}
	return QueueDoc{
		SchemaVersion: schemaVersion,
		Kind:          "dockpipe_user_insight_queue",
		Items:         []QueueItem{},
	}, nil
}

func loadQueue(path string) (QueueDoc, error) {
	var doc QueueDoc
	body, err := os.ReadFile(path)
	if err != nil {
		return QueueDoc{}, err
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return QueueDoc{}, err
	}
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = schemaVersion
	}
	if doc.Kind == "" {
		doc.Kind = "dockpipe_user_insight_queue"
	}
	if doc.Items == nil {
		doc.Items = []QueueItem{}
	}
	return doc, nil
}

func loadInsights(path string, allowMissing bool) (InsightsDoc, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return defaultInsightsDoc(), nil
		}
		return InsightsDoc{}, err
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed == "null" {
		return defaultInsightsDoc(), nil
	}
	var doc InsightsDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return InsightsDoc{}, err
	}
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = schemaVersion
	}
	if doc.Kind == "" {
		doc.Kind = "dockpipe_user_insights"
	}
	if doc.Separation == nil {
		doc.Separation = defaultInsightsDoc().Separation
	}
	if doc.Insights == nil {
		doc.Insights = []Insight{}
	}
	return doc, nil
}

func defaultInsightsDoc() InsightsDoc {
	return InsightsDoc{
		SchemaVersion: schemaVersion,
		Kind:          "dockpipe_user_insights",
		Separation: map[string]string{
			"user_insights":   "This file — structured human guidance; not verified facts.",
			"repo_facts":      "bin/.dockpipe/packages/dorkpipe/self-analysis/ (example layout)",
			"system_findings": "bin/.dockpipe/ci-analysis/findings.json",
		},
		Insights: []Insight{},
	}
}

func loadRules() (Rules, error) {
	body, err := rulesFS.ReadFile("rules.json")
	if err != nil {
		return Rules{}, err
	}
	var rules Rules
	if err := json.Unmarshal(body, &rules); err != nil {
		return Rules{}, err
	}
	return rules, nil
}

func buildInsight(item QueueItem, rules Rules, now string) (Insight, error) {
	match, err := firstClassifier(item.RawText, rules)
	if err != nil {
		return Insight{}, err
	}
	category := rules.DefaultCategory
	if match != nil {
		category = match.Category
	}
	status := "pending"
	if match != nil && match.ForceReview {
		status = "pending"
	} else if slices.Contains(rules.ReviewRequiredCategories, category) {
		status = "pending"
	} else if slices.Contains(rules.AutoPromoteCategories, category) {
		status = "accepted"
	}

	var classifier *ClassifierSnapshot
	if match != nil {
		classifier = &ClassifierSnapshot{Pattern: match.Pattern, Category: match.Category}
	}

	return Insight{
		ID:             "insight-" + item.ID,
		QueueItemID:    item.ID,
		Category:       category,
		CategoryHint:   item.CategoryHint,
		NormalizedText: normalizeText(item.RawText),
		Intent:         "user_guidance_signal",
		Severity:       severityFor(item.RawText),
		Scope:          item.Scope,
		Provenance: InsightProvenance{
			Source:         item.Source,
			OriginalText:   item.RawText,
			CapturedAtUTC:  item.TimestampUTC,
			ProcessedAtUTC: now,
			Classifier:     classifier,
		},
		Confidence: InsightConfidence{
			Role:  "user_signal",
			Score: nil,
			Note:  "Not verified by automated scan; not authoritative truth.",
		},
		Status:     status,
		Supersedes: nil,
		Stale:      false,
		History:    []InsightHistoryLog{},
	}, nil
}

func firstClassifier(text string, rules Rules) (*ClassifierRule, error) {
	for i := range rules.Classifiers {
		re, err := regexp.Compile(rules.Classifiers[i].Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile classifier %q: %w", rules.Classifiers[i].Pattern, err)
		}
		if re.MatchString(text) {
			return &rules.Classifiers[i], nil
		}
	}
	return nil, nil
}

func severityFor(text string) string {
	switch {
	case severityCritical.MatchString(text):
		return "critical"
	case severityHigh.MatchString(text):
		return "high"
	case severityMedium.MatchString(text):
		return "medium"
	case severityLow.MatchString(text):
		return "low"
	default:
		return "unspecified"
	}
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func compactScope(repoPath, component, workflow string) map[string]string {
	scope := map[string]string{}
	if repoPath != "" {
		scope["repo_path"] = repoPath
	}
	if component != "" {
		scope["component"] = component
	}
	if workflow != "" {
		scope["workflow"] = workflow
	}
	if len(scope) == 0 {
		return nil
	}
	return scope
}

func shortHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])[:14]
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func matchesInsightID(insight Insight, id string) bool {
	if id == insight.ID || id == insight.QueueItemID {
		return true
	}
	if "insight-"+id == insight.ID {
		return true
	}
	return strings.TrimPrefix(id, "insight-") == insight.QueueItemID
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func appendJSONLine(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(body)
	return err
}
