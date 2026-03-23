// Command dockpipe is the main CLI: run → isolate → act (Go implementation).
package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"

	"dockpipe/src/lib/dockpipe/application"
)

// Version is set at link time: -X main.Version=X.Y.Z (see Makefile, packaging/build-deb.sh, CI).
// When left as "dev", versionString() uses the embedded copy of repo-root VERSION (see src/cmd/dockpipe/VERSION).
var Version = "dev"

//go:embed VERSION
var versionFile string

func versionString() string {
	v := strings.TrimSpace(Version)
	if v != "" && v != "dev" {
		return v
	}
	return strings.TrimSpace(versionFile)
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
}
