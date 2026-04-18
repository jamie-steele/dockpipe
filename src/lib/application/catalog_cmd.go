package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

type catalogWorkflowRecord struct {
	WorkflowID  string `json:"workflow_id"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	IconPath    string `json:"icon_path,omitempty"`
	ConfigPath  string `json:"config_path,omitempty"`
}

type catalogListOutput struct {
	Workflows  []catalogWorkflowRecord `json:"workflows"`
	Resolvers  []string                `json:"resolvers"`
	Strategies []string                `json:"strategies"`
	Runtimes   []string                `json:"runtimes"`
}

func cmdCatalog(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: dockpipe catalog list [--workdir <path>] [--format json|text]

  Prints the launcher-facing DockPipe catalog. The launcher should consume this contract instead of
  scanning repo/package trees directly.`)
	}
	switch args[0] {
	case "list":
		return cmdCatalogList(args[1:])
	default:
		return fmt.Errorf("unknown catalog subcommand %q (try: list)", args[0])
	}
}

func cmdCatalogList(args []string) error {
	format := "text"
	workdir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return fmt.Errorf("--workdir requires a path")
			}
			workdir = args[i+1]
			i++
		case "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("--format requires json or text")
			}
			format = strings.ToLower(strings.TrimSpace(args[i+1]))
			i++
		case "--help", "-h":
			fmt.Print(`dockpipe catalog list [--workdir <path>] [--format json|text]

Print the DockPipe-owned workflow/resolver/runtime catalog for launchers and tools.
`)
			return nil
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown option %s", args[i])
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return err
	}
	out, err := buildCatalogListOutput(projectRoot, workdir)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case "text":
		return writeCatalogText(os.Stdout, out)
	default:
		return fmt.Errorf("unknown --format %q (use json or text)", format)
	}
}

func buildCatalogListOutput(projectRoot, workdir string) (catalogListOutput, error) {
	workflows, err := listCatalogWorkflows(projectRoot, workdir)
	if err != nil {
		return catalogListOutput{}, err
	}
	return catalogListOutput{
		Workflows:  workflows,
		Resolvers:  listCatalogResolvers(projectRoot, workdir),
		Strategies: listCatalogCoreCategoryNames(projectRoot, workdir, "strategies"),
		Runtimes:   listCatalogCoreCategoryNames(projectRoot, workdir, "runtimes"),
	}, nil
}

func listCatalogWorkflows(projectRoot, workdir string) ([]catalogWorkflowRecord, error) {
	names, err := infrastructure.ListWorkflowNamesInRepoRootAndPackages(projectRoot, workdir)
	if err != nil {
		return nil, err
	}
	out := make([]catalogWorkflowRecord, 0, len(names))
	for _, name := range names {
		cfgPath, err := infrastructure.ResolveWorkflowConfigPathWithWorkdir(projectRoot, workdir, name)
		if err != nil {
			continue
		}
		wf, err := infrastructure.LoadWorkflow(cfgPath)
		if err != nil {
			continue
		}
		display := strings.TrimSpace(wf.Name)
		if display == "" {
			display = name
		}
		out = append(out, catalogWorkflowRecord{
			WorkflowID:  name,
			DisplayName: display,
			Description: strings.TrimSpace(wf.Description),
			Category:    strings.TrimSpace(wf.Category),
			IconPath:    resolveCatalogWorkflowIcon(cfgPath, strings.TrimSpace(wf.Icon)),
			ConfigPath:  cfgPath,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkflowID < out[j].WorkflowID
	})
	return out, nil
}

func resolveCatalogWorkflowIcon(cfgPath, icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return ""
	}
	if filepath.IsAbs(icon) {
		return icon
	}
	return filepath.Clean(filepath.Join(filepath.Dir(cfgPath), icon))
}

func listCatalogResolvers(projectRoot, workdir string) []string {
	set := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		set[name] = struct{}{}
	}

	collectResolverDirs := func(root string) {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || d.Name() != "profile" {
				return nil
			}
			add(filepath.Base(filepath.Dir(path)))
			return nil
		})
	}

	for _, root := range infrastructure.ResolverCompileRootsCached(projectRoot) {
		collectResolverDirs(root)
	}
	collectDirectResolverConfigs(filepath.Join(infrastructure.CoreDir(projectRoot), "resolvers"), add)
	if localResolvers, err := infrastructure.PackagesResolversDir(workdir); err == nil {
		collectTarballLeafNames(localResolvers, "dockpipe-resolver-*.tar.gz", "dockpipe-resolver-", add)
	}
	if globalResolvers, err := infrastructure.GlobalPackagesResolversDir(); err == nil {
		collectTarballLeafNames(globalResolvers, "dockpipe-resolver-*.tar.gz", "dockpipe-resolver-", add)
	}
	return sortedCatalogSet(set)
}

func listCatalogCoreCategoryNames(projectRoot, workdir, category string) []string {
	set := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		set[name] = struct{}{}
	}
	collectCoreCategory(filepath.Join(infrastructure.CoreDir(projectRoot), category), add)
	if localCore, err := infrastructure.PackagesCoreDir(workdir); err == nil {
		collectCoreCategory(filepath.Join(localCore, category), add)
	}
	if globalCore, err := infrastructure.GlobalTemplatesCoreDir(); err == nil {
		collectCoreCategory(filepath.Join(globalCore, category), add)
	}
	return sortedCatalogSet(set)
}

func collectDirectResolverConfigs(root string, add func(string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if _, err := os.Stat(filepath.Join(root, name, "config.yml")); err == nil {
			add(name)
		}
	}
}

func collectCoreCategory(root string, add func(string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, ent := range entries {
		name := strings.TrimSpace(ent.Name())
		if name == "" || strings.EqualFold(name, "README.md") || strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		add(name)
	}
}

func collectTarballLeafNames(root, pattern, prefix string, add func(string)) {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return
	}
	for _, match := range matches {
		base := filepath.Base(match)
		if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, ".tar.gz") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".tar.gz")
		if idx := strings.LastIndex(name, "-"); idx > 0 {
			name = name[:idx]
		}
		add(name)
	}
}

func sortedCatalogSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func writeCatalogText(f *os.File, out catalogListOutput) error {
	for _, wf := range out.Workflows {
		fmt.Fprintf(f, "workflow\t%s\t%s\t%s\t%s\n", wf.WorkflowID, wf.DisplayName, wf.Category, wf.ConfigPath)
	}
	for _, name := range out.Resolvers {
		fmt.Fprintf(f, "resolver\t%s\n", name)
	}
	for _, name := range out.Strategies {
		fmt.Fprintf(f, "strategy\t%s\n", name)
	}
	for _, name := range out.Runtimes {
		fmt.Fprintf(f, "runtime\t%s\n", name)
	}
	return nil
}
