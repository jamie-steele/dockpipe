package application

import (
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/infrastructure"
	"golang.org/x/term"
)

type usageEntry struct {
	left  string
	right string
}

type usageSection struct {
	title   string
	intro   []string
	entries []usageEntry
}

var mainUsageSections = []usageSection{
	{
		intro: []string{
			"DockPipe — run anything, anywhere, in isolation.",
		},
	},
	{
		title: "Usage",
		intro: []string{
			"dockpipe [options] -- <command> [args...]",
			"dockpipe --workflow <name> [options] [--] [args for last step]",
		},
	},
	{
		title: "Core",
		entries: []usageEntry{
			{"--workflow <name>", "Workflow to run (e.g. test)"},
			{"--no-compile-deps", "Skip the default pre-run transitive compile (package compile for-workflow). Env: DOCKPIPE_COMPILE_DEPS=0"},
			{"--compile-deps", "No-op (kept for compatibility); transitive compile is default when env is unset"},
			{"--runtime <name>", "Runtime profile (e.g. docker)"},
			{"--resolver <name>", "Resolver profile (optional)"},
		},
	},
	{
		title: "Optional",
		entries: []usageEntry{
			{"--workdir <path>", "Host directory mounted at /work (default: cwd; omit when you run dockpipe from the project root)"},
			{"--isolate <name>", "Image or template for the container"},
			{"--", "End of dockpipe flags; rest goes to your command"},
		},
	},
	{
		title: "More flags",
		entries: []usageEntry{
			{"--workflow-file, --run, --act, --strategy, --repo, --branch, --mount", ""},
			{"--workflows-dir <path>", "Repo-relative or absolute root for named workflows (default: workflows/; env: DOCKPIPE_WORKFLOWS_DIR)"},
			{"--env, --env-file, --var, --no-op-inject", "Skip vault op inject; env: DOCKPIPE_OP_INJECT=0"},
			{"--tf <cmds>", "Terraform pipeline: set DOCKPIPE_TF_COMMANDS (e.g. plan, apply). Workflows that run terraform-pipeline.sh use it (e.g. dockpipe.cloudflare.r2infra, package-store-infra when set). Also: --tf-dry-run, --tf-no-auto-approve"},
			{"--data-dir, --data-vol, --no-data, --reinit, -f, -d/--detach", ""},
		},
	},
	{
		title: "Commands",
		entries: []usageEntry{
			{"init", "Add DockPipe to the current project"},
			{"install", "Fetch templates/core from HTTPS (e.g. Cloudflare R2); see install core --help"},
			{"clone <name>", "Copy a compiled workflow package to workflows/ when allow_clone is true (see package manifest)"},
			{"build", "Compile packages into bin/.dockpipe/internal (same as package compile all --force; replaces existing outputs)"},
			{"clean", "Remove compiled package store (bin/.dockpipe/internal/packages)"},
			{"rebuild", "clean then build"},
			{"package list|manifest|build|compile", "Packages: list, manifest, author core tarball, or compile into bin/.dockpipe/internal"},
			{"compile", "Same as dockpipe package compile (core, resolvers, workflows)"},
			{"release upload", "Upload a file to S3-compatible storage (self-hosted; uses aws CLI)"},
			{"workflow validate|list", "Validate YAML or print the DockPipe-resolved workflow catalog"},
			{"catalog list", "Print the DockPipe-owned launcher/tooling catalog (workflows, resolvers, strategies, runtimes)"},
			{"pipelang compile|invoke|materialize", "PipeLang typed authoring helpers"},
			{"doctor", "Check docker, bash, and bundled assets"},
			{"core script-path <dots>", "Print absolute path to a core asset (same as scripts/core.<dots> in YAML)"},
			{"terraform pipeline-path | terraform run <cmds>", "Terraform helpers (see dockpipe terraform --help)"},
			{"runs list [--workdir]", "List active host-run records under bin/.dockpipe/runs/"},
			{"windows setup|doctor", "Windows: optional WSL bridge setup"},
			{"action|pre|template init", "Copy sample scripts (use each with --help)"},
		},
	},
	{
		title: "Examples",
		intro: []string{
			"dockpipe init",
			"dockpipe --workflow test --runtime docker",
			"dockpipe --workflow test --runtime docker -- go test ./...",
		},
	},
	{
		intro: []string{
			"Requires: docker and bash on the host. Git for --repo and worktree-style flows.",
			"Workflows: YAML. See docs/cli-reference.md for every flag.",
		},
	},
}

var initUsageSections = []usageSection{
	{
		intro: []string{
			"dockpipe init",
			"Project setup in the current directory, or add a new workflow.",
		},
	},
	{
		title: "Usage",
		entries: []usageEntry{
			{"dockpipe init [flags]", "Create a blank project scaffold plus workflows/example/ when no DockPipe workflows exist yet"},
			{"dockpipe init <name> [flags]", "Create workflows/<name>/config.yml as an empty starter (see --workflows-dir)"},
			{"dockpipe init <name> --from <src>", "Copy a bundled template or filesystem path into workflows/<name>/"},
		},
	},
	{
		title: "Flags",
		entries: []usageEntry{
			{"--from <source>", "With <name>: copy from blank (same as default), init, run, run-apply, a path, ..."},
			{"--workflows-dir <path>", "With <name>: repo-relative or absolute directory for named workflows (default: workflows). Same as DOCKPIPE_WORKFLOWS_DIR for dockpipe run"},
			{"--runtime <name>", "Written into new config (with <name>)"},
			{"--resolver <name>", "Written into new config (with <name>)"},
			{"--strategy <name>", "Written into new config (with <name>)"},
			{"--gitignore", "Append a marked block to .gitignore at the git repo root (idempotent; requires a git working tree)"},
		},
	},
	{
		title: "Examples",
		intro: []string{
			"dockpipe init",
			"dockpipe init --gitignore",
			"dockpipe init my-pipeline",
			"dockpipe init my-pipeline --from run-apply --resolver codex --runtime docker",
			"dockpipe init my-starter --from init",
		},
	},
}

func printUsage() {
	fmt.Print(infrastructure.RenderBannerForTerminal(os.Stdout, os.Stderr))
	fmt.Print("\n")
	fmt.Print(renderUsageSections(mainUsageSections, usageWidth()))
}

func printInitUsage() {
	fmt.Print(renderUsageSections(initUsageSections, usageWidth()))
}

const runsUsageText = `dockpipe runs list — show host-run registry entries

While a skip_container workflow step runs a host script, dockpipe may write
workdir/bin/.dockpipe/runs/<id>.json (and optional sidecars). This command lists
those JSON files.

Usage:
  dockpipe runs list [--workdir <path>]

  --workdir   Project directory (default: DOCKPIPE_WORKDIR or current directory)

`

func usageWidth() int {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		if f == nil {
			continue
		}
		fd, ok := terminalFDForUsage(f)
		if !ok {
			continue
		}
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			return w
		}
	}
	return 100
}

func terminalFDForUsage(f *os.File) (int, bool) {
	switch f {
	case os.Stdout:
		return 1, true
	case os.Stderr:
		return 2, true
	default:
		return 0, false
	}
}

func renderUsageSections(sections []usageSection, width int) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		if section.title != "" {
			b.WriteString(section.title)
			b.WriteString(":\n")
		}
		for _, line := range section.intro {
			introIndent := ""
			if section.title != "" {
				introIndent = "  "
			}
			writeWrappedLine(&b, introIndent, line, width)
		}
		if len(section.intro) > 0 && len(section.entries) > 0 {
			b.WriteByte('\n')
		}
		if len(section.entries) > 0 {
			writeUsageEntries(&b, section.entries, width)
		}
	}
	return b.String()
}

func writeUsageEntries(b *strings.Builder, entries []usageEntry, width int) {
	leftWidth := longestLeft(entries)
	if leftWidth < 18 {
		leftWidth = 18
	}
	if leftWidth > 38 {
		leftWidth = 38
	}
	if width < 76 {
		leftWidth = 0
	}
	for _, entry := range entries {
		writeUsageEntry(b, entry, width, leftWidth)
	}
}

func longestLeft(entries []usageEntry) int {
	max := 0
	for _, entry := range entries {
		if n := len(entry.left); n > max {
			max = n
		}
	}
	return max
}

func writeUsageEntry(b *strings.Builder, entry usageEntry, width, leftWidth int) {
	baseIndent := "  "
	if entry.right == "" {
		writeWrappedLine(b, baseIndent, entry.left, width)
		return
	}
	if leftWidth == 0 {
		writeWrappedLine(b, baseIndent, entry.left, width)
		writeWrappedLine(b, "      ", entry.right, width)
		return
	}
	gap := "  "
	if len(entry.left) > leftWidth {
		writeWrappedLine(b, baseIndent, entry.left, width)
		writeWrappedLine(b, baseIndent+strings.Repeat(" ", leftWidth)+gap, entry.right, width)
		return
	}
	writeWrappedPair(b, baseIndent+padRight(entry.left, leftWidth)+gap, baseIndent+strings.Repeat(" ", leftWidth)+gap, entry.right, width)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func writeWrappedLine(b *strings.Builder, indent, text string, width int) {
	if text == "" {
		b.WriteByte('\n')
		return
	}
	if width <= len(indent)+20 {
		b.WriteString(indent)
		b.WriteString(text)
		b.WriteByte('\n')
		return
	}
	max := width - len(indent)
	words := strings.Fields(text)
	if len(words) == 0 {
		b.WriteString(indent)
		b.WriteByte('\n')
		return
	}
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > max {
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteByte('\n')
			line = word
			continue
		}
		line += " " + word
	}
	b.WriteString(indent)
	b.WriteString(line)
	b.WriteByte('\n')
}

func writeWrappedPair(b *strings.Builder, firstIndent, nextIndent, text string, width int) {
	if text == "" {
		b.WriteString(firstIndent)
		b.WriteByte('\n')
		return
	}
	if width <= len(nextIndent)+20 {
		b.WriteString(firstIndent)
		b.WriteString(text)
		b.WriteByte('\n')
		return
	}
	maxFirst := width - len(firstIndent)
	maxNext := width - len(nextIndent)
	words := strings.Fields(text)
	if len(words) == 0 {
		b.WriteString(firstIndent)
		b.WriteByte('\n')
		return
	}
	line := words[0]
	first := true
	for _, word := range words[1:] {
		max := maxNext
		if first {
			max = maxFirst
		}
		if len(line)+1+len(word) > max {
			if first {
				b.WriteString(firstIndent)
				first = false
			} else {
				b.WriteString(nextIndent)
			}
			b.WriteString(line)
			b.WriteByte('\n')
			line = word
			continue
		}
		line += " " + word
	}
	if first {
		b.WriteString(firstIndent)
	} else {
		b.WriteString(nextIndent)
	}
	b.WriteString(line)
	b.WriteByte('\n')
}
