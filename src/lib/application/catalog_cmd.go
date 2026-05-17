package application

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/pipelang"
)

type catalogWorkflowRecord struct {
	WorkflowID  string                       `json:"workflow_id"`
	DisplayName string                       `json:"display_name,omitempty"`
	Description string                       `json:"description,omitempty"`
	Category    string                       `json:"category,omitempty"`
	IconPath    string                       `json:"icon_path,omitempty"`
	ConfigPath  string                       `json:"config_path,omitempty"`
	Vars        map[string]string            `json:"vars,omitempty"`
	Inputs      []catalogWorkflowInputRecord `json:"inputs,omitempty"`
	Types       []string                     `json:"types,omitempty"`
}

type catalogWorkflowInputRecord struct {
	FieldName    string            `json:"field_name"`
	EnvName      string            `json:"env_name"`
	Type         string            `json:"type,omitempty"`
	Description  string            `json:"description,omitempty"`
	DefaultValue string            `json:"default_value,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
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
			IconPath:    resolveCatalogWorkflowIcon(cfgPath, strings.TrimSpace(wf.Icon), name),
			ConfigPath:  cfgPath,
			Vars:        cloneCatalogVars(wf.Vars),
			Inputs:      buildCatalogWorkflowInputs(cfgPath, wf),
			Types:       append([]string(nil), wf.Types...),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkflowID < out[j].WorkflowID
	})
	return out, nil
}

var (
	pipeSummaryStartRe   = regexp.MustCompile(`^\s*///\s*<summary>\s*(.*?)\s*$`)
	pipeSummaryEndRe     = regexp.MustCompile(`^(.*?)\s*</summary>\s*$`)
	pipeAnnotationLineRe = regexp.MustCompile(`^\s*\[\s*[A-Za-z_][A-Za-z0-9_]*\s*=.*\]\s*$`)
	pipeFieldLineRe      = regexp.MustCompile(`^\s*public\s+([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s*(?:[;=].*)?$`)
)

func cloneCatalogVars(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type catalogPipeTypeShape struct {
	Annotations []pipelang.Annotation
	Fields      []pipelang.FieldSig
	ClassName   string
}

func buildCatalogWorkflowInputs(cfgPath string, wf *domain.Workflow) []catalogWorkflowInputRecord {
	if len(wf.Types) == 0 {
		return nil
	}
	moduleRoot := filepath.Dir(cfgPath)
	defaultsByClass := map[string]map[string]string{}
	seen := map[string]struct{}{}
	filesByRoot := map[string]map[string][]byte{}
	progByRoot := map[string]*pipelang.Program{}
	var out []catalogWorkflowInputRecord

	for _, raw := range wf.Types {
		filePath, typeRef, err := parseCatalogTypeSpec(moduleRoot, raw)
		if err != nil {
			continue
		}
		typeRoot := filepath.Dir(filePath)
		files, ok := filesByRoot[typeRoot]
		if !ok {
			files, _, err = readPipeFilesUnder(typeRoot)
			if err != nil || len(files) == 0 {
				continue
			}
			filesByRoot[typeRoot] = files
		}
		prog, ok := progByRoot[typeRoot]
		if !ok {
			prog, err = mergePipeLangProgram(files)
			if err != nil {
				continue
			}
			progByRoot[typeRoot] = prog
		}
		shape := findCatalogPipeTypeShape(prog, typeRef)
		if shape == nil {
			continue
		}
		fieldDocs := extractCatalogPipeFieldDocs(filePath)
		className := shape.ClassName
		if className == "" {
			className, err = inferEntryClassFromTypeRef(files, typeRef)
		}
		classDefaults := map[string]string{}
		if className != "" && err == nil {
			if cached, ok := defaultsByClass[className]; ok {
				classDefaults = cached
			} else {
				classDefaults = findCatalogClassDefaults(prog, className)
				defaultsByClass[className] = classDefaults
			}
		}
		envPrefix := catalogInferredEnvPrefix(shape.ClassName, typeRef)
		for _, field := range shape.Fields {
			envName := catalogFieldEnvName(field, envPrefix)
			if envName == "" {
				continue
			}
			key := strings.ToUpper(strings.TrimSpace(envName))
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			defaultValue := ""
			if wf.Vars != nil {
				if v, ok := wf.Vars[key]; ok {
					defaultValue = v
				}
			}
			if defaultValue == "" {
				defaultValue = classDefaults[field.Name]
			}
			out = append(out, catalogWorkflowInputRecord{
				FieldName:    field.Name,
				EnvName:      key,
				Type:         string(field.Type),
				Description:  strings.TrimSpace(fieldDocs[field.Name]),
				DefaultValue: defaultValue,
				Attributes:   catalogAnnotationMap(field.Annotations),
			})
		}
	}
	return out
}

func catalogFieldEnvName(field pipelang.FieldSig, prefix string) string {
	base := catalogFieldNameToEnv(field.Name)
	if base == "" {
		return ""
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return base
	}
	return prefix + base
}

func catalogInferredEnvPrefix(className, typeRef string) string {
	name := strings.TrimSpace(className)
	if name == "" {
		name = strings.TrimSpace(typeRef)
	}
	if strings.Contains(strings.ToLower(name), "vm") {
		return "DOCKPIPE_VM_"
	}
	return ""
}

func catalogAnnotationMap(in []pipelang.Annotation) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, ann := range in {
		key := strings.ToLower(strings.TrimSpace(ann.Name))
		if key == "" {
			continue
		}
		out[key] = ann.Value.StringValue()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergePipeLangProgram(files map[string][]byte) (*pipelang.Program, error) {
	merged := &pipelang.Program{}
	for name, b := range files {
		p, err := pipelang.Parse(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		merged.Interfaces = append(merged.Interfaces, p.Interfaces...)
		merged.Classes = append(merged.Classes, p.Classes...)
	}
	return merged, nil
}

func parseCatalogTypeSpec(moduleRoot, raw string) (string, string, error) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return "", "", fmt.Errorf("empty type spec")
	}
	left := spec
	typeRef := ""
	if i := strings.Index(spec, "<"); i >= 0 {
		j := strings.LastIndex(spec, ">")
		if j <= i+1 {
			return "", "", fmt.Errorf("invalid type spec %q", spec)
		}
		left = strings.TrimSpace(spec[:i])
		typeRef = strings.TrimSpace(spec[i+1 : j])
	}
	if filepath.Ext(left) == "" {
		left += ".pipe"
	}
	abs, err := filepath.Abs(filepath.Join(moduleRoot, filepath.FromSlash(left)))
	if err != nil {
		return "", "", err
	}
	if typeRef == "" {
		typeRef = strings.TrimSuffix(filepath.Base(left), filepath.Ext(left))
	}
	return abs, typeRef, nil
}

func findCatalogInterfaceDecl(prog *pipelang.Program, name string) *pipelang.InterfaceDecl {
	for _, decl := range prog.Interfaces {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findCatalogClassDecl(prog *pipelang.Program, name string) *pipelang.ClassDecl {
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findCatalogPipeTypeShape(prog *pipelang.Program, name string) *catalogPipeTypeShape {
	if decl := findCatalogInterfaceDecl(prog, name); decl != nil {
		return &catalogPipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      decl.Fields,
		}
	}
	if decl := findCatalogClassDecl(prog, name); decl != nil {
		fields := make([]pipelang.FieldSig, 0, len(decl.Fields))
		for _, field := range decl.Fields {
			fields = append(fields, pipelang.FieldSig{
				Visibility:  field.Visibility,
				Annotations: field.Annotations,
				Type:        field.Type,
				Name:        field.Name,
			})
		}
		return &catalogPipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      fields,
			ClassName:   decl.Name,
		}
	}
	return nil
}

func findCatalogClassDefaults(prog *pipelang.Program, className string) map[string]string {
	out := map[string]string{}
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Name) != strings.TrimSpace(className) {
			continue
		}
		for _, field := range decl.Fields {
			if lit, ok := field.Default.(*pipelang.LiteralExpr); ok {
				out[field.Name] = lit.Value.StringValue()
			}
		}
		break
	}
	return out
}

func extractCatalogPipeFieldDocs(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	var pending []string
	inSummary := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if inSummary {
			if m := pipeSummaryEndRe.FindStringSubmatch(line); len(m) == 2 {
				text := strings.TrimSpace(m[1])
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = false
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "///"))
			if text != "" {
				pending = append(pending, text)
			}
			continue
		}
		if m := pipeSummaryStartRe.FindStringSubmatch(line); len(m) == 2 {
			text := strings.TrimSpace(m[1])
			if strings.Contains(text, "</summary>") {
				text = strings.TrimSpace(strings.TrimSuffix(text, "</summary>"))
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = false
			} else {
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = true
			}
			continue
		}
		if m := pipeFieldLineRe.FindStringSubmatch(line); len(m) == 3 {
			fieldName := strings.TrimSpace(m[2])
			if fieldName != "" && len(pending) > 0 {
				out[fieldName] = strings.Join(pending, " ")
			}
			pending = nil
			continue
		}
		if pipeAnnotationLineRe.MatchString(line) {
			continue
		}
		if strings.TrimSpace(line) != "" {
			pending = nil
		}
	}
	return out
}

func catalogFieldNameToEnv(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	prevLowerOrDigit := false
	prevUpper := false
	for i, r := range name {
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if i > 0 && isUpper && (prevLowerOrDigit || (prevUpper && i+1 < len(name) && name[i+1] >= 'a' && name[i+1] <= 'z')) {
			b.WriteByte('_')
		} else if i > 0 && isDigit && !prevLowerOrDigit && !prevUpper {
			b.WriteByte('_')
		}
		if r == '-' || r == ' ' {
			b.WriteByte('_')
			prevLowerOrDigit = false
			prevUpper = false
			continue
		}
		if isLower {
			r = r - 'a' + 'A'
		}
		b.WriteRune(r)
		prevLowerOrDigit = isLower || isDigit
		prevUpper = isUpper
	}
	return b.String()
}

func resolveCatalogWorkflowIcon(cfgPath, icon, workflowID string) string {
	icon = strings.TrimSpace(icon)
	if icon != "" {
		if filepath.IsAbs(icon) {
			return icon
		}
		return filepath.Clean(filepath.Join(filepath.Dir(cfgPath), icon))
	}

	manifestPath := findNearestPackageManifest(cfgPath)
	if manifestPath == "" {
		return ""
	}
	pm, err := domain.ParsePackageManifest(manifestPath)
	if err != nil || pm == nil {
		return ""
	}
	manifestDir := filepath.Dir(manifestPath)
	if artwork := strings.TrimSpace(pm.Artwork[strings.TrimSpace(workflowID)]); artwork != "" {
		if filepath.IsAbs(artwork) {
			return artwork
		}
		return filepath.Clean(filepath.Join(manifestDir, artwork))
	}
	if icon := strings.TrimSpace(pm.Icon); icon != "" {
		if filepath.IsAbs(icon) {
			return icon
		}
		return filepath.Clean(filepath.Join(manifestDir, icon))
	}
	if artwork := strings.TrimSpace(pm.Artwork["icon"]); artwork != "" {
		if filepath.IsAbs(artwork) {
			return artwork
		}
		return filepath.Clean(filepath.Join(manifestDir, artwork))
	}
	return ""
}

func findNearestPackageManifest(cfgPath string) string {
	cur := filepath.Dir(filepath.Clean(cfgPath))
	for {
		manifest := filepath.Join(cur, infrastructure.PackageManifestFilename)
		if info, err := os.Stat(manifest); err == nil && !info.IsDir() {
			return manifest
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
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
