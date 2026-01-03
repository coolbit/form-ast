package ast

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type Node interface {
	String() string
	Children() []Node
	Fields(form Form) (fields []string)
	AllFields() (fields []string)
	KeyValue(form Form) (keyValue map[string]any)
}

type Form = map[string]any

type FieldNode struct{ FieldName string }

func (f *FieldNode) String() string   { return fmt.Sprintf(`(Field) field="%s"`, f.FieldName) }
func (f *FieldNode) Children() []Node { return []Node{} }
func (f *FieldNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}
	if _, ok := GetValueByKeyPath(form, splitArrowPath(f.FieldName)); ok {
		return []string{f.FieldName}
	}
	return []string{}
}
func (f *FieldNode) AllFields() []string { return []string{f.FieldName} }
func (f *FieldNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	if v, ok := GetValueByKeyPath(form, splitArrowPath(f.FieldName)); ok {
		out[f.FieldName] = v
	}
	return out
}
func Field(field string) *FieldNode { return &FieldNode{FieldName: field} }

type ChoiceNode struct {
	FieldName string
	Multiple  bool
	Options   []*OptionNode
}

func (n *ChoiceNode) String() string {
	return fmt.Sprintf(`(Choice) field="%s" multiple=%v`, n.FieldName, n.Multiple)
}
func (n *ChoiceNode) Children() []Node {
	nodes := make([]Node, len(n.Options))
	for i := range n.Options {
		nodes[i] = n.Options[i]
	}
	return nodes
}
func (n *ChoiceNode) AllFields() (fields []string) {
	fields = []string{n.FieldName}
	for i := range n.Options {
		fields = append(fields, n.Options[i].AllFields()...)
	}
	return fields
}
func (n *ChoiceNode) Fields(form Form) []string {
	if form == nil {
		return []string{}
	}
	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return []string{}
	}

	fields := []string{n.FieldName}

	if n.Multiple {
		vals, ok := raw.([]string)
		valsAny, okAny := raw.([]any)
		if !(ok || okAny) {
			return fields
		}
		if okAny {
			vals = make([]string, 0, len(valsAny))
			for i := range valsAny {
				if str, ok1 := valsAny[i].(string); ok1 {
					vals = append(vals, str)
				}
			}
		}

		for _, v := range vals {
			fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
				return opt.Value == v
			}, form)...)
		}
	} else {
		if v, ok := raw.(string); ok {
			fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
				return opt.Value == v
			}, form)...)
		}
	}
	return fields
}
func collectOptionFields(opts []*OptionNode, match func(*OptionNode) bool, form Form) []string {
	var out []string
	for _, opt := range opts {
		if match(opt) {
			out = append(out, opt.Fields(form)...)
		}
	}
	return out
}
func (n *ChoiceNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return out
	}
	out[n.FieldName] = raw

	if n.Multiple {
		selections, ok := raw.([]string)
		selectionsAny, okAny := raw.([]any)
		if !(ok || okAny) {
			return out
		}
		if okAny {
			selections = make([]string, 0, len(selectionsAny))
			for i := range selectionsAny {
				if str, ok1 := selectionsAny[i].(string); ok1 {
					selections = append(selections, str)
				}
			}
		}

		for _, sel := range selections {
			for _, opt := range n.Options {
				if opt.Value == sel {
					for k, v := range opt.KeyValue(form) {
						out[k] = v
					}
				}
			}
		}
	} else {
		if sel, ok := raw.(string); ok {
			for _, opt := range n.Options {
				if opt.Value == sel {
					for k, v := range opt.KeyValue(form) {
						out[k] = v
					}
					break
				}
			}
		}
	}
	return out
}
func Choice(field string, multiple bool, opts ...*OptionNode) *ChoiceNode {
	return &ChoiceNode{FieldName: field, Multiple: multiple, Options: opts}
}

type OptionNode struct {
	Value string
	Nodes []Node
}

func (o *OptionNode) String() string   { return fmt.Sprintf(`(Option) option="%v"`, o.Value) }
func (o *OptionNode) Children() []Node { return o.Nodes }
func (o *OptionNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, c := range o.Nodes {
		fields = append(fields, c.Fields(form)...)
	}
	return fields
}
func (o *OptionNode) AllFields() (fields []string) {
	fields = []string{}
	for _, c := range o.Nodes {
		fields = append(fields, c.AllFields()...)
	}
	return fields
}
func (o *OptionNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(o.Children(), form)
}
func Option(option string, children ...Node) *OptionNode {
	return &OptionNode{Value: option, Nodes: children}
}

type ContainerNode struct {
	Label         string
	ChildrenNodes []Node
}

func (c *ContainerNode) String() string { return fmt.Sprintf(`(Container) name="%v"`, c.Label) }
func (c *ContainerNode) Children() []Node {
	return c.ChildrenNodes
}
func (c *ContainerNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, child := range c.Children() {
		fields = append(fields, child.Fields(form)...)
	}
	return
}
func (c *ContainerNode) AllFields() (fields []string) {
	fields = []string{}
	for _, child := range c.Children() {
		fields = append(fields, child.AllFields()...)
	}
	return
}
func Container(label string, children ...Node) *ContainerNode {
	return &ContainerNode{Label: label, ChildrenNodes: children}
}
func (c *ContainerNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(c.Children(), form)
}

type IfNode struct {
	ExprSrc string
	Expr    Expr
	Then    Node
	Else    Node
}

func If(expr string, then Node, elseNode Node) *IfNode {
	parsed, err := ParseExpr(expr)
	if err != nil {
		parsed = &BoolExpr{Value: false}
	}
	return &IfNode{
		ExprSrc: strings.TrimSpace(expr),
		Expr:    parsed,
		Then:    then,
		Else:    elseNode,
	}
}

func (n *IfNode) String() string {
	return fmt.Sprintf(`(If) expr="%s"`, n.ExprSrc)
}

func (n *IfNode) Children() []Node {
	var out []Node
	if n.Then != nil {
		out = append(out, n.Then)
	}
	if n.Else != nil {
		out = append(out, n.Else)
	}
	return out
}

func (n *IfNode) Fields(form Form) []string {
	if form == nil {
		return []string{}
	}
	paths := n.Expr.CollectPaths()
	fields := append([]string{}, paths...)

	ok, _ := EvalBool(n.Expr, form)
	if ok {
		if n.Then != nil {
			fields = append(fields, n.Then.Fields(form)...)
		}
	} else {
		if n.Else != nil {
			fields = append(fields, n.Else.Fields(form)...)
		}
	}
	return unique(fields)
}

func (n *IfNode) AllFields() []string {
	paths := n.Expr.CollectPaths()
	var out []string
	out = append(out, paths...)
	if n.Then != nil {
		out = append(out, n.Then.AllFields()...)
	}
	if n.Else != nil {
		out = append(out, n.Else.AllFields()...)
	}
	return unique(out)
}

func (n *IfNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	ok, _ := EvalBool(n.Expr, form)
	if ok {
		if n.Then != nil {
			for k, v := range n.Then.KeyValue(form) {
				out[k] = v
			}
		}
	} else {
		if n.Else != nil {
			for k, v := range n.Else.KeyValue(form) {
				out[k] = v
			}
		}
	}
	return out
}

func mergeChildKeyValues(children []Node, form Form) map[string]any {
	out := make(map[string]any)
	for _, c := range children {
		for k, v := range c.KeyValue(form) {
			out[k] = v
		}
	}
	return out
}

type PathSegment struct { // e.g.: [a]->[b]->0->[c] => [{Key:"a",IsIndex:false}, {Key:"b",IsIndex:false}, {Key:"0",IsIndex:true}, {Key:"c",IsIndex:false}]
	Key     string
	IsIndex bool
}

func splitArrowPath(field string) []PathSegment { // supported: "[a]->[b]->0->[c]"
	rawParts := strings.Split(field, SEPARATOR)
	segs := make([]PathSegment, 0, len(rawParts))
	for _, raw := range rawParts {
		p := strings.TrimSpace(raw)

		// map[key]
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			p = p[1 : len(p)-1]
			segs = append(segs, PathSegment{Key: p, IsIndex: false})
			continue
		}

		// array[index]
		index, err := strconv.Atoi(p)
		segs = append(segs, PathSegment{
			Key:     p,
			IsIndex: err == nil && index >= 0,
		})
	}
	return segs
}

func GetValueByKeyPath(data any, path []PathSegment) (any, bool) {
	cur := data
	for _, seg := range path {
		if !seg.IsIndex {
			switch m := cur.(type) {
			case Form:
				v, exists := m[seg.Key]
				if !exists {
					return nil, false
				}
				cur = v
			default:
				return nil, false
			}
			continue
		}

		switch m := cur.(type) {
		case []any:
			idx, err := strconv.Atoi(seg.Key)
			if err != nil || idx < 0 || idx >= len(m) {
				return nil, false
			}
			cur = m[idx]
		case []string:
			idx, err := strconv.Atoi(seg.Key)
			if err != nil || idx < 0 || idx >= len(m) {
				return nil, false
			}
			cur = m[idx]
		case []map[string]any:
			idx, err := strconv.Atoi(seg.Key)
			if err != nil || idx < 0 || idx >= len(m) {
				return nil, false
			}
			cur = m[idx]
		case Form:
			v, exists := m[seg.Key]
			if !exists {
				return nil, false
			}
			cur = v
		default:
			return nil, false
		}
	}
	return cur, true
}

func ShortKey(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.TrimSpace(path), SEPARATOR)
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "[") && strings.HasSuffix(last, "]") {
		return last[1 : len(last)-1]
	}
	return last
}

const SEPARATOR = "->"

type TreePrinter struct {
	writer io.Writer
}

func NewTreePrinter(w io.Writer) *TreePrinter {
	if w == nil {
		w = bytes.NewBuffer([]byte{})
	}
	return &TreePrinter{writer: w}
}

func (p *TreePrinter) Print(root Node) error {
	if _, err := fmt.Fprintf(p.writer, "%s\n", root.String()); err != nil {
		return err
	}
	return p.printAll(root, "")
}

func (p *TreePrinter) printAll(n Node, prefix string) error {
	children := n.Children()
	for i, c := range children {
		branch := "├── "
		next := prefix + "│   "
		if i == len(children)-1 {
			branch = "└── "
			next = prefix + "    "
		}
		if _, err := fmt.Fprintf(p.writer, "%s%s%s\n", prefix, branch, c.String()); err != nil {
			return err
		}
		if err := p.printAll(c, next); err != nil {
			return err
		}
	}
	return nil
}

type AST struct {
	root      Node
	allFields []string
}

func NewAST(root Node) (*AST, error) {
	if root == nil {
		return nil, fmt.Errorf("root is nil")
	}
	if err := ValidateNoCycles(root); err != nil {
		return nil, err
	}

	all := root.AllFields()
	return &AST{
		root:      root,
		allFields: unique(all),
	}, nil
}

func (a *AST) Selected(form Form) []string {
	sel := a.root.Fields(form)
	return unique(sel)
}

func (a *AST) Print(w io.Writer) error {
	printer := NewTreePrinter(w)
	return printer.Print(a.root)
}

func (a *AST) AllFields() []string {
	out := make([]string, len(a.allFields))
	copy(out, a.allFields)
	return out
}
func (a *AST) KeyValue(form Form) map[string]any {
	return a.root.KeyValue(form)
}

func unique(fields []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}

func ValidateNoCycles(root Node) error {
	onPath := make(map[Node]bool)
	visited := make(map[Node]bool)

	var dfs func(n Node) error
	dfs = func(n Node) error {
		if onPath[n] {
			return fmt.Errorf("cycle detected at node %s", n.String())
		}
		if visited[n] {
			return nil
		}
		onPath[n] = true
		for _, c := range n.Children() {
			if err := dfs(c); err != nil {
				return err
			}
		}
		onPath[n] = false
		visited[n] = true
		return nil
	}

	return dfs(root)
}

type Expr interface {
	Eval(form Form) (any, error)
	CollectPaths() []string
	String() string
}

type NumberExpr struct{ Value float64 }

func (e *NumberExpr) Eval(Form) (any, error) { return e.Value, nil }
func (e *NumberExpr) CollectPaths() []string { return nil }
func (e *NumberExpr) String() string         { return fmt.Sprintf("%v", e.Value) }

type StringExpr struct{ Value string }

func (e *StringExpr) Eval(Form) (any, error) { return e.Value, nil }
func (e *StringExpr) CollectPaths() []string { return nil }
func (e *StringExpr) String() string         { return strconv.Quote(e.Value) }

type BoolExpr struct{ Value bool }

func (e *BoolExpr) Eval(Form) (any, error) { return e.Value, nil }
func (e *BoolExpr) CollectPaths() []string { return nil }
func (e *BoolExpr) String() string {
	if e.Value {
		return "true"
	}
	return "false"
}

// PathChainExpr the path chain parsed by Pratt: seg1 -> seg2 -> ...
type PathChainExpr struct {
	Segments []PathSegment
}

func (e *PathChainExpr) Eval(form Form) (any, error) {
	v, ok := GetValueByKeyPath(form, e.Segments)
	if !ok {
		return nil, fmt.Errorf("path not found: %s", e.CollectPaths()[0])
	}
	return v, nil
}

func (e *PathChainExpr) CollectPaths() []string {
	var b strings.Builder
	for i, seg := range e.Segments {
		if i > 0 {
			b.WriteString(SEPARATOR)
		}
		if seg.IsIndex { // array
			b.WriteString(seg.Key)
		} else { // map
			b.WriteByte('[')
			b.WriteString(seg.Key)
			b.WriteByte(']')
		}
	}
	return []string{b.String()}
}

func (e *PathChainExpr) String() string {
	paths := e.CollectPaths()
	if len(paths) == 1 {
		return paths[0]
	}
	return strings.Join(paths, ",")
}

type IdentExpr struct{ Name string }

func (e *IdentExpr) Eval(Form) (any, error) { return e.Name, nil }
func (e *IdentExpr) CollectPaths() []string { return nil }
func (e *IdentExpr) String() string         { return e.Name }

type CallExpr struct {
	Name string
	Args []Expr
}

func (e *CallExpr) Eval(form Form) (any, error) { return evalCall(e.Name, e.Args, form) }
func (e *CallExpr) CollectPaths() []string {
	var out []string
	for _, a := range e.Args {
		out = append(out, a.CollectPaths()...)
	}
	return unique(out)
}
func (e *CallExpr) String() string {
	parts := make([]string, len(e.Args))
	for i, a := range e.Args {
		parts[i] = a.String()
	}
	return fmt.Sprintf("%s(%s)", e.Name, strings.Join(parts, ", "))
}

type PrefixExpr struct {
	Op    string // '! Or a single negative sign 'u-'
	Right Expr
}

func (e *PrefixExpr) Eval(form Form) (any, error) { return evalPrefix(e.Op, e.Right, form) }
func (e *PrefixExpr) CollectPaths() []string      { return e.Right.CollectPaths() }
func (e *PrefixExpr) String() string              { return fmt.Sprintf("(%s %s)", e.Op, e.Right.String()) }

type InfixExpr struct {
	Left  Expr
	Op    string // + - * / > >= < <= == != && ||
	Right Expr
}

func (e *InfixExpr) Eval(form Form) (any, error) { return evalInfix(e.Left, e.Op, e.Right, form) }
func (e *InfixExpr) CollectPaths() []string {
	return unique(append(e.Left.CollectPaths(), e.Right.CollectPaths()...))
}
func (e *InfixExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Op, e.Right.String())
}

type tokType int

const (
	tEOF tokType = iota
	tNumber
	tString
	tIdent
	tPathKey  // [name]
	tLParen   // (
	tRParen   // )
	tComma    // ,
	tOp       // + - * / > < >= <= == != && || !
	tArrow    // ->
	tQuestion // ?
	tColon    // :
)

type token struct {
	typ tokType
	val string
}

type lexer struct {
	s   string
	pos int
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.s) {
		return 0
	}
	return rune(l.s[l.pos])
}
func (l *lexer) next() rune {
	if l.pos >= len(l.s) {
		return 0
	}
	r := rune(l.s[l.pos])
	l.pos++
	return r
}
func (l *lexer) skipSpace() {
	for unicode.IsSpace(l.peek()) {
		l.next()
	}
}

func (l *lexer) readNumber(first rune) token {
	var sb strings.Builder
	sb.WriteRune(first)
	dotUsed := first == '.'
	expUsed := false
	for {
		r := l.peek()
		switch {
		case unicode.IsDigit(r):
			sb.WriteRune(l.next())
		case r == '.' && !dotUsed:
			dotUsed = true
			sb.WriteRune(l.next())
		case (r == 'e' || r == 'E') && !expUsed:
			expUsed = true
			sb.WriteRune(l.next())
			if l.peek() == '+' || l.peek() == '-' {
				sb.WriteRune(l.next())
			}
			// There must be at least one exponential figure
			for unicode.IsDigit(l.peek()) {
				sb.WriteRune(l.next())
			}
		default:
			return token{tNumber, sb.String()}
		}
	}
}

func (l *lexer) readIdent(first rune) token {
	sb := strings.Builder{}
	sb.WriteRune(first)
	for {
		r := l.peek()
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(l.next())
		} else {
			break
		}
	}
	return token{tIdent, sb.String()}
}
func (l *lexer) readString() (token, error) {
	sb := strings.Builder{}
	for {
		r := l.next()
		if r == 0 {
			return token{}, fmt.Errorf("unterminated string")
		}
		if r == '"' {
			break
		}
		// TODO Simplification: Do not handle escaping
		sb.WriteRune(r)
	}
	return token{tString, sb.String()}, nil
}

func (l *lexer) nextToken() (token, error) {
	l.skipSpace()
	r := l.next()
	switch {
	case r == 0:
		return token{tEOF, ""}, nil
	case unicode.IsDigit(r) || (r == '.' && unicode.IsDigit(l.peek())):
		return l.readNumber(r), nil
	case unicode.IsLetter(r) || r == '_':
		return l.readIdent(r), nil
	case r == '"':
		return l.readString()
	case r == '[':
		// Read the paired ']'
		start := l.pos
		for {
			ch := l.next()
			if ch == 0 {
				return token{}, fmt.Errorf("unterminated [path] key")
			}
			if ch == ']' {
				inner := l.s[start : l.pos-1]
				return token{tPathKey, inner}, nil
			}
		}
	case r == '(':
		return token{tLParen, "("}, nil
	case r == ')':
		return token{tRParen, ")"}, nil
	case r == ',':
		return token{tComma, ","}, nil
	case r == '~':
		return token{tOp, "~"}, nil
	case r == '?':
		return token{tQuestion, "?"}, nil
	case r == ':':
		return token{tColon, ":"}, nil
	case r == '^':
		return token{tOp, "^"}, nil
	case r == '-':
		if l.peek() == '>' {
			l.next() // consume '>'
			return token{tArrow, "->"}, nil
		}
		return token{tOp, "-"}, nil
	default:
		// Handle the comparison of two characters
		r2 := l.peek()
		op := string(r)

		if r == '<' && r2 == '<' {
			l.next()
			return token{tOp, "<<"}, nil
		}
		if r == '>' && r2 == '>' {
			l.next()
			return token{tOp, ">>"}, nil
		}

		if (r == '>' || r == '<' || r == '=' || r == '!') && r2 == '=' {
			op += string(l.next())
			return token{tOp, op}, nil
		} else if r == '&' && r2 == '&' {
			op += string(l.next())
			return token{tOp, op}, nil
		} else if r == '|' && r2 == '|' {
			op += string(l.next())
			return token{tOp, op}, nil
		}
		return token{tOp, op}, nil
	}
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) cur() token {
	if p.i >= len(p.toks) {
		return token{tEOF, ""}
	}
	return p.toks[p.i]
}
func (p *parser) eat() token {
	t := p.cur()
	if p.i < len(p.toks) {
		p.i++
	}
	return t
}
func (p *parser) expect(typ tokType, val string) error {
	if p.cur().typ != typ || (val != "" && p.cur().val != val) {
		return fmt.Errorf("expect %v:%q, got %v:%q", typ, val, p.cur().typ, p.cur().val)
	}
	p.eat()
	return nil
}

// Binding force (the greater the priority)
func lbp(t token) int {
	switch t.typ {
	case tArrow:
		return 80 // The priority of path connection is the highest
	case tQuestion: // ?
		return 5
	case tOp:
		switch t.val {
		case "||":
			return 10
		case "&&":
			return 20
		case "==", "!=", "<", ">", "<=", ">=":
			return 30
		case "|", "^", "&", "<<", ">>":
			return 35
		case "+", "-":
			return 40
		case "*", "/", "%":
			return 50
		}
	}
	return 0
}

func (p *parser) parseExpr(rbp int) (Expr, error) {
	t := p.eat()
	left, err := p.nud(t)
	if err != nil {
		return nil, err
	}
	for {
		curTok := p.cur()
		if curTok.typ != tOp && curTok.typ != tArrow && curTok.typ != tQuestion {
			break
		}
		if lbp(curTok) <= rbp {
			break
		}
		p.eat()
		left, err = p.led(curTok, left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *parser) nud(t token) (Expr, error) {
	switch t.typ {
	case tNumber:
		float, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return &NumberExpr{}, err
		}
		return &NumberExpr{Value: float}, nil
	case tString:
		return &StringExpr{Value: t.val}, nil
	case tIdent:
		// Function call ident '(' args? ') '
		if p.cur().typ == tLParen {
			p.eat() // '('
			var args []Expr
			if p.cur().typ != tRParen {
				for {
					arg, err := p.parseExpr(0)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.cur().typ == tComma {
						p.eat()
						continue
					}
					break
				}
			}
			if err := p.expect(tRParen, ")"); err != nil {
				return nil, err
			}
			return &CallExpr{Name: t.val, Args: args}, nil
		}
		// Ordinary identifiers (non-functions)
		switch strings.ToLower(t.val) {
		case "true":
			return &BoolExpr{Value: true}, nil
		case "false":
			return &BoolExpr{Value: false}, nil
		}
		return &IdentExpr{Name: t.val}, nil

	case tPathKey:
		// Path starting point: Depart from the root form
		return &PathChainExpr{Segments: []PathSegment{{Key: t.val, IsIndex: false}}}, nil

	case tOp:
		switch t.val {
		case "!":
			right, err := p.parseExpr(60) // Prefix binding force > all binary
			if err != nil {
				return nil, err
			}
			return &PrefixExpr{Op: "!", Right: right}, nil
		case "-":
			// unary minus
			right, err := p.parseExpr(60)
			if err != nil {
				return nil, err
			}
			return &PrefixExpr{Op: "u-", Right: right}, nil
		case "~":
			right, err := p.parseExpr(60)
			if err != nil {
				return nil, err
			}
			return &PrefixExpr{Op: "~", Right: right}, nil
		}

	case tLParen:
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if err := p.expect(tRParen, ")"); err != nil {
			return nil, err
		}
		return expr, nil
	}
	return nil, fmt.Errorf("unexpected token in nud: %v %q", t.typ, t.val)
}

func (p *parser) led(t token, left Expr) (Expr, error) {
	switch t.typ {
	case tQuestion: // ?
		trueExpr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}

		//  :
		if err := p.expect(tColon, ":"); err != nil {
			return nil, err
		}

		falseExpr, err := p.parseExpr(4)
		if err != nil {
			return nil, err
		}

		return &TernaryExpr{
			Cond:  left,
			True:  trueExpr,
			False: falseExpr,
		}, nil
	case tArrow:
		seg, err := p.parsePathSegment()
		if err != nil {
			return nil, err
		}
		var chain *PathChainExpr
		switch l := left.(type) {
		case *PathChainExpr:
			chain = l
		default:
			return nil, fmt.Errorf("left side of '->' must be a path (got %T)", left)
		}
		chain.Segments = append(chain.Segments, seg)
		return chain, nil

	case tOp:
		right, err := p.parseExpr(lbp(t))
		if err != nil {
			return nil, err
		}
		return &InfixExpr{Left: left, Op: t.val, Right: right}, nil
	}
	return nil, fmt.Errorf("unexpected token in led: %v %q", t.typ, t.val)
}

// Read a path segment： [key] | ident | number
func (p *parser) parsePathSegment() (PathSegment, error) {
	switch p.cur().typ {
	case tPathKey:
		key := p.eat().val
		return PathSegment{Key: key, IsIndex: false}, nil
	case tIdent:
		key := p.eat().val
		return PathSegment{Key: key, IsIndex: false}, nil
	case tNumber:
		num := p.eat().val
		return PathSegment{Key: num, IsIndex: true}, nil
	default:
		return PathSegment{}, fmt.Errorf("invalid path segment after '->': %v %q", p.cur().typ, p.cur().val)
	}
}

func ParseExpr(src string) (Expr, error) {
	lex := &lexer{s: src}
	var toks []token
	for {
		t, err := lex.nextToken()
		if err != nil {
			return nil, err
		}
		if t.typ == tEOF {
			break
		}
		toks = append(toks, t)
	}
	p := &parser{toks: toks}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.cur().typ != tEOF {
		return nil, fmt.Errorf("unexpected token: %v %q", p.cur().typ, p.cur().val)
	}
	return expr, nil
}

func EvalBool(e Expr, form Form) (bool, error) {
	v, err := e.Eval(form)
	if err != nil {
		return false, err
	}
	return toBool(v), nil
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case nil:
		return false
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes" || s == "y"
	case int, int64, float64, float32:
		f, _ := toFloat(t)
		return f != 0
	}
	return false
}

func evalPrefix(op string, right Expr, form Form) (any, error) {
	rv, err := right.Eval(form)
	if err != nil {
		return nil, err
	}
	switch op {
	case "!":
		return !toBool(rv), nil
	case "u-":
		if f, ok := toFloat(rv); ok {
			return -f, nil
		}
		return float64(0), nil
	case "~":
		f, ok := toFloat(rv)
		if !ok {
			return float64(0), nil
		}
		return float64(^int64(f)), nil
	}
	return nil, fmt.Errorf("unknown prefix op: %s", op)
}

func evalInfix(left Expr, op string, right Expr, form Form) (any, error) {
	lv, err := left.Eval(form)
	if err != nil {
		return nil, err
	}
	rv, err := right.Eval(form)
	if err != nil {
		return nil, err
	}

	switch op {
	case "+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>":
		lf, lok := toFloat(lv)
		rf, rok := toFloat(rv)

		if op == "+" && (!lok || !rok) {
			return fmt.Sprintf("%v%v", lv, rv), nil
		}
		if !(lok && rok) {
			return float64(0), nil
		}

		if op == "&" || op == "|" || op == "^" || op == "<<" || op == ">>" {
			li := int64(lf)
			ri := int64(rf)
			switch op {
			case "&":
				return float64(li & ri), nil
			case "|":
				return float64(li | ri), nil
			case "^":
				return float64(li ^ ri), nil
			case "<<":
				return float64(li << ri), nil
			case ">>":
				return float64(li >> ri), nil
			}
		}

		switch op {
		case "+":
			return lf + rf, nil
		case "-":
			return lf - rf, nil
		case "*":
			return lf * rf, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return lf / rf, nil
		case "%":
			if rf == 0 {
				return nil, fmt.Errorf("modulo by zero")
			}
			return math.Mod(lf, rf), nil
		}
	case ">", ">=", "<", "<=", "==", "!=":
		// 1. Try numeric comparison first
		if lf, lok := toFloat(lv); lok {
			if rf, rok := toFloat(rv); rok {
				switch op {
				case ">":
					return lf > rf, nil
				case ">=":
					return lf >= rf, nil
				case "<":
					return lf < rf, nil
				case "<=":
					return lf <= rf, nil
				case "==":
					return lf == rf, nil
				case "!=":
					return lf != rf, nil
				}
			}
		}

		// 2. Fallback to String comparison
		ls := fmt.Sprintf("%v", lv)
		rs := fmt.Sprintf("%v", rv)
		switch op {
		case "==":
			return ls == rs, nil
		case "!=":
			return ls != rs, nil
		case ">":
			return ls > rs, nil
		case ">=":
			return ls >= rs, nil
		case "<":
			return ls < rs, nil
		case "<=":
			return ls <= rs, nil
		default:
			return false, nil
		}
	case "&&", "||":
		lb := toBool(lv)
		rb := toBool(rv)
		if op == "&&" {
			return lb && rb, nil
		}
		return lb || rb, nil
	}
	return nil, fmt.Errorf("unknown infix op: %s", op)
}

var funcRegistry = map[string]CustomFunc{}
var funcRegistryMutex sync.RWMutex

type CustomFunc func(args []any) (any, error)

func RegisterFunc(name string, fn CustomFunc) {
	funcRegistryMutex.Lock()
	defer funcRegistryMutex.Unlock()

	funcRegistry[strings.ToLower(name)] = fn
}

func evalCall(name string, args []Expr, form Form) (any, error) {
	funcRegistryMutex.RLock()
	fn, ok := funcRegistry[strings.ToLower(name)]
	funcRegistryMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown function: %s", name)
	}

	values := make([]any, len(args))
	for i, a := range args {
		v, err := a.Eval(form)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}

	return fn(values)
}

func init() {
	RegisterFunc("int", func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("int(x) requires 1 arg")
		}
		f, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("invalid number for int")
		}
		return float64(int(f)), nil
	})

	RegisterFunc("float", func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("float(x) requires 1 arg")
		}
		f, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("invalid number for float")
		}
		return f, nil
	})
}

type TernaryExpr struct {
	Cond  Expr
	True  Expr
	False Expr
}

func (e *TernaryExpr) Eval(form Form) (any, error) {
	condVal, err := EvalBool(e.Cond, form)
	if err != nil {
		return nil, err
	}
	if condVal {
		return e.True.Eval(form)
	}
	return e.False.Eval(form)
}

func (e *TernaryExpr) CollectPaths() []string {
	return unique(append(append(e.Cond.CollectPaths(), e.True.CollectPaths()...), e.False.CollectPaths()...))
}

func (e *TernaryExpr) String() string {
	return fmt.Sprintf("(%s ? %s : %s)", e.Cond.String(), e.True.String(), e.False.String())
}
