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
		if !ok {
			return fields
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
		if !ok {
			return out
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

func mergeChildKeyValues(children []Node, form Form) map[string]any {
	out := make(map[string]any)
	for _, c := range children {
		for k, v := range c.KeyValue(form) {
			out[k] = v
		}
	}
	return out
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

type TreePrinter struct {
	form   Form
	writer io.Writer
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

		if tn, ok := c.(*FieldNode); ok {
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

type PathSegment struct { // PathSegment [a]->[b]->0->c => [{Key:"a",IsIndex:false}, {Key:"b",IsIndex:false}, {Key:"0",IsIndex:true}, {Key:"c",IsIndex:false}]
	Key     string
	IsIndex bool
}

func splitArrowPath(field string) []PathSegment { // splitArrowPath support the syntax: "[a]->[b]->0->c"
	rawParts := strings.Split(field, SEPARATOR)
	segs := make([]PathSegment, 0, len(rawParts))
	for _, raw := range rawParts {
		p := strings.TrimSpace(raw)
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			p = p[1 : len(p)-1]
			segs = append(segs, PathSegment{Key: p, IsIndex: false})
			continue
		}
		segs = append(segs, PathSegment{Key: p, IsIndex: func(s string) bool { _, err := strconv.Atoi(s); return err == nil }(p)})
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
