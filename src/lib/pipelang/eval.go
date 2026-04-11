package pipelang

import (
	"fmt"
)

func evalExpr(e Expr, env map[string]Value) (Value, error) {
	switch n := e.(type) {
	case *LiteralExpr:
		return n.Value, nil
	case *IdentExpr:
		v, ok := env[n.Name]
		if !ok {
			return Value{}, fmt.Errorf("unknown identifier %q", n.Name)
		}
		return v, nil
	case *UnaryExpr:
		v, err := evalExpr(n.Expr, env)
		if err != nil {
			return Value{}, err
		}
		switch n.Op {
		case "!":
			if v.Type != TypeBool {
				return Value{}, fmt.Errorf("operator ! expects bool")
			}
			return Value{Type: TypeBool, Bool: !v.Bool}, nil
		case "-":
			switch v.Type {
			case TypeInt:
				return Value{Type: TypeInt, Int: -v.Int}, nil
			case TypeFloat:
				return Value{Type: TypeFloat, Float: -v.Float}, nil
			default:
				return Value{}, fmt.Errorf("operator - expects int or float")
			}
		default:
			return Value{}, fmt.Errorf("unsupported unary operator %q", n.Op)
		}
	case *BinaryExpr:
		l, err := evalExpr(n.Left, env)
		if err != nil {
			return Value{}, err
		}
		r, err := evalExpr(n.Right, env)
		if err != nil {
			return Value{}, err
		}
		return evalBinary(n.Op, l, r)
	default:
		return Value{}, fmt.Errorf("unsupported expression")
	}
}

func evalBinary(op string, l, r Value) (Value, error) {
	switch op {
	case "+":
		if l.Type == TypeString && r.Type == TypeString {
			return Value{Type: TypeString, String: l.String + r.String}, nil
		}
		if isNumeric(l.Type) && isNumeric(r.Type) {
			if l.Type == TypeFloat || r.Type == TypeFloat {
				return Value{Type: TypeFloat, Float: asFloat(l) + asFloat(r)}, nil
			}
			return Value{Type: TypeInt, Int: l.Int + r.Int}, nil
		}
	case "-":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			if l.Type == TypeFloat || r.Type == TypeFloat {
				return Value{Type: TypeFloat, Float: asFloat(l) - asFloat(r)}, nil
			}
			return Value{Type: TypeInt, Int: l.Int - r.Int}, nil
		}
	case "*":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			if l.Type == TypeFloat || r.Type == TypeFloat {
				return Value{Type: TypeFloat, Float: asFloat(l) * asFloat(r)}, nil
			}
			return Value{Type: TypeInt, Int: l.Int * r.Int}, nil
		}
	case "/":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			if asFloat(r) == 0 {
				return Value{}, fmt.Errorf("division by zero")
			}
			return Value{Type: TypeFloat, Float: asFloat(l) / asFloat(r)}, nil
		}
	case "<":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			return Value{Type: TypeBool, Bool: asFloat(l) < asFloat(r)}, nil
		}
	case "<=":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			return Value{Type: TypeBool, Bool: asFloat(l) <= asFloat(r)}, nil
		}
	case ">":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			return Value{Type: TypeBool, Bool: asFloat(l) > asFloat(r)}, nil
		}
	case ">=":
		if isNumeric(l.Type) && isNumeric(r.Type) {
			return Value{Type: TypeBool, Bool: asFloat(l) >= asFloat(r)}, nil
		}
	case "==":
		if l.Type == r.Type {
			return Value{Type: TypeBool, Bool: equals(l, r)}, nil
		}
	case "!=":
		if l.Type == r.Type {
			return Value{Type: TypeBool, Bool: !equals(l, r)}, nil
		}
	case "&&":
		if l.Type == TypeBool && r.Type == TypeBool {
			return Value{Type: TypeBool, Bool: l.Bool && r.Bool}, nil
		}
	case "||":
		if l.Type == TypeBool && r.Type == TypeBool {
			return Value{Type: TypeBool, Bool: l.Bool || r.Bool}, nil
		}
	}
	return Value{}, fmt.Errorf("invalid operands for %q", op)
}

func equals(a, b Value) bool {
	switch a.Type {
	case TypeString:
		return a.String == b.String
	case TypeInt:
		return a.Int == b.Int
	case TypeFloat:
		return a.Float == b.Float
	case TypeBool:
		return a.Bool == b.Bool
	default:
		return false
	}
}

func asFloat(v Value) float64 {
	if v.Type == TypeFloat {
		return v.Float
	}
	return float64(v.Int)
}
