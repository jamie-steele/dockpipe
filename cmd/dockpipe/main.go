// Command dockpipe is the main CLI: run → isolate → act (Go implementation).
package main

import (
	"fmt"
	"os"
	"runtime"

	"dockpipe/lib/dockpipe/application"
)

func main() {
	argv := os.Args[1:]
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
