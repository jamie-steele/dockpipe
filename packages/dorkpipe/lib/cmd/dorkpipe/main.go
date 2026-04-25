// Command dorkpipe is the DorkPipe orchestrator CLI (DAG on top of DockPipe).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dorkpipe.orchestrator/cianalysis"
	"dorkpipe.orchestrator/composegen"
	"dorkpipe.orchestrator/engine"
	"dorkpipe.orchestrator/eval"
	"dorkpipe.orchestrator/handoff"
	"dorkpipe.orchestrator/promotion"
	"dorkpipe.orchestrator/spec"
	"dorkpipe.orchestrator/statepaths"
	"dorkpipe.orchestrator/userinsight"
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
	case "ci":
		ciCmd(os.Args[2:])
	case "insight":
		insightCmd(os.Args[2:])
	case "handoff":
		handoffCmd(os.Args[2:])
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
  dorkpipe ci normalize-scans [--workdir <dir>]
  dorkpipe insight <subcommand> [flags]
  dorkpipe handoff build-cursor [--workdir <dir>]
  dorkpipe handoff compliance [--workdir <dir>]
  dorkpipe validate -f <spec.yaml>
  dorkpipe eval [--workdir <dir>]
  dorkpipe promote [--workdir <dir>]

Environment:
  OLLAMA_HOST, DATABASE_URL (or per-node database_url_env in spec)
  PATH must include dockpipe for dockpipe/codex nodes.

`)
}

func handoffCmd(argv []string) {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe handoff <build-cursor|compliance> [--workdir <dir>]")
		os.Exit(2)
	}
	switch argv[0] {
	case "build-cursor":
		handoffBuildCursorCmd(argv[1:])
	case "compliance":
		handoffComplianceCmd(argv[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown handoff subcommand %q\n", argv[0])
		fmt.Fprintln(os.Stderr, "usage: dorkpipe handoff <build-cursor|compliance> [--workdir <dir>]")
		os.Exit(2)
	}
}

func handoffBuildCursorCmd(argv []string) {
	fs := flag.NewFlagSet("handoff build-cursor", flag.ExitOnError)
	workdir := fs.String("workdir", "", "repository root (default cwd)")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			env[k] = v
		}
	}
	res, err := handoff.BuildCursor(wd, env)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "build-cursor-handoff: wrote %s\n", res.DocumentPath)
	fmt.Fprintf(os.Stderr, "build-cursor-handoff: bytes %d\n", res.Bytes)
	fmt.Fprintf(os.Stderr, "build-cursor-handoff: wrote %s\n", res.PastePath)
}

func handoffComplianceCmd(argv []string) {
	fs := flag.NewFlagSet("handoff compliance", flag.ExitOnError)
	workdir := fs.String("workdir", "", "repository root (default cwd)")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	out, err := handoff.ComplianceSummary(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(out)
}

func insightCmd(argv []string) {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe insight <enqueue|process|export-by-category|mark-stale|review|supersede> [flags]")
		os.Exit(2)
	}
	switch argv[0] {
	case "enqueue":
		insightEnqueueCmd(argv[1:])
	case "process":
		insightProcessCmd(argv[1:])
	case "export-by-category":
		insightExportByCategoryCmd(argv[1:])
	case "mark-stale":
		insightMarkStaleCmd(argv[1:])
	case "review":
		insightReviewCmd(argv[1:])
	case "supersede":
		insightSupersedeCmd(argv[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown insight subcommand %q\n", argv[0])
		fmt.Fprintln(os.Stderr, "usage: dorkpipe insight <enqueue|process|export-by-category|mark-stale|review|supersede> [flags]")
		os.Exit(2)
	}
}

func insightEnqueueCmd(argv []string) {
	fs := flag.NewFlagSet("insight enqueue", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	message := fs.String("m", "", "insight text")
	categoryHint := fs.String("category-hint", "unknown", "category hint")
	repoPath := fs.String("repo-path", "", "repo path scope")
	component := fs.String("component", "", "component scope")
	workflow := fs.String("workflow", "", "workflow scope")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	raw := *message
	if raw == "" {
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		raw = string(body)
	}
	id, err := userinsight.Enqueue(wd, raw, *categoryHint, *repoPath, *component, *workflow)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(id)
}

func insightProcessCmd(argv []string) {
	fs := flag.NewFlagSet("insight process", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	res, err := userinsight.Process(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "user-insight-process: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "user-insight-process: wrote %s (new normalized insights this run: %d)\n", res.InsightsPath, res.NewCount)
}

func insightExportByCategoryCmd(argv []string) {
	fs := flag.NewFlagSet("insight export-by-category", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	_ = fs.Parse(argv)
	wd := mustWorkdir(*workdir)
	outDir, err := userinsight.ExportByCategory(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "user-insight-export-by-category: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "user-insight-export-by-category: wrote %s/*.json\n", outDir)
}

func insightMarkStaleCmd(argv []string) {
	fs := flag.NewFlagSet("insight mark-stale", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	_ = fs.Parse(argv)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe insight mark-stale <insight-or-queue-id> [--workdir <dir>]")
		os.Exit(2)
	}
	wd := mustWorkdir(*workdir)
	id := fs.Arg(0)
	if err := userinsight.MarkStale(wd, id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("user-insight-mark-stale: %s\n", id)
}

func insightReviewCmd(argv []string) {
	fs := flag.NewFlagSet("insight review", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	reason := fs.String("reason", "", "review reason")
	_ = fs.Parse(argv)
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe insight review accept|reject <insight-or-queue-id> [--reason ...] [--workdir <dir>]")
		os.Exit(2)
	}
	wd := mustWorkdir(*workdir)
	action := fs.Arg(0)
	id := fs.Arg(1)
	status, err := userinsight.Review(wd, action, id, *reason)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("user-insight-review: %s %s -> %s\n", action, id, status)
}

func insightSupersedeCmd(argv []string) {
	fs := flag.NewFlagSet("insight supersede", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/analysis (default cwd)")
	_ = fs.Parse(argv)
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe insight supersede <new_insight_id> <old_insight_id> [--workdir <dir>]")
		os.Exit(2)
	}
	wd := mustWorkdir(*workdir)
	newID := fs.Arg(0)
	oldID := fs.Arg(1)
	if err := userinsight.Supersede(wd, newID, oldID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("user-insight-supersede: %s supersedes %s\n", newID, oldID)
}

func mustWorkdir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return wd
}

func ciCmd(argv []string) {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dorkpipe ci normalize-scans [--workdir <dir>]")
		os.Exit(2)
	}
	switch argv[0] {
	case "normalize-scans":
		ciNormalizeScansCmd(argv[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown ci subcommand %q\n", argv[0])
		fmt.Fprintln(os.Stderr, "usage: dorkpipe ci normalize-scans [--workdir <dir>]")
		os.Exit(2)
	}
}

func ciNormalizeScansCmd(argv []string) {
	fs := flag.NewFlagSet("ci normalize-scans", flag.ExitOnError)
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/ci-raw (default cwd)")
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
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			env[k] = v
		}
	}
	res, err := cianalysis.Normalize(wd, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "normalize-ci-scans: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "normalize-ci-scans: wrote %s (%d findings) and %s\n", res.FindingsPath, res.Count, res.SummaryPath)
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
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/packages/dorkpipe/metrics.jsonl (default cwd)")
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
	path, err := statepaths.MetricsPath(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	workdir := fs.String("workdir", "", "directory containing bin/.dockpipe/packages/dorkpipe/ (default cwd)")
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
