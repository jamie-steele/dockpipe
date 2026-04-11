package pipelang

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokInt
	tokFloat
	tokString
	tokBool
	tokInterface
	tokClass
	tokStruct
	tokPublic
	tokPrivate
	tokLBrace
	tokRBrace
	tokLParen
	tokRParen
	tokColon
	tokSemi
	tokComma
	tokAssign
	tokArrow
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokBang
	tokEQ
	tokNE
	tokLT
	tokLE
	tokGT
	tokGE
	tokAndAnd
	tokOrOr
)

type token struct {
	kind tokenKind
	lit  string
	pos  int
}

func lex(src string) ([]token, error) {
	out := make([]token, 0, 128)
	i := 0
	for i < len(src) {
		ch := src[i]
		if isSpace(ch) {
			i++
			continue
		}
		if ch == '/' && i+1 < len(src) && src[i+1] == '/' {
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if isIdentStart(ch) {
			start := i
			i++
			for i < len(src) && isIdentPart(src[i]) {
				i++
			}
			lit := src[start:i]
			switch lit {
			case "Interface":
				out = append(out, token{kind: tokInterface, lit: lit, pos: start})
			case "Class":
				out = append(out, token{kind: tokClass, lit: lit, pos: start})
			case "Struct":
				out = append(out, token{kind: tokStruct, lit: lit, pos: start})
			case "public":
				out = append(out, token{kind: tokPublic, lit: lit, pos: start})
			case "private":
				out = append(out, token{kind: tokPrivate, lit: lit, pos: start})
			case "true", "false":
				out = append(out, token{kind: tokBool, lit: lit, pos: start})
			default:
				out = append(out, token{kind: tokIdent, lit: lit, pos: start})
			}
			continue
		}
		if isDigit(ch) {
			start := i
			isFloat := false
			i++
			for i < len(src) && isDigit(src[i]) {
				i++
			}
			if i < len(src) && src[i] == '.' {
				isFloat = true
				i++
				if i >= len(src) || !isDigit(src[i]) {
					return nil, fmt.Errorf("invalid float literal at %d", start)
				}
				for i < len(src) && isDigit(src[i]) {
					i++
				}
			}
			lit := src[start:i]
			if isFloat {
				out = append(out, token{kind: tokFloat, lit: lit, pos: start})
			} else {
				out = append(out, token{kind: tokInt, lit: lit, pos: start})
			}
			continue
		}
		if ch == '"' {
			start := i
			i++
			var b strings.Builder
			for i < len(src) {
				if src[i] == '"' {
					i++
					out = append(out, token{kind: tokString, lit: b.String(), pos: start})
					goto next
				}
				if src[i] == '\\' {
					if i+1 >= len(src) {
						return nil, fmt.Errorf("unterminated escape at %d", start)
					}
					esc := src[i : i+2]
					u, err := strconv.Unquote("\"" + esc + "\"")
					if err != nil {
						return nil, fmt.Errorf("invalid escape at %d", i)
					}
					b.WriteString(u)
					i += 2
					continue
				}
				b.WriteByte(src[i])
				i++
			}
			return nil, fmt.Errorf("unterminated string at %d", start)
		}

		switch ch {
		case '{':
			out = append(out, token{kind: tokLBrace, lit: "{", pos: i})
			i++
		case '}':
			out = append(out, token{kind: tokRBrace, lit: "}", pos: i})
			i++
		case '(':
			out = append(out, token{kind: tokLParen, lit: "(", pos: i})
			i++
		case ')':
			out = append(out, token{kind: tokRParen, lit: ")", pos: i})
			i++
		case ':':
			out = append(out, token{kind: tokColon, lit: ":", pos: i})
			i++
		case ';':
			out = append(out, token{kind: tokSemi, lit: ";", pos: i})
			i++
		case ',':
			out = append(out, token{kind: tokComma, lit: ",", pos: i})
			i++
		case '+':
			out = append(out, token{kind: tokPlus, lit: "+", pos: i})
			i++
		case '-':
			out = append(out, token{kind: tokMinus, lit: "-", pos: i})
			i++
		case '*':
			out = append(out, token{kind: tokStar, lit: "*", pos: i})
			i++
		case '/':
			out = append(out, token{kind: tokSlash, lit: "/", pos: i})
			i++
		case '=':
			if i+1 < len(src) && src[i+1] == '>' {
				out = append(out, token{kind: tokArrow, lit: "=>", pos: i})
				i += 2
			} else if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokEQ, lit: "==", pos: i})
				i += 2
			} else {
				out = append(out, token{kind: tokAssign, lit: "=", pos: i})
				i++
			}
		case '!':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokNE, lit: "!=", pos: i})
				i += 2
			} else {
				out = append(out, token{kind: tokBang, lit: "!", pos: i})
				i++
			}
		case '<':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokLE, lit: "<=", pos: i})
				i += 2
			} else {
				out = append(out, token{kind: tokLT, lit: "<", pos: i})
				i++
			}
		case '>':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokGE, lit: ">=", pos: i})
				i += 2
			} else {
				out = append(out, token{kind: tokGT, lit: ">", pos: i})
				i++
			}
		case '&':
			if i+1 < len(src) && src[i+1] == '&' {
				out = append(out, token{kind: tokAndAnd, lit: "&&", pos: i})
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected '&' at %d", i)
			}
		case '|':
			if i+1 < len(src) && src[i+1] == '|' {
				out = append(out, token{kind: tokOrOr, lit: "||", pos: i})
				i += 2
			} else {
				return nil, fmt.Errorf("unexpected '|' at %d", i)
			}
		default:
			return nil, fmt.Errorf("unexpected character %q at %d", ch, i)
		}
	next:
	}
	out = append(out, token{kind: tokEOF, pos: len(src)})
	return out, nil
}

func isSpace(ch byte) bool { return unicode.IsSpace(rune(ch)) }
func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}
func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}
