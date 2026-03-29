package pipelang

import "fmt"

// TypeName is a primitive type in PipeLang v0.0.0.1.
type TypeName string

const (
	TypeString TypeName = "string"
	TypeInt    TypeName = "int"
	TypeBool   TypeName = "bool"
	TypeFloat  TypeName = "float"
)

func (t TypeName) IsValid() bool {
	switch t {
	case TypeString, TypeInt, TypeBool, TypeFloat:
		return true
	default:
		return false
	}
}

type Program struct {
	Interfaces []*InterfaceDecl
	Classes    []*ClassDecl
}

type InterfaceDecl struct {
	Name    string
	Fields  []FieldSig
	Methods []MethodSig
}

type ClassDecl struct {
	Name       string
	Implements string
	Fields     []FieldDecl
	Methods    []MethodDecl
}

type FieldSig struct {
	Type TypeName
	Name string
}

type MethodSig struct {
	ReturnType TypeName
	Name       string
	Params     []Param
}

type FieldDecl struct {
	Type    TypeName
	Name    string
	Default Expr
}

type MethodDecl struct {
	ReturnType TypeName
	Name       string
	Params     []Param
	Body       Expr
}

type Param struct {
	Type TypeName
	Name string
}

type Expr interface {
	isExpr()
}

type (
	LiteralExpr struct {
		Value Value
	}
	IdentExpr struct {
		Name string
	}
	UnaryExpr struct {
		Op   string
		Expr Expr
	}
	BinaryExpr struct {
		Op          string
		Left, Right Expr
	}
)

func (*LiteralExpr) isExpr() {}
func (*IdentExpr) isExpr()   {}
func (*UnaryExpr) isExpr()   {}
func (*BinaryExpr) isExpr()  {}

type Value struct {
	Type   TypeName
	String string
	Int    int64
	Float  float64
	Bool   bool
}

func (v Value) StringValue() string {
	switch v.Type {
	case TypeString:
		return v.String
	case TypeInt:
		return fmt.Sprintf("%d", v.Int)
	case TypeFloat:
		return fmt.Sprintf("%g", v.Float)
	case TypeBool:
		if v.Bool {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func ZeroValue(t TypeName) Value {
	switch t {
	case TypeString:
		return Value{Type: TypeString, String: ""}
	case TypeInt:
		return Value{Type: TypeInt, Int: 0}
	case TypeFloat:
		return Value{Type: TypeFloat, Float: 0}
	case TypeBool:
		return Value{Type: TypeBool, Bool: false}
	default:
		return Value{}
	}
}
