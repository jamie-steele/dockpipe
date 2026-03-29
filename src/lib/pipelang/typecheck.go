package pipelang

import (
	"fmt"
	"sort"
	"strings"
)

type checkedProgram struct {
	program    *Program
	interfaces map[string]*InterfaceDecl
	classes    map[string]*ClassDecl
}

func Check(prog *Program) (*checkedProgram, error) {
	if prog == nil {
		return nil, fmt.Errorf("program is nil")
	}
	cp := &checkedProgram{
		program:    prog,
		interfaces: map[string]*InterfaceDecl{},
		classes:    map[string]*ClassDecl{},
	}
	for _, i := range prog.Interfaces {
		if i == nil {
			continue
		}
		if strings.TrimSpace(i.Name) == "" {
			return nil, fmt.Errorf("interface name is empty")
		}
		if _, ok := cp.interfaces[i.Name]; ok {
			return nil, fmt.Errorf("duplicate interface %q", i.Name)
		}
		cp.interfaces[i.Name] = i
		if err := validateInterface(i); err != nil {
			return nil, err
		}
	}
	for _, c := range prog.Classes {
		if c == nil {
			continue
		}
		if strings.TrimSpace(c.Name) == "" {
			return nil, fmt.Errorf("class name is empty")
		}
		if _, ok := cp.classes[c.Name]; ok {
			return nil, fmt.Errorf("duplicate class %q", c.Name)
		}
		cp.classes[c.Name] = c
		if err := validateClass(c); err != nil {
			return nil, err
		}
	}
	if len(cp.classes) == 0 {
		return nil, fmt.Errorf("no class declarations found")
	}
	for _, c := range prog.Classes {
		if err := cp.validateImplements(c); err != nil {
			return nil, err
		}
	}
	return cp, nil
}

func validateInterface(i *InterfaceDecl) error {
	seen := map[string]struct{}{}
	for _, f := range i.Fields {
		if !f.Type.IsValid() {
			return fmt.Errorf("interface %s field %s has invalid type %q", i.Name, f.Name, f.Type)
		}
		if _, ok := seen[f.Name]; ok {
			return fmt.Errorf("interface %s has duplicate member %q", i.Name, f.Name)
		}
		seen[f.Name] = struct{}{}
	}
	for _, m := range i.Methods {
		if !m.ReturnType.IsValid() {
			return fmt.Errorf("interface %s method %s has invalid return type %q", i.Name, m.Name, m.ReturnType)
		}
		if _, ok := seen[m.Name]; ok {
			return fmt.Errorf("interface %s has duplicate member %q", i.Name, m.Name)
		}
		seen[m.Name] = struct{}{}
		if err := validateParams(i.Name, m.Name, m.Params); err != nil {
			return err
		}
	}
	return nil
}

func validateClass(c *ClassDecl) error {
	seen := map[string]struct{}{}
	fieldTypes := map[string]TypeName{}
	for _, f := range c.Fields {
		if !f.Type.IsValid() {
			return fmt.Errorf("class %s field %s has invalid type %q", c.Name, f.Name, f.Type)
		}
		if _, ok := seen[f.Name]; ok {
			return fmt.Errorf("class %s has duplicate member %q", c.Name, f.Name)
		}
		seen[f.Name] = struct{}{}
		fieldTypes[f.Name] = f.Type
	}
	for _, m := range c.Methods {
		if !m.ReturnType.IsValid() {
			return fmt.Errorf("class %s method %s has invalid return type %q", c.Name, m.Name, m.ReturnType)
		}
		if _, ok := seen[m.Name]; ok {
			return fmt.Errorf("class %s has duplicate member %q", c.Name, m.Name)
		}
		seen[m.Name] = struct{}{}
		if err := validateParams(c.Name, m.Name, m.Params); err != nil {
			return err
		}
	}
	for _, f := range c.Fields {
		if f.Default == nil {
			continue
		}
		t, err := inferExprType(f.Default, map[string]TypeName{})
		if err != nil {
			return fmt.Errorf("class %s field %s default: %w", c.Name, f.Name, err)
		}
		if t != f.Type {
			return fmt.Errorf("class %s field %s default type %s does not match %s", c.Name, f.Name, t, f.Type)
		}
	}
	for _, m := range c.Methods {
		env := map[string]TypeName{}
		for k, v := range fieldTypes {
			env[k] = v
		}
		for _, p := range m.Params {
			env[p.Name] = p.Type
		}
		t, err := inferExprType(m.Body, env)
		if err != nil {
			return fmt.Errorf("class %s method %s: %w", c.Name, m.Name, err)
		}
		if t != m.ReturnType {
			return fmt.Errorf("class %s method %s returns %s but declared %s", c.Name, m.Name, t, m.ReturnType)
		}
	}
	return nil
}

func validateParams(owner, method string, params []Param) error {
	seen := map[string]struct{}{}
	for _, p := range params {
		if !p.Type.IsValid() {
			return fmt.Errorf("%s method %s parameter %s has invalid type %q", owner, method, p.Name, p.Type)
		}
		if _, ok := seen[p.Name]; ok {
			return fmt.Errorf("%s method %s has duplicate parameter %q", owner, method, p.Name)
		}
		seen[p.Name] = struct{}{}
	}
	return nil
}

func (cp *checkedProgram) validateImplements(c *ClassDecl) error {
	if strings.TrimSpace(c.Implements) == "" {
		return nil
	}
	i, ok := cp.interfaces[c.Implements]
	if !ok {
		return fmt.Errorf("class %s implements unknown interface %q", c.Name, c.Implements)
	}
	fields := map[string]FieldDecl{}
	for _, f := range c.Fields {
		fields[f.Name] = f
	}
	methods := map[string]MethodDecl{}
	for _, m := range c.Methods {
		methods[m.Name] = m
	}
	for _, f := range i.Fields {
		cf, ok := fields[f.Name]
		if !ok {
			return fmt.Errorf("class %s missing interface field %s.%s", c.Name, i.Name, f.Name)
		}
		if cf.Type != f.Type {
			return fmt.Errorf("class %s field %s type %s does not match interface %s", c.Name, f.Name, cf.Type, f.Type)
		}
	}
	for _, m := range i.Methods {
		cm, ok := methods[m.Name]
		if !ok {
			return fmt.Errorf("class %s missing interface method %s.%s", c.Name, i.Name, m.Name)
		}
		if cm.ReturnType != m.ReturnType {
			return fmt.Errorf("class %s method %s return type %s does not match interface %s", c.Name, m.Name, cm.ReturnType, m.ReturnType)
		}
		if len(cm.Params) != len(m.Params) {
			return fmt.Errorf("class %s method %s parameter count mismatch", c.Name, m.Name)
		}
		for idx := range cm.Params {
			if cm.Params[idx].Type != m.Params[idx].Type {
				return fmt.Errorf("class %s method %s parameter %d type %s does not match interface %s", c.Name, m.Name, idx+1, cm.Params[idx].Type, m.Params[idx].Type)
			}
		}
	}
	return nil
}

func inferExprType(e Expr, env map[string]TypeName) (TypeName, error) {
	switch n := e.(type) {
	case *LiteralExpr:
		return n.Value.Type, nil
	case *IdentExpr:
		t, ok := env[n.Name]
		if !ok {
			return "", fmt.Errorf("unknown identifier %q", n.Name)
		}
		return t, nil
	case *UnaryExpr:
		t, err := inferExprType(n.Expr, env)
		if err != nil {
			return "", err
		}
		switch n.Op {
		case "!":
			if t != TypeBool {
				return "", fmt.Errorf("operator ! expects bool, got %s", t)
			}
			return TypeBool, nil
		case "-":
			if t != TypeInt && t != TypeFloat {
				return "", fmt.Errorf("operator - expects int or float, got %s", t)
			}
			return t, nil
		default:
			return "", fmt.Errorf("unsupported unary operator %q", n.Op)
		}
	case *BinaryExpr:
		lt, err := inferExprType(n.Left, env)
		if err != nil {
			return "", err
		}
		rt, err := inferExprType(n.Right, env)
		if err != nil {
			return "", err
		}
		return inferBinaryType(n.Op, lt, rt)
	default:
		return "", fmt.Errorf("unsupported expression")
	}
}

func inferBinaryType(op string, lt, rt TypeName) (TypeName, error) {
	switch op {
	case "+":
		if lt == TypeString && rt == TypeString {
			return TypeString, nil
		}
		if isNumeric(lt) && isNumeric(rt) {
			if lt == TypeFloat || rt == TypeFloat {
				return TypeFloat, nil
			}
			return TypeInt, nil
		}
	case "-", "*":
		if isNumeric(lt) && isNumeric(rt) {
			if lt == TypeFloat || rt == TypeFloat {
				return TypeFloat, nil
			}
			return TypeInt, nil
		}
	case "/":
		if isNumeric(lt) && isNumeric(rt) {
			return TypeFloat, nil
		}
	case "<", "<=", ">", ">=":
		if isNumeric(lt) && isNumeric(rt) {
			return TypeBool, nil
		}
	case "==", "!=":
		if lt == rt {
			return TypeBool, nil
		}
	case "&&", "||":
		if lt == TypeBool && rt == TypeBool {
			return TypeBool, nil
		}
	}
	return "", fmt.Errorf("invalid operand types for %q: %s and %s", op, lt, rt)
}

func isNumeric(t TypeName) bool {
	return t == TypeInt || t == TypeFloat
}

func pickEntryClass(cp *checkedProgram, name string) (*ClassDecl, error) {
	if cp == nil {
		return nil, fmt.Errorf("checked program is nil")
	}
	if strings.TrimSpace(name) != "" {
		c, ok := cp.classes[name]
		if !ok {
			return nil, fmt.Errorf("class %q not found", name)
		}
		return c, nil
	}
	if len(cp.program.Classes) == 0 {
		return nil, fmt.Errorf("no class declarations found")
	}
	return cp.program.Classes[0], nil
}

func sortedClassNames(cp *checkedProgram) []string {
	out := make([]string, 0, len(cp.classes))
	for name := range cp.classes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
