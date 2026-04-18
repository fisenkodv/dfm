// Package cond evaluates `when:` expressions attached to directives.
//
// The grammar is intentionally tiny so it can be implemented without any
// parser library:
//
//	expr    := or
//	or      := and ("||" and)*
//	and     := unary ("&&" unary)*
//	unary   := "!" unary | primary
//	primary := "(" expr ")" | comparison | ident
//	comparison := ident ("==" | "!=") string
//	ident   := [A-Za-z_][A-Za-z0-9_.-]*
//	string  := "..." | '...'
//
// Known identifiers: "os", "arch", "hostname". Unknown identifiers evaluate
// to false (not an error) so that authors can reference optional
// conditions without exploding parse-time.
package cond

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"
)

// Context carries the runtime values used to resolve identifiers in a
// when expression. Populated once per `dfm` invocation.
type Context struct {
	OS       string // runtime.GOOS
	Arch     string // runtime.GOARCH
	Hostname string
}

// DefaultContext builds a Context from the process environment. Errors
// getting hostname fall back to an empty string rather than aborting —
// an empty hostname simply means `hostname == "..."` is false.
func DefaultContext() Context {
	h, _ := os.Hostname()
	return Context{OS: runtime.GOOS, Arch: runtime.GOARCH, Hostname: h}
}

// Eval parses and evaluates a when expression. An empty string evaluates
// to true (so "no condition" means "always run"). A parse error is
// returned as an error so the config author sees it; evaluation of an
// unknown identifier returns (false, nil).
func Eval(expr string, ctx Context) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	p := &parser{src: expr, ctx: ctx}
	p.advance()
	v, err := p.parseOr()
	if err != nil {
		return false, err
	}
	if p.tok.kind != tokEOF {
		return false, fmt.Errorf("when: unexpected %q at position %d", p.tok.val, p.tok.pos)
	}
	return v, nil
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString
	tokEq   // ==
	tokNeq  // !=
	tokAnd  // &&
	tokOr   // ||
	tokNot  // !
	tokLPar // (
	tokRPar // )
	tokIllegal
)

type token struct {
	kind tokKind
	val  string
	pos  int
}

type parser struct {
	src string
	pos int
	tok token
	ctx Context
}

// advance reads the next token, skipping whitespace.
func (p *parser) advance() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.src) {
		p.tok = token{kind: tokEOF, pos: p.pos}
		return
	}
	start := p.pos
	c := p.src[p.pos]
	switch {
	case c == '(':
		p.pos++
		p.tok = token{kind: tokLPar, val: "(", pos: start}
	case c == ')':
		p.pos++
		p.tok = token{kind: tokRPar, val: ")", pos: start}
	case c == '=':
		if p.pos+1 < len(p.src) && p.src[p.pos+1] == '=' {
			p.pos += 2
			p.tok = token{kind: tokEq, val: "==", pos: start}
		} else {
			p.pos++
			p.tok = token{kind: tokIllegal, val: "=", pos: start}
		}
	case c == '!':
		if p.pos+1 < len(p.src) && p.src[p.pos+1] == '=' {
			p.pos += 2
			p.tok = token{kind: tokNeq, val: "!=", pos: start}
		} else {
			p.pos++
			p.tok = token{kind: tokNot, val: "!", pos: start}
		}
	case c == '&' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '&':
		p.pos += 2
		p.tok = token{kind: tokAnd, val: "&&", pos: start}
	case c == '|' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '|':
		p.pos += 2
		p.tok = token{kind: tokOr, val: "||", pos: start}
	case c == '"' || c == '\'':
		quote := c
		p.pos++
		sb := strings.Builder{}
		for p.pos < len(p.src) && p.src[p.pos] != quote {
			sb.WriteByte(p.src[p.pos])
			p.pos++
		}
		if p.pos < len(p.src) {
			p.pos++ // consume closing quote
		}
		p.tok = token{kind: tokString, val: sb.String(), pos: start}
	default:
		// identifier: [A-Za-z_][A-Za-z0-9_.-]*
		if !isIdentStart(rune(c)) {
			p.pos++
			p.tok = token{kind: tokIllegal, val: string(c), pos: start}
			return
		}
		for p.pos < len(p.src) && isIdentPart(rune(p.src[p.pos])) {
			p.pos++
		}
		p.tok = token{kind: tokIdent, val: p.src[start:p.pos], pos: start}
	}
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-'
}

func (p *parser) parseOr() (bool, error) {
	v, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for p.tok.kind == tokOr {
		p.advance()
		rhs, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		v = v || rhs
	}
	return v, nil
}

func (p *parser) parseAnd() (bool, error) {
	v, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for p.tok.kind == tokAnd {
		p.advance()
		rhs, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		v = v && rhs
	}
	return v, nil
}

func (p *parser) parseUnary() (bool, error) {
	if p.tok.kind == tokNot {
		p.advance()
		v, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		return !v, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (bool, error) {
	switch p.tok.kind {
	case tokLPar:
		p.advance()
		v, err := p.parseOr()
		if err != nil {
			return false, err
		}
		if p.tok.kind != tokRPar {
			return false, fmt.Errorf("when: expected ')' at position %d", p.tok.pos)
		}
		p.advance()
		return v, nil
	case tokIdent:
		name := p.tok.val
		p.advance()
		if p.tok.kind == tokEq || p.tok.kind == tokNeq {
			op := p.tok.kind
			p.advance()
			if p.tok.kind != tokString {
				return false, fmt.Errorf("when: expected string after comparison, got %q", p.tok.val)
			}
			want := p.tok.val
			p.advance()
			got := p.resolve(name)
			eq := got == want
			if op == tokNeq {
				eq = !eq
			}
			return eq, nil
		}
		// Bare identifier — useful for future boolean flags. For now,
		// unknown identifiers are false (and so is any identifier without
		// a comparison, since we don't have booleans yet).
		return false, nil
	case tokEOF:
		return false, fmt.Errorf("when: unexpected end of expression")
	default:
		return false, fmt.Errorf("when: unexpected token %q", p.tok.val)
	}
}

func (p *parser) resolve(name string) string {
	switch name {
	case "os":
		return p.ctx.OS
	case "arch":
		return p.ctx.Arch
	case "hostname":
		return p.ctx.Hostname
	}
	return "" // unknown idents resolve to empty
}
