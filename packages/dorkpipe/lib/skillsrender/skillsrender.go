package skillsrender

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const rendererName = "dorkpipe-skills-render"

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type skillMeta struct {
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	ShortDescription string `yaml:"short_description"`
}

type skill struct {
	Name             string
	Description      string
	ShortDescription string
	Instructions     string
}

type config struct {
	Target string
	Output string
	DryRun bool
	Force  bool
	List   bool
	Skills string
}

type reportEntry struct {
	Name   string
	Status string
	Detail string
}

func Run(args []string, env map[string]string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, env, stderr)
	if err != nil {
		return err
	}
	assetsDir, err := resolveAssetsDir(env)
	if err != nil {
		return err
	}
	sourceDir := filepath.Join(assetsDir, "skills")
	skills, err := discoverSkills(sourceDir)
	if err != nil {
		return err
	}
	if cfg.List {
		for _, item := range skills {
			fmt.Fprintf(stdout, "%s\t%s\n", item.Name, item.Description)
		}
		return nil
	}
	selected, err := selectSkills(skills, cfg.Skills)
	if err != nil {
		return err
	}
	base, err := resolveOutputBase(cfg.Target, cfg.Output)
	if err != nil {
		return err
	}
	if !cfg.DryRun {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return err
		}
	}
	report, failures := renderAll(selected, base, cfg)
	fmt.Fprintln(stdout, "DorkPipe skills render report")
	fmt.Fprintf(stdout, "target: %s\n", cfg.Target)
	fmt.Fprintf(stdout, "output: %s\n", base)
	for _, entry := range report {
		fmt.Fprintf(stdout, "- %s: %s (%s)\n", entry.Status, entry.Name, entry.Detail)
	}
	if cfg.Target == "claude" && !cfg.DryRun {
		staleAgents := filepath.Join(base, "claude-agents.json")
		if _, err := os.Stat(staleAgents); err == nil {
			_ = os.Remove(staleAgents)
		}
	}
	if failures > 0 {
		return fmt.Errorf("skills render failed for %d skill(s)", failures)
	}
	return nil
}

func parseArgs(args []string, env map[string]string, stderr io.Writer) (config, error) {
	argv := append([]string(nil), args...)
	if len(argv) == 0 {
		if raw := strings.TrimSpace(env["DOCKPIPE_ARGS_JSON"]); raw != "" {
			var parsed []string
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				return config{}, fmt.Errorf("invalid DOCKPIPE_ARGS_JSON: %w", err)
			}
			argv = parsed
		}
	}
	var cfg config
	fs := flag.NewFlagSet("skills-render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.Target, "target", "generic", "")
	fs.StringVar(&cfg.Output, "output", "", "")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "")
	fs.BoolVar(&cfg.Force, "force", false, "")
	fs.BoolVar(&cfg.List, "list", false, "")
	fs.StringVar(&cfg.Skills, "skills", "", "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: skills-render [--target codex|claude|generic] [--output <path>] [--dry-run] [--force] [--list] [--skills <comma-separated-ids>]")
	}
	if err := fs.Parse(argv); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	switch cfg.Target {
	case "codex", "claude", "generic":
	default:
		return config{}, fmt.Errorf("unsupported target %q", cfg.Target)
	}
	return cfg, nil
}

func resolveAssetsDir(env map[string]string) (string, error) {
	if raw := strings.TrimSpace(env["DOCKPIPE_ASSETS_DIR"]); raw != "" {
		return filepath.Abs(raw)
	}
	return "", errors.New("DOCKPIPE_ASSETS_DIR is not set")
}

func discoverSkills(sourceDir string) ([]skill, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("missing skills source directory: %s", sourceDir)
	}
	var skills []skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item, err := readSkill(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		skills = append(skills, item)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func readSkill(skillDir string) (skill, error) {
	dirName := filepath.Base(skillDir)
	metaPath := filepath.Join(skillDir, "skill.yml")
	instructionsPath := filepath.Join(skillDir, "instructions.md")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return skill{}, fmt.Errorf("%s: missing skill.yml", dirName)
	}
	instructionsBytes, err := os.ReadFile(instructionsPath)
	if err != nil {
		return skill{}, fmt.Errorf("%s: missing instructions.md", dirName)
	}
	var meta skillMeta
	if err := yaml.Unmarshal(metaBytes, &meta); err != nil {
		return skill{}, fmt.Errorf("%s: parse skill.yml: %w", dirName, err)
	}
	item := skill{
		Name:             strings.TrimSpace(meta.Name),
		Description:      strings.TrimSpace(meta.Description),
		ShortDescription: strings.TrimSpace(meta.ShortDescription),
		Instructions:     strings.TrimSpace(string(instructionsBytes)),
	}
	if !nameRE.MatchString(item.Name) {
		return skill{}, fmt.Errorf("%s: invalid name %q", dirName, item.Name)
	}
	if item.Name != dirName {
		return skill{}, fmt.Errorf("%s: skill.yml name must match directory", dirName)
	}
	if item.Description == "" {
		return skill{}, fmt.Errorf("%s: missing description", item.Name)
	}
	if item.Instructions == "" {
		return skill{}, fmt.Errorf("%s: empty instructions", item.Name)
	}
	return item, nil
}

func selectSkills(skills []skill, raw string) ([]skill, error) {
	if strings.TrimSpace(raw) == "" {
		return skills, nil
	}
	selected := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			selected[part] = true
		}
	}
	var out []skill
	seen := map[string]bool{}
	for _, item := range skills {
		if selected[item.Name] {
			out = append(out, item)
			seen[item.Name] = true
		}
	}
	var unknown []string
	for name := range selected {
		if !seen[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown skills: %s", strings.Join(unknown, ", "))
	}
	return out, nil
}

func resolveOutputBase(target, output string) (string, error) {
	if strings.TrimSpace(output) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		switch target {
		case "codex":
			return filepath.Abs(filepath.Join(home, ".codex", "skills"))
		case "claude":
			return filepath.Abs(filepath.Join(home, ".claude", "skills"))
		default:
			return "", fmt.Errorf("--target %s requires --output <path>", target)
		}
	}
	expanded, err := expandPath(output)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}

func expandPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	raw = os.ExpandEnv(raw)
	if raw == "~" || strings.HasPrefix(raw, "~"+string(os.PathSeparator)) || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "~/"), "~"+string(os.PathSeparator))
		if trimmed == "" {
			return home, nil
		}
		return filepath.Join(home, trimmed), nil
	}
	return raw, nil
}

func renderAll(skills []skill, base string, cfg config) ([]reportEntry, int) {
	var report []reportEntry
	failures := 0
	for _, item := range skills {
		if err := renderSkill(item, base, cfg, &report); err != nil {
			failures++
			report = append(report, reportEntry{Name: item.Name, Status: "failed", Detail: err.Error()})
		}
	}
	return report, failures
}

func renderSkill(item skill, base string, cfg config, report *[]reportEntry) error {
	files, err := renderFiles(item, cfg.Target)
	if err != nil {
		return err
	}
	manifest := map[string]string{
		"renderer": rendererName,
		"source":   item.Name,
		"target":   cfg.Target,
		"sha256":   contentHash(files),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	files[".dorkpipe-skill-render.json"] = string(manifestBytes) + "\n"

	skillDir := filepath.Join(base, item.Name)
	if err := ensureWithinBase(base, skillDir); err != nil {
		return err
	}
	changed, err := changedExistingFiles(skillDir, files)
	if err != nil {
		return err
	}
	if len(changed) > 0 && !cfg.Force {
		*report = append(*report, reportEntry{
			Name:   item.Name,
			Status: "skipped",
			Detail: "existing user-modified files: " + strings.Join(changed, ", "),
		})
		return nil
	}
	if cfg.DryRun {
		*report = append(*report, reportEntry{Name: item.Name, Status: "would-render", Detail: skillDir})
		return nil
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	if cfg.Target == "claude" {
		if err := removeClaudeStaleFiles(skillDir); err != nil {
			return err
		}
	}
	for rel, text := range files {
		target := filepath.Join(skillDir, rel)
		if err := ensureWithinBase(base, target); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			return err
		}
	}
	*report = append(*report, reportEntry{Name: item.Name, Status: "rendered", Detail: skillDir})
	return nil
}

func renderFiles(item skill, target string) (map[string]string, error) {
	switch target {
	case "codex":
		short := item.ShortDescription
		if short == "" {
			short = item.Description
		}
		body := strings.Join([]string{
			"---",
			"name: " + item.Name,
			"description: " + singleLine(item.Description),
			"metadata:",
			"  short-description: " + singleLine(short),
			"---",
			"",
			strings.TrimRight(item.Instructions, "\n"),
			"",
		}, "\n")
		return map[string]string{"SKILL.md": body}, nil
	case "claude":
		body := strings.Join([]string{
			"---",
			"name: " + item.Name,
			"description: " + singleLine(item.Description),
			"---",
			"",
			strings.TrimRight(item.Instructions, "\n"),
			"",
		}, "\n")
		return map[string]string{"SKILL.md": body}, nil
	case "generic":
		meta := map[string]string{
			"name":              item.Name,
			"description":       item.Description,
			"short_description": item.ShortDescription,
		}
		metaBytes, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"skill.json":      string(metaBytes) + "\n",
			"instructions.md": strings.TrimRight(item.Instructions, "\n") + "\n",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported target %q", target)
	}
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func contentHash(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sum := sha256.New()
	for _, key := range keys {
		_, _ = sum.Write([]byte(key))
		_, _ = sum.Write([]byte{0})
		_, _ = sum.Write([]byte(files[key]))
		_, _ = sum.Write([]byte{0})
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}

func ensureWithinBase(base, target string) error {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to write outside output directory: %s", target)
	}
	return nil
}

func changedExistingFiles(skillDir string, files map[string]string) ([]string, error) {
	var changed []string
	for rel, desired := range files {
		path := filepath.Join(skillDir, rel)
		current, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if string(current) != desired {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

func removeClaudeStaleFiles(skillDir string) error {
	manifestPath := filepath.Join(skillDir, ".dorkpipe-skill-render.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil
	}
	if fmt.Sprint(manifest["renderer"]) != rendererName {
		return nil
	}
	for _, stale := range []string{"CLAUDE.md", "agent.json"} {
		path := filepath.Join(skillDir, stale)
		if _, err := os.Stat(path); err == nil {
			if removeErr := os.Remove(path); removeErr != nil {
				return removeErr
			}
		}
	}
	return nil
}
