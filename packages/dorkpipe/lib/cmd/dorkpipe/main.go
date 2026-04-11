// Command dorkpipe is the DorkPipe orchestrator CLI (DAG on top of DockPipe).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dorkpipe.orchestrator/composegen"
	"dorkpipe.orchestrator/engine"
	"dorkpipe.orchestrator/eval"
	"dorkpipe.orchestrator/promotion"
	"dorkpipe.orchestrator/spec"
	"dorkpipe.orchestrator/workers"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "request":
		requestCmd(os.Args[2:])
	case "edit":
		editCmd(os.Args[2:])
	case "apply-edit":
		applyEditCmd(os.Args[2:])
	case "compose":
		composeCmd(os.Args[2:])
	case "validate":
		validateCmd(os.Args[2:])
	case "eval":
		evalCmd(os.Args[2:])
	case "promote":
		promoteCmd(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `dorkpipe — local-first orchestrator on top of dockpipe

Usage:
  dorkpipe run -f <spec.yaml> [--workdir <dir>]
  dorkpipe request --message <text> [--workdir <dir>]
  dorkpipe edit --message <text> [--workdir <dir>] [--apply]
  dorkpipe apply-edit --artifact-dir <dir> [--workdir <dir>]
  dorkpipe compose [-o <compose.yml>] [--no-ollama]
  dorkpipe validate -f <spec.yaml>
  dorkpipe eval [--workdir <dir>]
  dorkpipe promote [--workdir <dir>]

Environment:
  OLLAMA_HOST, DATABASE_URL (or per-node database_url_env in spec)
  PATH must include dockpipe for dockpipe/codex nodes.

`)
}

func runCmd(argv []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	specPath := fs.String("f", "", "path to DAG YAML")
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	dockpipeBin := fs.String("dockpipe-bin", "", "path to dockpipe binary (default: PATH)")
	_ = fs.Parse(argv)
	if *specPath == "" {
		fs.Usage()
		os.Exit(2)
	}
	wd := *workdir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	doc, err := spec.ParseFile(*specPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ex := &workers.Executor{
		Workdir:     wd,
		Env:         os.Environ(),
		DockpipeBin: *dockpipeBin,
	}
	if err := engine.Run(context.Background(), doc, ex, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "dorkpipe: ok (%q)\n", doc.Name)
}

func composeCmd(argv []string) {
	fs := flag.NewFlagSet("compose", flag.ExitOnError)
	out := fs.String("o", "docker-compose.dorkpipe.yml", "output file")
	noOllama := fs.Bool("no-ollama", false, "omit ollama service")
	_ = fs.Parse(argv)
	o := composegen.DefaultOptions()
	o.IncludeOllama = !*noOllama
	path := *out
	if !filepath.IsAbs(path) {
		wd, _ := os.Getwd()
		path = filepath.Join(wd, path)
	}
	if err := composegen.WriteFile(path, o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
}

func validateCmd(argv []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	specPath := fs.String("f", "", "path to DAG YAML")
	_ = fs.Parse(argv)
	if *specPath == "" {
		fs.Usage()
		os.Exit(2)
	}
	doc, err := spec.ParseFile(*specPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := validateDockpipe(doc); err != nil {
		fmt.Fprintln(os.Stderr, "validate:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "OK: %q\n", doc.Name)
}

func evalCmd(argv []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing .dorkpipe/metrics.jsonl (default cwd)")
	_ = fs.Parse(argv)
	wd := *workdir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	path := filepath.Join(wd, ".dorkpipe", "metrics.jsonl")
	st, err := eval.SummarizeFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "dorkpipe eval (%d lines)\n", st.Lines)
	fmt.Printf("avg_calibrated=%.4f escalation_rate=%.4f early_stop_rate=%.4f avg_skipped_nodes=%.2f\n",
		st.AvgCalibrated, st.EscalationRate, st.EarlyStopRate, st.AvgSkipped)
}

func promoteCmd(argv []string) {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing .dorkpipe/ (default cwd)")
	_ = fs.Parse(argv)
	wd := *workdir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	sug, err := promotion.Analyze(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "promote: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(promotion.FormatSuggestions(sug))
}

func validateDockpipe(d *spec.Doc) error {
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if !strings.EqualFold(n.Kind, "dockpipe") && !strings.EqualFold(n.Kind, "codex") {
			continue
		}
		if n.DockpipeBin != "" {
			continue
		}
		_, err := exec.LookPath("dockpipe")
		if err != nil {
			return fmt.Errorf("dockpipe not on PATH (required for node %q)", n.ID)
		}
	}
	return nil
}
