package ast

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Node interface {
	String() string
	Children() []Node
	Fields(form Form) (fields []string)
	AllFields() (fields []string)
	KeyValue(form Form) (keyValue map[string]any)
}

type Form = map[string]any

type TextNode struct {
	FieldName string
}

func (n *TextNode) String() string   { return fmt.Sprintf(`(Text) name="%s"`, n.FieldName) }
func (n *TextNode) Children() []Node { return []Node{} }
func (n *TextNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}
	if _, ok := GetValueByKeyPath(form, splitArrowPath(n.FieldName)); ok {
		return []string{n.FieldName}
	}
	return []string{}
}

func (n *TextNode) AllFields() []string { return []string{n.FieldName} }

type RadioNode struct {
	FieldName string
	Options   []*OptionNode
}

func (n *RadioNode) String() string { return fmt.Sprintf(`(Radio) name="%s"`, n.FieldName) }
func (n *RadioNode) Children() []Node {
	nodes := make([]Node, len(n.Options))
	for i := range n.Options {
		nodes[i] = n.Options[i]
	}
	return nodes
}

func (n *RadioNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	str, ok := raw.(string)
	if !exists || !ok {
		return []string{}
	}

	fields = []string{n.FieldName}
	fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
		return opt.Value == str
	}, form)...)

	return fields
}

func (n *RadioNode) AllFields() (fields []string) {
	fields = []string{n.FieldName}
	for i := range n.Options {
		fields = append(fields, n.Options[i].AllFields()...)
	}
	return fields
}

type CheckboxNode struct {
	FieldName string
	Options   []*OptionNode
}

func (n *CheckboxNode) String() string { return fmt.Sprintf(`(Checkbox) name="%s"`, n.FieldName) }
func (n *CheckboxNode) Children() []Node {
	nodes := make([]Node, len(n.Options))
	for i := range n.Options {
		nodes[i] = n.Options[i]
	}
	return nodes
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

func (n *CheckboxNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	v, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	selected, ok := v.([]string)
	if !exists || !ok {
		return []string{}
	}

	fields = []string{n.FieldName}
	for _, str := range selected {
		fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
			return opt.Value == str
		}, form)...)
	}

	return fields
}

func (n *CheckboxNode) AllFields() (fields []string) {
	fields = []string{n.FieldName}
	for i := range n.Options {
		fields = append(fields, n.Options[i].AllFields()...)
	}
	return fields
}

type SelectNode struct {
	FieldName string
	Options   []*OptionNode
}

func (n *SelectNode) String() string { return fmt.Sprintf(`(Select) name="%s"`, n.FieldName) }
func (n *SelectNode) Children() []Node {
	nodes := make([]Node, len(n.Options))
	for i := range n.Options {
		nodes[i] = n.Options[i]
	}
	return nodes
}

func (n *SelectNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	val, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return []string{}
	}
	fields = []string{n.FieldName}
	if slice, ok := val.([]string); ok {
		for _, str := range slice {
			fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
				return opt.Value == str
			}, form)...)
		}
	} else if single, ok := val.(string); ok {
		fields = append(fields, collectOptionFields(n.Options, func(opt *OptionNode) bool {
			return opt.Value == single
		}, form)...)
	}
	return fields
}

func (n *SelectNode) AllFields() (fields []string) {
	fields = []string{n.FieldName}
	for i := range n.Options {
		fields = append(fields, n.Options[i].AllFields()...)
	}
	return fields
}

type StepNode struct {
	Label         string
	ChildrenNodes []Node
}

func (n *StepNode) String() string   { return fmt.Sprintf(`(Step) label="%s"`, n.Label) }
func (n *StepNode) Children() []Node { return n.ChildrenNodes }
func (n *StepNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, c := range n.Children() {
		fields = append(fields, c.Fields(form)...)
	}
	return fields
}

func (n *StepNode) AllFields() (fields []string) {
	fields = []string{}
	for _, c := range n.Children() {
		fields = append(fields, c.AllFields()...)
	}
	return fields
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

func Step(name string, children ...Node) *StepNode {
	return &StepNode{Label: name, ChildrenNodes: children}
}

func Text(field string) *TextNode { return &TextNode{FieldName: field} }

func Radio(field string, opts ...*OptionNode) *RadioNode {
	return &RadioNode{FieldName: field, Options: opts}
}

func Checkbox(field string, opts ...*OptionNode) *CheckboxNode {
	return &CheckboxNode{FieldName: field, Options: opts}
}

func Select(field string, opts ...*OptionNode) *SelectNode {
	return &SelectNode{FieldName: field, Options: opts}
}

func Option(option string, children ...Node) *OptionNode {
	return &OptionNode{Value: option, Nodes: children}
}

type GroupNode struct {
	Label         string
	ChildrenNodes []Node
}

func (g *GroupNode) String() string {
	return fmt.Sprintf(`(Group) label="%s"`, g.Label)
}
func (g *GroupNode) Children() []Node {
	return g.ChildrenNodes
}

func (g *GroupNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, c := range g.Children() {
		fields = append(fields, c.Fields(form)...)
	}
	return
}

func (g *GroupNode) AllFields() (fields []string) {
	fields = []string{}
	for _, c := range g.Children() {
		fields = append(fields, c.AllFields()...)
	}
	return
}

func Group(label string, children ...Node) *GroupNode {
	return &GroupNode{Label: label, ChildrenNodes: children}
}

type PageNode struct {
	Label         string
	ChildrenNodes []Node
}

func (p *PageNode) String() string {
	return fmt.Sprintf(`(Page) label="%s"`, p.Label)
}
func (p *PageNode) Children() []Node {
	return p.ChildrenNodes
}

func (p *PageNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, c := range p.Children() {
		fields = append(fields, c.Fields(form)...)
	}
	return
}

func (p *PageNode) AllFields() (fields []string) {
	fields = []string{}
	for _, c := range p.Children() {
		fields = append(fields, c.AllFields()...)
	}
	return
}

func Page(label string, children ...Node) *PageNode {
	return &PageNode{Label: label, ChildrenNodes: children}
}

type SectionNode struct {
	Label         string
	ChildrenNodes []Node
}

func (s *SectionNode) String() string {
	return fmt.Sprintf(`(Section) label="%s"`, s.Label)
}
func (s *SectionNode) Children() []Node {
	return s.ChildrenNodes
}

func (s *SectionNode) Fields(form Form) (fields []string) {
	if form == nil {
		return []string{}
	}

	fields = []string{}
	for _, c := range s.Children() {
		fields = append(fields, c.Fields(form)...)
	}
	return
}

func (s *SectionNode) AllFields() (fields []string) {
	fields = []string{}
	for _, c := range s.Children() {
		fields = append(fields, c.AllFields()...)
	}
	return
}

func Section(label string, children ...Node) *SectionNode {
	return &SectionNode{Label: label, ChildrenNodes: children}
}

func ValidateNoCycles(root Node) error {
	if root == nil {
		return fmt.Errorf("root is nil")
	}

	onPath := make(map[uintptr]bool)
	visited := make(map[uintptr]bool)

	var dfs func(n Node) error
	dfs = func(n Node) error {
		v := reflect.ValueOf(n)
		if v.Kind() != reflect.Ptr {
			return fmt.Errorf("node %T is not a pointer", n)
		}
		ptr := v.Pointer()

		if onPath[ptr] {
			return fmt.Errorf("cycle detected at node %s", n.String())
		}
		if visited[ptr] {
			return nil
		}

		onPath[ptr] = true
		for _, c := range n.Children() {
			if err := dfs(c); err != nil {
				return err
			}
		}
		onPath[ptr] = false
		visited[ptr] = true
		return nil
	}

	return dfs(root)
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

func (a *AST) Print(w io.Writer, form Form) error {
	printer := NewTreePrinter(w, form)
	return printer.Print(a.root)
}

func (a *AST) AllFields() []string {
	out := make([]string, len(a.allFields))
	copy(out, a.allFields)
	return out
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

func (p *TreePrinter) hasAnySelected(root Node) bool {
	return p.hasAnySelectedRec(root)
}

func (p *TreePrinter) hasAnySelectedRec(n Node) bool {
	if len(n.Fields(p.form)) > 0 {
		return true
	}
	for _, c := range n.Children() {
		if p.hasAnySelectedRec(c) {
			return true
		}
	}
	return false
}

type TreePrinter struct {
	form   Form
	writer io.Writer
}

func NewTreePrinter(w io.Writer, form Form) *TreePrinter {
	if w == nil {
		w = os.Stdout
	}
	return &TreePrinter{form: form, writer: w}
}

func (p *TreePrinter) Print(root Node) error {
	if p.form != nil && !p.hasAnySelected(root) {
		return nil
	}
	if _, err := fmt.Fprintf(p.writer, "%s\n", root.String()); err != nil {
		return err
	}
	if p.form != nil {
		return p.printChildren(root, "")
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

func (p *TreePrinter) printChildren(n Node, prefix string) error {
	children := n.Children()
	total := 0
	for _, c := range children {
		if p.hasAnySelectedRec(c) {
			total++
		}
	}

	printed := 0
	for _, c := range children {
		if !p.hasAnySelectedRec(c) {
			continue
		}
		printed++
		isLast := printed == total
		branch := "├── "
		next := prefix + "│   "
		if isLast {
			branch = "└── "
			next = prefix + "    "
		}

		if tn, ok := c.(*TextNode); ok {
			if val, exists := GetValueByKeyPath(p.form, splitArrowPath(tn.FieldName)); exists {
				if _, err := fmt.Fprintf(p.writer, "%s%s%s value=\"%v\"\n", prefix, branch, tn.String(), val); err != nil {
					return err
				}
			}
		} else {
			if _, err := fmt.Fprintf(p.writer, "%s%s%s\n", prefix, branch, c.String()); err != nil {
				return err
			}
			if err := p.printChildren(c, next); err != nil {
				return err
			}
		}
	}
	return nil
}

type PathSegment struct {
	Key     string
	IsIndex bool
}

//	[{Key:"a",IsIndex:false},
//	 {Key:"b",IsIndex:false},
//	 {Key:"0",IsIndex:true},
//	 {Key:"c",IsIndex:false}]

// splitArrowPath support the syntax: "[a]->[b]->0->c"
func splitArrowPath(field string) []PathSegment {
	rawParts := strings.Split(field, SEPARATOR)
	segs := make([]PathSegment, 0, len(rawParts))

	for _, raw := range rawParts {
		p := strings.TrimSpace(raw)
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			p = p[1 : len(p)-1]
			segs = append(segs, PathSegment{
				Key:     p,
				IsIndex: false,
			})
			continue
		}

		isIdx := false
		if _, err := strconv.Atoi(p); err == nil {
			isIdx = true
		}
		segs = append(segs, PathSegment{
			Key:     p,
			IsIndex: isIdx,
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
		case Form:
			v, exists := m[seg.Key]
			if !exists {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg.Key)
			if err != nil || idx < 0 || idx >= len(m) {
				return nil, false
			}
			cur = m[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func (n *TextNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	if v, ok := GetValueByKeyPath(form, splitArrowPath(n.FieldName)); ok {
		out[n.FieldName] = v
	}
	return out
}

func (o *OptionNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	for _, c := range o.Nodes {
		for k, v := range c.KeyValue(form) {
			out[k] = v
		}
	}
	return out
}

func (n *RadioNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return out
	}
	if str, ok := raw.(string); ok {
		out[n.FieldName] = str
		for _, opt := range n.Options {
			if opt.Value == str {
				for k, v := range opt.KeyValue(form) {
					out[k] = v
				}
				break
			}
		}
	}
	return out
}

func (n *CheckboxNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return out
	}
	if slice, ok := raw.([]string); ok {
		out[n.FieldName] = slice
		for _, sel := range slice {
			for _, opt := range n.Options {
				if opt.Value == sel {
					for k, v := range opt.KeyValue(form) {
						out[k] = v
					}
				}
			}
		}
	}
	return out
}

func (n *SelectNode) KeyValue(form Form) map[string]any {
	out := make(map[string]any)
	raw, exists := GetValueByKeyPath(form, splitArrowPath(n.FieldName))
	if !exists {
		return out
	}
	out[n.FieldName] = raw
	switch sel := raw.(type) {
	case string:
		for _, opt := range n.Options {
			if opt.Value == sel {
				for k, v := range opt.KeyValue(form) {
					out[k] = v
				}
				break
			}
		}
	case []string:
		for _, s := range sel {
			for _, opt := range n.Options {
				if opt.Value == s {
					for k, v := range opt.KeyValue(form) {
						out[k] = v
					}
				}
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

func (n *StepNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(n.Children(), form)
}
func (n *GroupNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(n.Children(), form)
}
func (n *PageNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(n.Children(), form)
}
func (n *SectionNode) KeyValue(form Form) map[string]any {
	return mergeChildKeyValues(n.Children(), form)
}

func (a *AST) KeyValue(form Form) map[string]any {
	return a.root.KeyValue(form)
}

func ShortKey(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, SEPARATOR)
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "[") && strings.HasSuffix(last, "]") {
		return last[1 : len(last)-1]
	}
	return last
}

const SEPARATOR = "->"
