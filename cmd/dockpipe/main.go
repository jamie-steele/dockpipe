// Command dockpipe is the main CLI: run → isolate → act (Go implementation).
package main

import (
	"fmt"
	"os"

	"dockpipe/lib/dockpipe/application"
)

func main() {
	if err := application.Run(os.Args[1:], os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
