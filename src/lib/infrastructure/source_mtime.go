package infrastructure

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// MaxModTimeFilesUnder returns the latest mod time of any regular file under root, walking recursively.
// Skips ".git" and ".dockpipe" directories. Returns zero time if no files are found.
func MaxModTimeFilesUnder(root string) (time.Time, error) {
	var max time.Time
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == ".dockpipe" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		found = true
		if info.ModTime().After(max) {
			max = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	if !found {
		return time.Time{}, nil
	}
	return max, nil
}

// SourceDirNewerThanPath returns true when the latest source file mod time under srcRoot is strictly
// after refPath's mod time (typically a compiled .tar.gz). Used to decide if a package compile is stale.
// If refPath is missing or srcRoot walk fails, returns true, nil (prefer rebuild).
func SourceDirNewerThanPath(srcRoot, refPath string) (bool, error) {
	refInfo, err := os.Stat(refPath)
	if err != nil {
		return true, nil
	}
	srcMax, err := MaxModTimeFilesUnder(srcRoot)
	if err != nil {
		return true, err
	}
	if srcMax.IsZero() {
		return true, nil
	}
	return srcMax.After(refInfo.ModTime()), nil
}

// PickLatestModTimePath returns the path with the newest os.Stat ModTime, or "" if paths is empty.
func PickLatestModTimePath(paths []string) string {
	var best string
	var bestT time.Time
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if best == "" || st.ModTime().After(bestT) {
			best = p
			bestT = st.ModTime()
		}
	}
	return best
}
