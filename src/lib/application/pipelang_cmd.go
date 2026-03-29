package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/pipelang"
)

func cmdPipeLang(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(pipelangUsageText)
		return nil
	}
	switch args[0] {
	case "compile":
		return cmdPipeLangCompile(args[1:])
	case "invoke":
		return cmdPipeLangInvoke(args[1:])
	default:
		return fmt.Errorf("unknown pipelang subcommand %q (try: compile or invoke)", args[0])
	}
}

func cmdPipeLangCompile(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(pipelangCompileUsageText)
		return nil
	}
	var inPath, outDir, entry string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--in":
			if i+1 >= len(args) {
				return fmt.Errorf("--in requires a file path")
			}
			inPath = args[i+1]
			i++
		case "--out":
			if i+1 >= len(args) {
				return fmt.Errorf("--out requires a directory")
			}
			outDir = args[i+1]
			i++
		case "--entry":
			if i+1 >= len(args) {
				return fmt.Errorf("--entry requires a class name")
			}
			entry = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown option %s", args[i])
			}
			if inPath == "" {
				inPath = args[i]
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if strings.TrimSpace(inPath) == "" {
		return fmt.Errorf("missing input file (use --in <file.pipe>)")
	}
	if outDir == "" {
		outDir = filepath.Join(".dockpipe", "pipelang")
	}
	src, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	res, err := pipelang.Compile(src, entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	base := res.EntryClass
	wf := filepath.Join(outDir, base+".workflow.yml")
	bj := filepath.Join(outDir, base+".bindings.json")
	be := filepath.Join(outDir, base+".bindings.env")
	if err := os.WriteFile(wf, res.WorkflowYAML, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(bj, res.BindingsJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(be, res.BindingsEnv, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] PipeLang compiled: %s\n", inPath)
	fmt.Fprintf(os.Stderr, "[dockpipe]   workflow: %s\n", wf)
	fmt.Fprintf(os.Stderr, "[dockpipe]   bindings: %s\n", bj)
	fmt.Fprintf(os.Stderr, "[dockpipe]   env: %s\n", be)
	return nil
}

func cmdPipeLangInvoke(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(pipelangInvokeUsageText)
		return nil
	}
	var inPath, className, methodName, format string
	var argVals []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--in":
			if i+1 >= len(args) {
				return fmt.Errorf("--in requires a file path")
			}
			inPath = args[i+1]
			i++
		case "--class":
			if i+1 >= len(args) {
				return fmt.Errorf("--class requires a class name")
			}
			className = args[i+1]
			i++
		case "--method":
			if i+1 >= len(args) {
				return fmt.Errorf("--method requires a method name")
			}
			methodName = args[i+1]
			i++
		case "--arg":
			if i+1 >= len(args) {
				return fmt.Errorf("--arg requires a value")
			}
			argVals = append(argVals, args[i+1])
			i++
		case "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("--format requires text|json|env")
			}
			format = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown option %s", args[i])
			}
			if inPath == "" {
				inPath = args[i]
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if strings.TrimSpace(inPath) == "" {
		return fmt.Errorf("missing input file (use --in <file.pipe>)")
	}
	if strings.TrimSpace(methodName) == "" {
		return fmt.Errorf("--method is required")
	}
	if format == "" {
		format = "text"
	}
	src, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	res, err := pipelang.Invoke(src, className, methodName, argVals)
	if err != nil {
		return err
	}
	switch format {
	case "text":
		fmt.Fprintln(os.Stdout, res.Value.StringValue())
	case "env":
		fmt.Fprintf(os.Stdout, "PIPELANG_RESULT_TYPE=%s\n", res.Type)
		fmt.Fprintf(os.Stdout, "PIPELANG_RESULT=%s\n", shellQuoteForEnv(res.Value.StringValue()))
	case "json":
		payload := map[string]any{
			"class":  res.ClassName,
			"method": res.MethodName,
			"type":   string(res.Type),
			"value":  jsonRawValue(res.Value),
		}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
	default:
		return fmt.Errorf("unknown --format %q (use text|json|env)", format)
	}
	return nil
}

func shellQuoteForEnv(s string) string {
	if s == "" {
		return "''"
	}
	repl := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + repl + "'"
}

func jsonRawValue(v pipelang.Value) any {
	switch v.Type {
	case pipelang.TypeString:
		return v.String
	case pipelang.TypeInt:
		return v.Int
	case pipelang.TypeFloat:
		return v.Float
	case pipelang.TypeBool:
		return v.Bool
	default:
		return nil
	}
}

const pipelangUsageText = `dockpipe pipelang

Typed authoring helpers for PipeLang (optional layer over workflow YAML).

Usage:
  dockpipe pipelang compile --in <file.pipe> [--entry <ClassName>] [--out <dir>]
  dockpipe pipelang invoke --in <file.pipe> [--class <ClassName>] --method <name> [--arg <value>]... [--format text|json|env]

`

const pipelangCompileUsageText = `dockpipe pipelang compile

Compile PipeLang source into inspectable artifacts.

Usage:
  dockpipe pipelang compile --in <file.pipe> [--entry <ClassName>] [--out <dir>]

Outputs:
  <out>/<Class>.workflow.yml
  <out>/<Class>.bindings.json
  <out>/<Class>.bindings.env

`

const pipelangInvokeUsageText = `dockpipe pipelang invoke

Invoke a PipeLang method via CLI only.

Usage:
  dockpipe pipelang invoke --in <file.pipe> [--class <ClassName>] --method <name> [--arg <value>]... [--format text|json|env]

Notes:
  - Methods are pure expression-bodied logic in v0.0.0.1.
  - No runtime/resolver execution path is used by invoke.

`
