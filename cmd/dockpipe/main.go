// Command dockpipe is the main CLI: run → isolate → act (Go implementation).
package main

import (
	"fmt"
	"os"
	"runtime"

	"dockpipe/lib/dockpipe/application"
)

// Version is set at link time: -X main.Version=X.Y.Z (see Makefile, packaging/build-deb.sh, CI).
var Version = "dev"

func main() {
	argv := os.Args[1:]
	if len(argv) == 1 {
		switch argv[0] {
		case "--version", "-v", "-V":
			fmt.Println(Version)
			return
		}
	}
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
