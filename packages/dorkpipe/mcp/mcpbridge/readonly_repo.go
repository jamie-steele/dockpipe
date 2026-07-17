package mcpbridge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	defaultRepoToolLimit    = 20
	maxRepoToolLimit        = 100
	defaultRepoReadMaxChars = 4000
	maxRepoReadMaxChars     = 20000
	maxRepoSearchFileBytes  = 256 * 1024
)

func repoListFiles(query string, limit int) ([]string, error) {
	root, err := effectiveRepoRoot()
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	query = strings.ToLower(strings.TrimSpace(query))
	limit = normalizeRepoToolLimit(limit)
	var out []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipRepoDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if query != "" && !strings.Contains(strings.ToLower(rel), query) {
			return nil
		}
		out = append(out, rel)
		if len(out) >= limit {
			return errRepoToolLimitReached
		}
		return nil
	})
	if err != nil && err != errRepoToolLimitReached {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func repoReadFile(path string, maxChars int) (string, error) {
	abs, err := ResolvePathUnderRepoRoot(path)
	if err != nil {
		return "", err
	}
	maxChars = normalizeRepoReadMaxChars(maxChars)
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(b)
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("file does not look like UTF-8 text")
	}
	return clampRepoText(text, maxChars), nil
}

func repoSearchText(query string, limit int) ([]string, error) {
	root, err := effectiveRepoRoot()
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	queryLower := strings.ToLower(query)
	limit = normalizeRepoToolLimit(limit)
	var out []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipRepoDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxRepoSearchFileBytes {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(b)
		if !utf8.ValidString(text) {
			return nil
		}
		lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		for idx, line := range lines {
			if strings.Contains(strings.ToLower(line), queryLower) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					break
				}
				out = append(out, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), idx+1, strings.TrimSpace(line)))
				if len(out) >= limit {
					return errRepoToolLimitReached
				}
			}
		}
		return nil
	})
	if err != nil && err != errRepoToolLimitReached {
		return nil, err
	}
	return out, nil
}

var errRepoToolLimitReached = fmt.Errorf("repo tool limit reached")

func shouldSkipRepoDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".next", ".turbo", "dist", "build", "target":
		return true
	default:
		return false
	}
}

func normalizeRepoToolLimit(limit int) int {
	if limit <= 0 {
		return defaultRepoToolLimit
	}
	if limit > maxRepoToolLimit {
		return maxRepoToolLimit
	}
	return limit
}

func normalizeRepoReadMaxChars(maxChars int) int {
	if maxChars <= 0 {
		return defaultRepoReadMaxChars
	}
	if maxChars > maxRepoReadMaxChars {
		return maxRepoReadMaxChars
	}
	return maxChars
}

func clampRepoText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n\n[truncated]"
}
