package pipelang

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type CompileOutput struct {
	EntryClass   string
	WorkflowYAML []byte
	BindingsJSON []byte
	BindingsEnv  []byte
	FieldValues  map[string]Value
}

type InvokeOutput struct {
	ClassName  string
	MethodName string
	Type       TypeName
	Value      Value
}

func Compile(src []byte, entryClass string) (*CompileOutput, error) {
	files := map[string][]byte{"<memory>.pipe": src}
	return CompileFiles(files, entryClass)
}

// CompileFiles parses and compiles a PipeLang program composed of multiple files.
// The merged declarations share one symbol space; class/interface references may cross file boundaries.
func CompileFiles(files map[string][]byte, entryClass string) (*CompileOutput, error) {
	prog, err := parseMergedProgram(files)
	if err != nil {
		return nil, err
	}
	cp, err := Check(prog)
	if err != nil {
		return nil, err
	}
	entry, err := pickEntryClass(cp, entryClass)
	if err != nil {
		return nil, err
	}
	vals, err := classDefaults(entry)
	if err != nil {
		return nil, err
	}
	exportedVals := exportedFieldValues(entry, vals)
	wfYAML, err := emitWorkflowYAML(entry.Name, exportedVals)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := emitBindingsJSON(entry, exportedVals)
	if err != nil {
		return nil, err
	}
	envBytes := emitBindingsEnv(exportedVals)
	return &CompileOutput{
		EntryClass:   entry.Name,
		WorkflowYAML: wfYAML,
		BindingsJSON: jsonBytes,
		BindingsEnv:  envBytes,
		FieldValues:  vals,
	}, nil
}

func Invoke(src []byte, className, methodName string, args []string) (*InvokeOutput, error) {
	files := map[string][]byte{"<memory>.pipe": src}
	return InvokeFiles(files, className, methodName, args)
}

// InvokeFiles resolves and executes a method from a merged multi-file PipeLang program.
func InvokeFiles(files map[string][]byte, className, methodName string, args []string) (*InvokeOutput, error) {
	prog, err := parseMergedProgram(files)
	if err != nil {
		return nil, err
	}
	cp, err := Check(prog)
	if err != nil {
		return nil, err
	}
	class, err := pickEntryClass(cp, className)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(methodName) == "" {
		return nil, fmt.Errorf("method name is required")
	}
	var method *MethodDecl
	for i := range class.Methods {
		if class.Methods[i].Name == methodName {
			method = &class.Methods[i]
			break
		}
	}
	if method == nil {
		return nil, fmt.Errorf("method %q not found on class %q", methodName, class.Name)
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic {
		return nil, fmt.Errorf("method %q on class %q is private and cannot be invoked from CLI", methodName, class.Name)
	}
	if len(args) != len(method.Params) {
		return nil, fmt.Errorf("method %s expects %d args, got %d", method.Name, len(method.Params), len(args))
	}
	ctx, err := classDefaults(class)
	if err != nil {
		return nil, err
	}
	for i, p := range method.Params {
		v, err := parseArgValue(p.Type, args[i])
		if err != nil {
			return nil, fmt.Errorf("arg %d (%s): %w", i+1, p.Name, err)
		}
		ctx[p.Name] = v
	}
	val, err := evalExpr(method.Body, ctx)
	if err != nil {
		return nil, err
	}
	return &InvokeOutput{ClassName: class.Name, MethodName: method.Name, Type: method.ReturnType, Value: val}, nil
}

func parseMergedProgram(files map[string][]byte) (*Program, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no input files")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	merged := &Program{}
	for _, name := range names {
		b := files[name]
		p, err := Parse(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		merged.Interfaces = append(merged.Interfaces, p.Interfaces...)
		merged.Classes = append(merged.Classes, p.Classes...)
	}
	return merged, nil
}

func parseArgValue(t TypeName, raw string) (Value, error) {
	switch t {
	case TypeString:
		return Value{Type: TypeString, String: raw}, nil
	case TypeInt:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("expected int")
		}
		return Value{Type: TypeInt, Int: v}, nil
	case TypeFloat:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Value{}, fmt.Errorf("expected float")
		}
		return Value{Type: TypeFloat, Float: v}, nil
	case TypeBool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Value{}, fmt.Errorf("expected bool")
		}
		return Value{Type: TypeBool, Bool: v}, nil
	default:
		return Value{}, fmt.Errorf("unsupported type %s", t)
	}
}

func classDefaults(c *ClassDecl) (map[string]Value, error) {
	ctx := map[string]Value{}
	for _, f := range c.Fields {
		if f.Default == nil {
			ctx[f.Name] = ZeroValue(f.Type)
			continue
		}
		v, err := evalExpr(f.Default, map[string]Value{})
		if err != nil {
			return nil, fmt.Errorf("field %s default: %w", f.Name, err)
		}
		ctx[f.Name] = v
	}
	return ctx, nil
}

func emitWorkflowYAML(className string, vals map[string]Value) ([]byte, error) {
	vars := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	fieldNames := make([]string, 0, len(vals))
	for name := range vals {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	for _, name := range fieldNames {
		k := toUpperSnake(name)
		v := vals[name].StringValue()
		vars.Content = append(vars.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}

	root := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.ToLower(className)},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "description"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Generated by PipeLang v0.0.0.1"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "vars"},
		&vars,
	)
	doc := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{&root}}
	return yaml.Marshal(&doc)
}

type methodBinding struct {
	Name       string `json:"name"`
	ReturnType string `json:"return_type"`
	Expr       string `json:"expr"`
}

type fieldBinding struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type bindingsManifest struct {
	Schema     int             `json:"schema"`
	EntryClass string          `json:"entry_class"`
	Fields     []fieldBinding  `json:"fields"`
	Methods    []methodBinding `json:"methods"`
}

func emitBindingsJSON(entry *ClassDecl, vals map[string]Value) ([]byte, error) {
	fields := make([]fieldBinding, 0, len(entry.Fields))
	for _, f := range entry.Fields {
		if normalizeVisibility(f.Visibility) != VisibilityPublic {
			continue
		}
		v := vals[f.Name]
		fields = append(fields, fieldBinding{Name: f.Name, Type: string(f.Type), Value: jsonValue(v)})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })

	methods := make([]methodBinding, 0, len(entry.Methods))
	for _, m := range entry.Methods {
		if normalizeVisibility(m.Visibility) != VisibilityPublic {
			continue
		}
		methods = append(methods, methodBinding{Name: m.Name, ReturnType: string(m.ReturnType), Expr: ExprString(m.Body)})
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })

	man := bindingsManifest{Schema: 1, EntryClass: entry.Name, Fields: fields, Methods: methods}
	return json.MarshalIndent(man, "", "  ")
}

func exportedFieldValues(entry *ClassDecl, vals map[string]Value) map[string]Value {
	out := map[string]Value{}
	for _, f := range entry.Fields {
		if normalizeVisibility(f.Visibility) != VisibilityPublic {
			continue
		}
		v, ok := vals[f.Name]
		if !ok {
			continue
		}
		out[f.Name] = v
	}
	return out
}

func jsonValue(v Value) any {
	switch v.Type {
	case TypeString:
		return v.String
	case TypeInt:
		return v.Int
	case TypeFloat:
		return v.Float
	case TypeBool:
		return v.Bool
	default:
		return nil
	}
}

func emitBindingsEnv(vals map[string]Value) []byte {
	names := make([]string, 0, len(vals))
	for name := range vals {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "PIPELANG_%s=%s\n", toUpperSnake(name), shellEscape(vals[name].StringValue()))
	}
	return []byte(b.String())
}

func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	repl := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + repl + "'"
}

func toUpperSnake(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'a' && r <= 'z' {
			r = r - ('a' - 'A')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func ExprString(e Expr) string {
	switch n := e.(type) {
	case *LiteralExpr:
		switch n.Value.Type {
		case TypeString:
			return strconv.Quote(n.Value.String)
		default:
			return n.Value.StringValue()
		}
	case *IdentExpr:
		return n.Name
	case *UnaryExpr:
		return n.Op + ExprString(n.Expr)
	case *BinaryExpr:
		return "(" + ExprString(n.Left) + " " + n.Op + " " + ExprString(n.Right) + ")"
	default:
		return ""
	}
}
