package application

import "strings"

const defaultPackageVersion = "0.0.0"

func authoredPackageVersion(workdir string) string {
	workdir = strings.TrimSpace(workdir)
	if workdir != "" {
		if v, err := readRepoVersion(workdir); err == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return defaultPackageVersion
}
