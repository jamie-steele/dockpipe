package infrastructure

import (
	"os"
	"path/filepath"
	"sort"
)

// ListWorkflowNamesInRepoRoot returns names of templates/<name>/ that contain config.yml under repoRoot.
func ListWorkflowNamesInRepoRoot(repoRoot string) ([]string, error) {
	dir := filepath.Join(repoRoot, "templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		cfg := filepath.Join(dir, name, "config.yml")
		if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}
