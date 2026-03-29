// Command dockpipe is the main CLI: run → isolate → act (Go implementation).
package main

import (
	"bufio"
	"errors"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/application"
	"golang.org/x/term"
)

// Version is set at link time: -X main.Version=X.Y.Z (see Makefile, release/packaging/build-deb.sh, CI).
// When left as "dev", versionString() uses the embedded copy of repo-root VERSION (see src/cmd/VERSION).
var Version = "dev"

//go:embed VERSION
var versionFile string

type gitignoreDecision int

const (
	gitignoreNoop gitignoreDecision = iota
	gitignoreApply
	gitignoreDeclined
)

var dockpipeGitignoreEntries = []string{
	"bin/.dockpipe/",
	".dockpipe/",
	"*.tfstate",
	"*.tfstate.*",
	".terraform/",
}

var isTerminalFn = term.IsTerminal

func versionString() string {
	v := strings.TrimSpace(Version)
	if v != "" && v != "dev" {
		return v
	}
	return strings.TrimSpace(versionFile)
}

func hasInitHelpArg(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func hasInitGitignoreFlag(args []string) bool {
	for _, a := range args {
		if a == "--gitignore" {
			return true
		}
	}
	return false
}

func stripInitGitignoreFlag(argv []string) []string {
	if len(argv) == 0 || argv[0] != "init" {
		return argv
	}
	out := make([]string, 0, len(argv))
	out = append(out, "init")
	for _, a := range argv[1:] {
		if a == "--gitignore" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func isInteractiveSession(stdin, stdout *os.File) bool {
	if stdin == nil || stdout == nil {
		return false
	}
	return isTerminalFn(int(stdin.Fd())) && isTerminalFn(int(stdout.Fd()))
}

func promptGitignore(in io.Reader, out io.Writer) (bool, error) {
	if _, err := fmt.Fprint(out, "Add recommended DockPipe .gitignore? (Y/n) "); err != nil {
		return false, err
	}
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "n" || ans == "no" {
		return false, nil
	}
	return true, nil
}

func decideInitGitignore(argv []string, interactive bool, in io.Reader, out io.Writer) ([]string, gitignoreDecision, error) {
	if len(argv) == 0 || argv[0] != "init" {
		return argv, gitignoreNoop, nil
	}
	initArgs := argv[1:]
	if hasInitHelpArg(initArgs) {
		return argv, gitignoreNoop, nil
	}
	if hasInitGitignoreFlag(initArgs) {
		return stripInitGitignoreFlag(argv), gitignoreApply, nil
	}
	if len(initArgs) > 0 {
		return argv, gitignoreNoop, nil
	}
	if !interactive {
		return argv, gitignoreNoop, nil
	}
	ok, err := promptGitignore(in, out)
	if err != nil {
		return nil, gitignoreNoop, err
	}
	if ok {
		return argv, gitignoreApply, nil
	}
	return argv, gitignoreDeclined, nil
}

func ensureDockpipeGitignore(projectDir string) (bool, error) {
	path := filepath.Join(projectDir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if os.IsNotExist(err) {
		data = nil
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	existing := map[string]struct{}{}
	for _, line := range strings.Split(normalized, "\n") {
		existing[strings.TrimSpace(line)] = struct{}{}
	}
	missing := make([]string, 0, len(dockpipeGitignoreEntries))
	for _, entry := range dockpipeGitignoreEntries {
		if _, ok := existing[entry]; !ok {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}
	var b strings.Builder
	if len(data) > 0 {
		b.Write(data)
		if data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	for _, entry := range missing {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func main() {
	argv := os.Args[1:]
	if len(argv) == 1 {
		switch argv[0] {
		case "--version", "-v", "-V":
			fmt.Println(versionString())
			return
		}
	}
	argv, gitignoreDecision, err := decideInitGitignore(argv, isInteractiveSession(os.Stdin, os.Stdout), os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Windows: run natively (Docker Desktop + Windows git) by default. Set
	// DOCKPIPE_USE_WSL_BRIDGE=1 to forward into WSL instead.
	if runtime.GOOS == "windows" {
		if handled, code := application.TryWindowsWSLBridge(argv, os.Stdin, os.Stdout, os.Stderr); handled {
			os.Exit(code)
		}
	}
	if err := application.Run(argv, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch gitignoreDecision {
	case gitignoreApply:
		projectDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		added, err := ensureDockpipeGitignore(projectDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if added {
			fmt.Println("✔ Added DockPipe entries to .gitignore")
		} else {
			fmt.Println("✔ DockPipe entries already present in .gitignore")
		}
	case gitignoreDeclined:
		fmt.Println("Skipped .gitignore update")
	}
}
