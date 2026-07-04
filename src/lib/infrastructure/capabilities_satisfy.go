package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

// SatisfiedCapabilityIDs returns dotted capability ids declared by resolver packages (package.yml capability:).
func SatisfiedCapabilityIDs(workdir, repoRoot string) map[string]struct{} {
	out := make(map[string]struct{})
	if pr, err := PackagesRoot(workdir); err == nil {
		mergeResolverCapabilitiesFromDir(filepath.Join(pr, "resolvers"), out)
	}
	if gr, err := GlobalPackagesRoot(); err == nil {
		mergeResolverCapabilitiesFromDir(filepath.Join(gr, "resolvers"), out)
	}
	for _, sr := range SystemPackagesResolversDirs() {
		mergeResolverCapabilitiesFromDir(sr, out)
	}
	cd := CoreDir(repoRoot)
	mergeResolverCapabilitiesFromDir(filepath.Join(cd, "resolvers"), out)
	return out
}

type yamlCapabilityOnly struct {
	Capability string `yaml:"capability"`
	Primitive  string `yaml:"primitive"`
}

func readResolverCapabilityFromPackageYML(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m yamlCapabilityOnly
	if err := yaml.Unmarshal(b, &m); err != nil {
		return "", err
	}
	c := strings.TrimSpace(m.Capability)
	if c == "" {
		c = strings.TrimSpace(m.Primitive)
	}
	return c, nil
}

func mergeResolverCapabilitiesFromDir(resolversDir string, out map[string]struct{}) {
	entries, err := os.ReadDir(resolversDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(resolversDir, e.Name(), PackageManifestFilename)
		c, err := readResolverCapabilityFromPackageYML(p)
		if err != nil || c == "" {
			continue
		}
		out[c] = struct{}{}
	}
}

// CheckWorkflowPackageRequiresCapabilities ensures sibling package.yml requires_capabilities are
// satisfied by installed resolver packages. Skips when the workflow is loaded from a tar URI,
// when package.yml is missing, or when requires_capabilities is empty.
func CheckWorkflowPackageRequiresCapabilities(workdir, repoRoot, wfRoot, wfConfigPath string) error {
	if strings.HasPrefix(strings.TrimSpace(wfConfigPath), "tar://") {
		return nil
	}
	pmPath := filepath.Join(wfRoot, PackageManifestFilename)
	b, err := os.ReadFile(pmPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var pm struct {
		RequiresCapabilities []string `yaml:"requires_capabilities"`
		RequiresPrimitives   []string `yaml:"requires_primitives"`
	}
	if err := yaml.Unmarshal(b, &pm); err != nil {
		return fmt.Errorf("package.yml: %w", err)
	}
	req := pm.RequiresCapabilities
	if len(req) == 0 {
		req = pm.RequiresPrimitives
	}
	if len(req) == 0 {
		return nil
	}
	sat := SatisfiedCapabilityIDs(workdir, repoRoot)
	var missing []string
	for _, r := range req {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := sat[r]; !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"workflow package requires capabilities not satisfied by installed resolver packages: %s — install resolvers that declare these in package.yml (see docs/concepts/capabilities.md)",
		strings.Join(missing, ", "),
	)
}

// CheckWorkflowResolverScriptDependencies ensures logical resolver-owned script ids like
// scripts/dorkpipe/... are backed by an explicit resolver declaration instead of silently
// resolving via any available workflow tarball/package.
func CheckWorkflowResolverScriptDependencies(workdir, repoRoot string, wf *domain.Workflow, wfRoot, wfConfigPath string) error {
	if wf == nil {
		return nil
	}
	required := requiredResolverScriptDomains(workdir, repoRoot, wf)
	if len(required) == 0 {
		return nil
	}
	declared, err := declaredWorkflowResolvers(repoRoot, workdir, wf, wfRoot, wfConfigPath)
	if err != nil {
		return err
	}
	var missing []string
	for domainName := range required {
		if declared[domainName] {
			continue
		}
		if !resolverProfileExists(repoRoot, workdir, domainName) {
			continue
		}
		missing = append(missing, domainName)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"logical resolver script paths require an explicit resolver dependency: %s — add package.yml requires_resolvers, inject.resolver, or workflow/step resolver/runtime selection",
		strings.Join(missing, ", "),
	)
}

func requiredResolverScriptDomains(workdir, repoRoot string, wf *domain.Workflow) map[string]bool {
	out := make(map[string]bool)
	add := func(rel string) {
		if domainName, ok := logicalResolverScriptDomain(workdir, repoRoot, rel); ok {
			out[domainName] = true
		}
	}
	for _, rel := range wf.Run {
		add(rel)
	}
	add(wf.Act)
	add(wf.Action)
	for _, step := range wf.Steps {
		for _, rel := range step.RunPaths() {
			add(rel)
		}
		add(step.ActPath())
	}
	return out
}

func logicalResolverScriptDomain(workdir, repoRoot, rel string) (string, bool) {
	rel = strings.TrimSpace(rel)
	if !strings.HasPrefix(rel, "scripts/") {
		return "", false
	}
	rest := strings.TrimPrefix(rel, "scripts/")
	first, after, ok := strings.Cut(rest, "/")
	if !ok || strings.TrimSpace(first) == "" || strings.TrimSpace(after) == "" {
		return "", false
	}
	if first == "core" {
		return "", false
	}
	projectRoot := strings.TrimSpace(workdir)
	if projectRoot == "" {
		projectRoot = strings.TrimSpace(repoRoot)
	}
	if root, err := domain.FindProjectRootWithDockpipeConfig(projectRoot); err == nil && strings.TrimSpace(root) != "" {
		projectRoot = root
	}
	if fileExists(filepath.Join(projectRoot, "scripts", filepath.FromSlash(rest))) {
		return "", false
	}
	if fileExists(filepath.Join(projectRoot, "src", "scripts", filepath.FromSlash(rest))) {
		return "", false
	}
	return first, true
}

func declaredWorkflowResolvers(repoRoot, workdir string, wf *domain.Workflow, wfRoot, wfConfigPath string) (map[string]bool, error) {
	out := make(map[string]bool)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		out[name] = true
	}
	add(wf.Resolver)
	add(wf.Runtime)
	add(wf.Isolate)
	for _, inj := range wf.Inject {
		add(inj.Resolver)
	}
	for _, step := range wf.Steps {
		add(step.Resolver)
		add(step.Runtime)
		add(step.Isolate)
	}
	pm, err := readWorkflowPackageManifestForDependencyChecks(wfRoot, wfConfigPath)
	if err != nil {
		return nil, err
	}
	if pm != nil {
		for _, name := range pm.RequiresResolvers {
			add(name)
		}
	}
	return out, nil
}

func readWorkflowPackageManifestForDependencyChecks(wfRoot, wfConfigPath string) (*domain.PackageManifest, error) {
	if strings.HasPrefix(strings.TrimSpace(wfConfigPath), "tar://") {
		tarPath, entry, ok := SplitTarWorkflowURI(wfConfigPath)
		if !ok {
			return nil, fmt.Errorf("invalid tar workflow URI")
		}
		pkgPath := filepath.ToSlash(filepath.Join(filepath.Dir(entry), PackageManifestFilename))
		b, err := packagebuild.ReadFileFromTarGz(tarPath, pkgPath)
		if err != nil {
			return nil, nil
		}
		var pm domain.PackageManifest
		if err := yaml.Unmarshal(b, &pm); err != nil {
			return nil, fmt.Errorf("%s: %w", pkgPath, err)
		}
		domain.NormalizePackageManifestYAMLAliases(&pm)
		if err := domain.ValidatePackageManifest(&pm); err != nil {
			return nil, fmt.Errorf("%s: %w", pkgPath, err)
		}
		return &pm, nil
	}
	pmPath := filepath.Join(wfRoot, PackageManifestFilename)
	if _, err := os.Stat(pmPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return domain.ParsePackageManifest(pmPath)
}

func resolverProfileExists(repoRoot, workdir, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if tgz, err := FindLatestResolverTarball(workdir, name); err == nil && strings.TrimSpace(tgz) != "" {
		return true
	}
	projectRoot := strings.TrimSpace(workdir)
	if projectRoot == "" {
		projectRoot = strings.TrimSpace(repoRoot)
	}
	if roots := ResolverCompileRootsCached(projectRoot); len(NestedResolverLeafDirs(name, roots)) > 0 {
		return true
	}
	if dirExists(filepath.Join(CoreDir(repoRoot), "resolvers", name)) {
		return true
	}
	if dirExists(filepath.Join(CoreDir(repoRoot), "resolvers", name, "profile")) {
		return true
	}
	if gr, err := GlobalPackagesResolversDir(); err == nil {
		if dirExists(filepath.Join(gr, name)) || dirExists(filepath.Join(gr, name, "profile")) {
			return true
		}
	}
	for _, sr := range SystemPackagesResolversDirs() {
		if dirExists(filepath.Join(sr, name)) || dirExists(filepath.Join(sr, name, "profile")) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
