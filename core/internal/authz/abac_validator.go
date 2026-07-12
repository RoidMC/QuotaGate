package authz

import (
	"fmt"
	"strings"
	"unicode"
)

// validateABACRule performs whitelist validation on ABAC sub/obj rules.
// Allowed grammar:
//
//	expr   := primary ("==" | "!=" | "in") primary
//	primary:= attr | literal | array | "(" expr ")"
//	attr   := "r" "." ("sub" | "obj") "." (field | "Attrs" "." ident)
//	field  := "ID" | "Owner" | "Name" | "Roles"
//	array  := "[" literal { "," literal } "]"
//	literal:= string | number | bool
//	ident  := [a-zA-Z_][a-zA-Z0-9_]*
//
// Only ==, != and in are permitted. Boolean combinators are rejected to keep
// the policy surface small and auditable.
func validateABACRule(rule, prefix string) error {
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return nil
	}
	if isConstantTruth(trimmed, prefix) {
		return fmt.Errorf("%w: ABAC %s_rule cannot be a constant truth expression", ErrInvalidABACRule, prefix)
	}
	p := newRuleParser(trimmed)
	if err := p.parseExpr(prefix); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidABACRule, err)
	}
	if !p.eof() {
		return fmt.Errorf("%w: unexpected token %q", ErrInvalidABACRule, p.peek().text)
	}
	return nil
}

func isConstantTruth(rule, prefix string) bool {
	dangerous := []string{
		"true",
		"1==1",
		"1 == 1",
		fmt.Sprintf("r.%s == r.%s", prefix, prefix),
	}
	lower := strings.ToLower(strings.TrimSpace(rule))
	for _, p := range dangerous {
		if strings.EqualFold(lower, p) {
			return true
		}
	}
	return false
}

// --- Lexer ---

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokString
	tokNumber
	tokBool
	tokDot
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokComma
	tokEq
	tokNe
	tokIn
)

type token struct {
	kind tokenKind
	text string
}

type lexer struct {
	input string
	pos   int
}

func newRuleLexer(input string) *lexer {
	return &lexer{input: input}
}

func (l *lexer) next() token {
	l.skipSpace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF}
	}

	ch := l.input[l.pos]

	switch ch {
	case '(':
		l.pos++
		return token{kind: tokLParen, text: "("}
	case ')':
		l.pos++
		return token{kind: tokRParen, text: ")"}
	case '[':
		l.pos++
		return token{kind: tokLBracket, text: "["}
	case ']':
		l.pos++
		return token{kind: tokRBracket, text: "]"}
	case ',':
		l.pos++
		return token{kind: tokComma, text: ","}
	case '.':
		l.pos++
		return token{kind: tokDot, text: "."}
	case '!':
		if l.match("!=") {
			return token{kind: tokNe, text: "!="}
		}
	case '=':
		if l.match("==") {
			return token{kind: tokEq, text: "=="}
		}
	}

	if l.matchWord("in") {
		return token{kind: tokIn, text: "in"}
	}
	if l.matchWord("true") || l.matchWord("false") {
		return token{kind: tokBool, text: l.input[l.pos-4 : l.pos]}
	}

	if ch == '"' || ch == '\'' {
		return l.readString(ch)
	}
	if unicode.IsDigit(rune(ch)) || (ch == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(rune(l.input[l.pos+1]))) {
		return l.readNumber()
	}
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		return l.readIdent()
	}

	// Unknown character: return a token so the parser can report it cleanly.
	l.pos++
	return token{kind: tokIdent, text: string(ch)}
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.input) {
		if !unicode.IsSpace(rune(l.input[l.pos])) {
			return
		}
		l.pos++
	}
}

func (l *lexer) match(s string) bool {
	if l.pos+len(s) > len(l.input) {
		return false
	}
	if l.input[l.pos:l.pos+len(s)] != s {
		return false
	}
	// Ensure "in" is not part of a longer identifier.
	if s == "in" && l.pos+len(s) < len(l.input) {
		next := l.input[l.pos+len(s)]
		if unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next)) || next == '_' {
			return false
		}
	}
	l.pos += len(s)
	return true
}

func (l *lexer) matchWord(s string) bool {
	start := l.pos
	if !l.match(s) {
		return false
	}
	// Already guarded inside match for "in"; general guard for others.
	if start+len(s) < len(l.input) {
		next := l.input[start+len(s)]
		if unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next)) || next == '_' {
			l.pos = start
			return false
		}
	}
	return true
}

func (l *lexer) readString(quote byte) token {
	start := l.pos
	l.pos++ // consume opening quote
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			sb.WriteByte(l.input[l.pos+1])
			l.pos += 2
			continue
		}
		if ch == quote {
			l.pos++
			return token{kind: tokString, text: l.input[start:l.pos]}
		}
		if ch == '\n' {
			break
		}
		sb.WriteByte(ch)
		l.pos++
	}
	return token{kind: tokString, text: l.input[start:l.pos]}
}

func (l *lexer) readNumber() token {
	start := l.pos
	if l.input[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.input) && (unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '.') {
		l.pos++
	}
	return token{kind: tokNumber, text: l.input[start:l.pos]}
}

func (l *lexer) readIdent() token {
	start := l.pos
	for l.pos < len(l.input) && (unicode.IsLetter(rune(l.input[l.pos])) || unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '_') {
		l.pos++
	}
	return token{kind: tokIdent, text: l.input[start:l.pos]}
}

// --- Parser ---

type ruleParser struct {
	lexer *lexer
	cur   token
	next  token
}

func newRuleParser(input string) *ruleParser {
	l := newRuleLexer(input)
	p := &ruleParser{lexer: l}
	p.advance()
	p.advance()
	return p
}

func (p *ruleParser) advance() {
	p.cur = p.next
	p.next = p.lexer.next()
}

func (p *ruleParser) peek() token {
	return p.cur
}

func (p *ruleParser) eof() bool {
	return p.cur.kind == tokEOF
}

func (p *ruleParser) expect(k tokenKind) (token, error) {
	if p.cur.kind != k {
		return p.cur, fmt.Errorf("expected %v, got %q", k, p.cur.text)
	}
	t := p.cur
	p.advance()
	return t, nil
}

func (p *ruleParser) parseExpr(prefix string) error {
	if p.cur.kind == tokLParen {
		p.advance()
		if err := p.parseExpr(prefix); err != nil {
			return err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return err
		}
		return nil
	}

	if err := p.parsePrimary(prefix); err != nil {
		return err
	}

	switch p.cur.kind {
	case tokEq, tokNe, tokIn:
		p.advance()
		if err := p.parsePrimary(prefix); err != nil {
			return err
		}
	default:
		return fmt.Errorf("expected operator ==, != or in, got %q", p.cur.text)
	}
	return nil
}

func (p *ruleParser) parsePrimary(prefix string) error {
	switch p.cur.kind {
	case tokIdent:
		if p.cur.text == "r" {
			return p.parseAttr(prefix)
		}
		return fmt.Errorf("unexpected identifier %q", p.cur.text)
	case tokString, tokNumber, tokBool:
		p.advance()
		return nil
	case tokLBracket:
		return p.parseArray()
	case tokLParen:
		p.advance()
		if err := p.parseExpr(prefix); err != nil {
			return err
		}
		_, err := p.expect(tokRParen)
		return err
	default:
		return fmt.Errorf("unexpected token %q", p.cur.text)
	}
}

func (p *ruleParser) parseAttr(prefix string) error {
	if _, err := p.expect(tokIdent); err != nil { // "r"
		return err
	}
	if _, err := p.expect(tokDot); err != nil {
		return err
	}
	target, err := p.expect(tokIdent)
	if err != nil {
		return err
	}
	if target.text != "sub" && target.text != "obj" {
		return fmt.Errorf("invalid request target %q", target.text)
	}
	if _, err := p.expect(tokDot); err != nil {
		return err
	}
	field, err := p.expect(tokIdent)
	if err != nil {
		return err
	}

	allowedFields := map[string]bool{"ID": true, "Owner": true, "Name": true, "Roles": true, "Attrs": true}
	if !allowedFields[field.text] {
		return fmt.Errorf("invalid attribute %q", field.text)
	}

	if field.text == "Attrs" {
		if _, err := p.expect(tokDot); err != nil {
			return err
		}
		if _, err := p.expect(tokIdent); err != nil {
			return err
		}
	}
	return nil
}

func (p *ruleParser) parseArray() error {
	if _, err := p.expect(tokLBracket); err != nil {
		return err
	}
	if p.cur.kind == tokRBracket {
		p.advance()
		return nil
	}
	for {
		if err := p.parsePrimary(""); err != nil {
			return err
		}
		if p.cur.kind == tokRBracket {
			p.advance()
			return nil
		}
		if _, err := p.expect(tokComma); err != nil {
			return err
		}
	}
}

// isConstantTruth is kept for the simple blacklist guard; the parser above
// already rejects malformed rules, but explicit constant-truth detection
// provides clearer error messages.
func isConstantTruthExpression(rule string) bool {
	lower := strings.ToLower(strings.TrimSpace(rule))
	if lower == "true" || lower == "1==1" || lower == "1 == 1" {
		return true
	}
	return false
}

// ValidateABACSubRule rejects dangerous or malformed sub rules.
func ValidateABACSubRule(rule string) error {
	return validateABACRule(rule, "sub")
}

// ValidateABACObjRule rejects dangerous or malformed obj rules.
func ValidateABACObjRule(rule string) error {
	return validateABACRule(rule, "obj")
}
