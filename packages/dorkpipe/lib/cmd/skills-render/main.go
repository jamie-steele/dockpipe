package main

import (
	"fmt"
	"os"
	"strings"

	"dorkpipe.orchestrator/skillsrender"
)

func main() {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	if err := skillsrender.Run(os.Args[1:], env, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
