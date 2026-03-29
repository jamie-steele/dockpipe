package pipelang

import (
	"fmt"
	"strconv"
)

type parser struct {
	toks []token
	idx  int
}

func Parse(src []byte) (*Program, error) {
	toks, err := lex(string(src))
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	return p.parseProgram()
}

func (p *parser) parseProgram() (*Program, error) {
	prog := &Program{}
	for p.peek().kind != tokEOF {
		vis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		switch p.peek().kind {
		case tokInterface:
			i, err := p.parseInterface(vis)
			if err != nil {
				return nil, err
			}
			prog.Interfaces = append(prog.Interfaces, i)
		case tokClass, tokStruct:
			c, err := p.parseClass(vis)
			if err != nil {
				return nil, err
			}
			prog.Classes = append(prog.Classes, c)
		default:
			return nil, p.errf("expected Interface, Class, or Struct")
		}
	}
	return prog, nil
}

func (p *parser) parseInterface(vis Visibility) (*InterfaceDecl, error) {
	if _, err := p.expect(tokInterface); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	decl := &InterfaceDecl{Name: nameTok.lit, Visibility: normalizeVisibility(vis)}
	for p.peek().kind != tokRBrace {
		memberVis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		t, n, params, isMethod, err := p.parseTypedMemberHeader()
		if err != nil {
			return nil, err
		}
		if isMethod {
			if _, err := p.expect(tokSemi); err != nil {
				return nil, err
			}
			decl.Methods = append(decl.Methods, MethodSig{
				Visibility: normalizeVisibility(memberVis),
				ReturnType: t,
				Name:       n,
				Params:     params,
			})
			continue
		}
		if _, err := p.expect(tokSemi); err != nil {
			return nil, err
		}
		decl.Fields = append(decl.Fields, FieldSig{
			Visibility: normalizeVisibility(memberVis),
			Type:       t,
			Name:       n,
		})
	}
	if _, err := p.expect(tokRBrace); err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *parser) parseClass(vis Visibility) (*ClassDecl, error) {
	switch p.peek().kind {
	case tokClass, tokStruct:
		p.next()
	default:
		return nil, p.errf("expected Class or Struct")
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, err
	}
	decl := &ClassDecl{Name: nameTok.lit, Visibility: normalizeVisibility(vis)}
	if p.peek().kind == tokColon {
		p.next()
		implTok, err := p.expect(tokIdent)
		if err != nil {
			return nil, err
		}
		decl.Implements = implTok.lit
	}
	if _, err := p.expect(tokLBrace); err != nil {
		return nil, err
	}
	for p.peek().kind != tokRBrace {
		memberVis, err := p.parseOptionalVisibility()
		if err != nil {
			return nil, err
		}
		t, n, params, isMethod, err := p.parseTypedMemberHeader()
		if err != nil {
			return nil, err
		}
		if isMethod {
			if _, err := p.expect(tokArrow); err != nil {
				return nil, err
			}
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokSemi); err != nil {
				return nil, err
			}
			decl.Methods = append(decl.Methods, MethodDecl{
				Visibility: normalizeVisibility(memberVis),
				ReturnType: t,
				Name:       n,
				Params:     params,
				Body:       expr,
			})
			continue
		}
		f := FieldDecl{Visibility: normalizeVisibility(memberVis), Type: t, Name: n}
		if p.peek().kind == tokAssign {
			p.next()
			expr, err := p.parseExpr(1)
			if err != nil {
				return nil, err
			}
			f.Default = expr
		}
		if _, err := p.expect(tokSemi); err != nil {
			return nil, err
		}
		decl.Fields = append(decl.Fields, f)
	}
	if _, err := p.expect(tokRBrace); err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *parser) parseOptionalVisibility() (Visibility, error) {
	switch p.peek().kind {
	case tokPublic:
		p.next()
		return VisibilityPublic, nil
	case tokPrivate:
		p.next()
		return VisibilityPrivate, nil
	default:
		return VisibilityPublic, nil
	}
}

func (p *parser) parseTypedMemberHeader() (TypeName, string, []Param, bool, error) {
	t, err := p.parseTypeName()
	if err != nil {
		return "", "", nil, false, err
	}
	nameTok, err := p.expect(tokIdent)
	if err != nil {
		return "", "", nil, false, err
	}
	if p.peek().kind != tokLParen {
		return t, nameTok.lit, nil, false, nil
	}
	p.next()
	params := []Param{}
	if p.peek().kind != tokRParen {
		for {
			pt, err := p.parseTypeName()
			if err != nil {
				return "", "", nil, false, err
			}
			pn, err := p.expect(tokIdent)
			if err != nil {
				return "", "", nil, false, err
			}
			params = append(params, Param{Type: pt, Name: pn.lit})
			if p.peek().kind == tokComma {
				p.next()
				continue
			}
			break
		}
	}
	if _, err := p.expect(tokRParen); err != nil {
		return "", "", nil, false, err
	}
	return t, nameTok.lit, params, true, nil
}

func (p *parser) parseTypeName() (TypeName, error) {
	tok, err := p.expect(tokIdent)
	if err != nil {
		return "", err
	}
	t := TypeName(tok.lit)
	if !t.IsValid() {
		return "", p.errf("unknown type %q", tok.lit)
	}
	return t, nil
}

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		opTok := p.peek()
		prec := binaryPrecedence(opTok.kind)
		if prec < minPrec {
			break
		}
		p.next()
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: opTok.lit, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	switch p.peek().kind {
	case tokBang, tokMinus:
		op := p.next().lit
		ex, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: op, Expr: ex}, nil
	default:
		return p.parsePrimary()
	}
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()
	switch t.kind {
	case tokString:
		p.next()
		return &LiteralExpr{Value: Value{Type: TypeString, String: t.lit}}, nil
	case tokInt:
		p.next()
		v, err := strconv.ParseInt(t.lit, 10, 64)
		if err != nil {
			return nil, err
		}
		return &LiteralExpr{Value: Value{Type: TypeInt, Int: v}}, nil
	case tokFloat:
		p.next()
		v, err := strconv.ParseFloat(t.lit, 64)
		if err != nil {
			return nil, err
		}
		return &LiteralExpr{Value: Value{Type: TypeFloat, Float: v}}, nil
	case tokBool:
		p.next()
		return &LiteralExpr{Value: Value{Type: TypeBool, Bool: t.lit == "true"}}, nil
	case tokIdent:
		p.next()
		return &IdentExpr{Name: t.lit}, nil
	case tokLParen:
		p.next()
		ex, err := p.parseExpr(1)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, err
		}
		return ex, nil
	default:
		return nil, p.errf("unexpected token %q in expression", t.lit)
	}
}

func binaryPrecedence(k tokenKind) int {
	switch k {
	case tokOrOr:
		return 1
	case tokAndAnd:
		return 2
	case tokEQ, tokNE:
		return 3
	case tokLT, tokLE, tokGT, tokGE:
		return 4
	case tokPlus, tokMinus:
		return 5
	case tokStar, tokSlash:
		return 6
	default:
		return 0
	}
}

func (p *parser) peek() token {
	if p.idx >= len(p.toks) {
		return token{kind: tokEOF, pos: len(p.toks)}
	}
	return p.toks[p.idx]
}

func (p *parser) next() token {
	t := p.peek()
	if p.idx < len(p.toks) {
		p.idx++
	}
	return t
}

func (p *parser) expect(k tokenKind) (token, error) {
	t := p.peek()
	if t.kind != k {
		return token{}, p.errf("expected %s, got %q", tokenName(k), t.lit)
	}
	p.next()
	return t, nil
}

func (p *parser) errf(format string, args ...any) error {
	t := p.peek()
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("parse error at %d: %s", t.pos, msg)
}

func tokenName(k tokenKind) string {
	switch k {
	case tokIdent:
		return "identifier"
	case tokLBrace:
		return "{"
	case tokRBrace:
		return "}"
	case tokLParen:
		return "("
	case tokRParen:
		return ")"
	case tokColon:
		return ":"
	case tokSemi:
		return ";"
	case tokComma:
		return ","
	case tokAssign:
		return "="
	case tokArrow:
		return "=>"
	default:
		return fmt.Sprintf("token(%d)", k)
	}
}
