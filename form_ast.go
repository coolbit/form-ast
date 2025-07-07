package ast

import (
	"bytes"
	"fmt"
	"io"
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

func mergeChildKeyValues(children []Node, form Form) map[string]any {
	out := make(map[string]any)
	for _, c := range children {
		for k, v := range c.KeyValue(form) {
			out[k] = v
		}
	}
	return out
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
		case []any:
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
