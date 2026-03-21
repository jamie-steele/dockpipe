package application

import (
	"path/filepath"
	"strings"
)

// translateBridgeArgv rewrites path-like flag values and subcommand args from Windows
// to WSL paths before exec'ing dockpipe inside Linux.
func translateBridgeArgv(distro string, argv []string) []string {
	before, after := splitArgvAtDoubleDash(argv)
	before = translateDockpipeArgs(distro, before)
	if after == nil {
		return before
	}
	return append(before, after...)
}

func splitArgvAtDoubleDash(argv []string) (before []string, after []string) {
	for i, a := range argv {
		if a == "--" {
			return argv[:i], argv[i:]
		}
	}
	return argv, nil
}

func translateDockpipeArgs(distro string, argv []string) []string {
	out := append([]string(nil), argv...)
	// Pass 1: long flags with path (or env) values
	for i := 0; i < len(out); {
		a := out[i]
		switch a {
		case "--data-dir", "--run", "--pre-script", "--act", "--action", "--workdir", "--work-path", "--bundle-out", "--env-file",
			"--isolate", "--template", "--image":
			if i+1 < len(out) {
				out[i+1] = maybeTranslateWinPath(distro, out[i+1])
			}
			i += 2
		case "--mount":
			if i+1 < len(out) {
				out[i+1] = translateMountSpec(distro, out[i+1])
			}
			i += 2
		case "--build":
			if i+1 < len(out) {
				out[i+1] = maybeTranslateWinPath(distro, out[i+1])
			}
			i += 2
		case "--env", "--var":
			if i+1 < len(out) {
				out[i+1] = translateEnvOrVarLine(distro, out[i+1])
			}
			i += 2
		default:
			i++
		}
	}
	// Pass 2: subcommand positionals (init / action init / pre init / template init)
	if len(out) == 0 {
		return out
	}
	switch out[0] {
	case "init":
		translateInitSubcommandArgs(distro, out)
	case "action", "pre":
		if len(out) >= 2 && (out[1] == "init" || out[1] == "create") {
			translateInitLikePositionals(distro, out, 2)
		}
	case "template":
		if len(out) >= 2 && (out[1] == "init" || out[1] == "create") {
			translateInitLikePositionals(distro, out, 2)
		}
	}
	return out
}

// translateInitSubcommandArgs maps Windows paths in dockpipe init --from <path> only.
// The optional workflow name positional must not be rewritten (it is not a filesystem path).
func translateInitSubcommandArgs(distro string, out []string) {
	for i := 1; i < len(out); i++ {
		if out[i] == "--from" && i+1 < len(out) {
			i++
			out[i] = maybeTranslateWinPath(distro, out[i])
		}
	}
}

func translateInitLikePositionals(distro string, out []string, start int) {
	for i := start; i < len(out); i++ {
		if out[i] == "--from" {
			if i+1 < len(out) {
				i++
				out[i] = maybeTranslateWinPath(distro, out[i])
			}
			continue
		}
		if strings.HasPrefix(out[i], "-") {
			continue
		}
		if out[i] == "." {
			continue
		}
		out[i] = maybeTranslateWinPath(distro, out[i])
	}
}

func isURL(s string) bool {
	return strings.Contains(strings.TrimSpace(s), "://")
}

func isProbablyWindowsFilesystemPath(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[1] == ':' {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	return false
}

func maybeTranslateWinPath(distro, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return p
	}
	if isURL(p) {
		return p
	}
	slash := filepath.ToSlash(p)
	if strings.HasPrefix(slash, "/mnt/") {
		return slash
	}
	// Linux-absolute path (already for WSL); do not rewrite
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, `\\`) {
		return p
	}
	pp := p
	if !isProbablyWindowsFilesystemPath(p) && strings.Contains(p, `\`) {
		wd, err := windowsGetwdFn()
		if err != nil {
			return p
		}
		abs, err := filepath.Abs(filepath.Join(wd, p))
		if err != nil {
			return p
		}
		pp = abs
	}
	if !isProbablyWindowsFilesystemPath(pp) && !strings.HasPrefix(pp, `\\`) {
		return p
	}
	return winPathToWSL(distro, pp)
}

func translateEnvOrVarLine(distro, line string) string {
	key, val, ok := strings.Cut(line, "=")
	if !ok {
		return line
	}
	val = strings.TrimSpace(val)
	unq := strings.Trim(val, `"'`)
	t := maybeTranslateWinPath(distro, unq)
	if t == unq {
		return line
	}
	return key + "=" + t
}

// splitDockerMountHostContainer splits "HOST:CONTAINER" where CONTAINER is a Unix-style path.
// Scans from the right so Windows hosts like C:/Users/x:/work are not split at the first ":/".
func splitDockerMountHostContainer(val string) (host, container string) {
	for i := len(val) - 1; i >= 0; i-- {
		if val[i] != ':' {
			continue
		}
		r := val[i+1:]
		if len(r) == 0 {
			continue
		}
		if r[0] == '/' || strings.HasPrefix(r, "./") || strings.HasPrefix(r, "../") {
			return strings.TrimSpace(val[:i]), r
		}
	}
	return val, ""
}

// translateMountSpec maps host paths in -v style "HOST:CONTAINER" (and optional :ro / :rw suffix).
func translateMountSpec(distro, val string) string {
	val = strings.TrimSpace(val)
	orig := val
	modeSuffix := ""
	lower := strings.ToLower(val)
	for _, suf := range []string{":ro", ":rw", ":z", ":Z"} {
		if strings.HasSuffix(lower, suf) {
			modeSuffix = val[len(val)-len(suf):]
			val = val[:len(val)-len(suf)]
			lower = strings.ToLower(val)
			break
		}
	}
	host, cont := splitDockerMountHostContainer(val)
	if cont == "" {
		if shouldTranslateMountHost(val) {
			return maybeTranslateWinPath(distro, val) + modeSuffix
		}
		return orig
	}
	if shouldTranslateMountHost(host) {
		host = maybeTranslateWinPath(distro, host)
	}
	return host + ":" + cont + modeSuffix
}

func shouldTranslateMountHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	return isProbablyWindowsFilesystemPath(host) || strings.Contains(host, `\`)
}
