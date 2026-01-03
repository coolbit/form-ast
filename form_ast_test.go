package ast

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUnique(t *testing.T) {
	t.Parallel()
	in := []string{"a", "b", "a", "c", "b"}
	exp := []string{"a", "b", "c"}
	out := unique(in)
	if !reflect.DeepEqual(out, exp) {
		t.Errorf("unique(%v) = %v; want %v", in, out, exp)
	}
}

func TestValidateNoCycles(t *testing.T) {
	// Serial test: building a cycle should error
	step := Container("S")
	group := Container("G", step)
	step.ChildrenNodes = append(step.ChildrenNodes, group)
	err := ValidateNoCycles(group)
	if err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Errorf("Expected cycle error, got %v", err)
	}
}

func TestNewAST_NilRoot(t *testing.T) {
	t.Parallel()
	_, err := NewAST(nil)
	if err == nil || !strings.Contains(err.Error(), "root is nil") {
		t.Errorf("Expected nil-root error, got %v", err)
	}
}

func TestAST_AllFieldsAndSelected(t *testing.T) {
	t.Parallel()
	radioOpts := []*OptionNode{
		Option("x", Field("fX")),
		Option("y", Field("fY")),
	}
	r := Choice("mode", false, radioOpts...)
	root := Container("Root", Field("u"), r)
	ast, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	expAll := unique([]string{"u", "mode", "fX", "fY"})
	if !reflect.DeepEqual(ast.AllFields(), expAll) {
		t.Errorf("AllFields = %v; want %v", ast.AllFields(), expAll)
	}
	sel := ast.Selected(Form{"mode": "y", "u": "val", "fY": "valY"})
	expSel := unique([]string{"u", "mode", "fY"})
	if !reflect.DeepEqual(sel, expSel) {
		t.Errorf("Selected = %v; want %v", sel, expSel)
	}
}

func TestCheckboxFields(t *testing.T) {
	t.Parallel()
	chk := Choice("opts", true,
		Option("a", Field("fA")),
		Option("b", Field("fB")),
		Option("c", Field("fC")),
	)
	root := Container("ChkRoot", chk)
	ast, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	form := Form{"opts": []string{"a", "c"}, "fA": "1", "fC": "3"}
	sel := ast.Selected(form)
	exp := unique([]string{"opts", "fA", "fC"})
	if !reflect.DeepEqual(sel, exp) {
		t.Errorf("Checkbox selected = %v; want %v", sel, exp)
	}
}

func TestSelectFields(t *testing.T) {
	t.Parallel()
	sel1 := Choice("sel1", false,
		Option("x", Field("fX1")),
		Option("y", Field("fY1")),
	)
	sel2 := Choice("sel2", true,
		Option("m", Field("fM2")),
		Option("n", Field("fN2")),
	)
	root := Container("SelRoot", sel1, sel2)
	ast, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	form := Form{"sel1": "y", "fY1": "yes", "sel2": []string{"m", "n"}, "fM2": "mval", "fN2": "nval"}
	sel := ast.Selected(form)
	exp := unique([]string{"sel1", "fY1", "sel2", "fM2", "fN2"})
	if !reflect.DeepEqual(sel, exp) {
		t.Errorf("Select selected = %v; want %v", sel, exp)
	}
}

func TestNestedContainers(t *testing.T) {
	t.Parallel()
	inner := Container("Inner", Field("fI"))
	mid := Container("Mid", inner, Field("fM"))
	outer := Container("Outer", mid, Field("fO"))
	ast, err := NewAST(outer)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	expAll := unique([]string{"fI", "fM", "fO"})
	if !reflect.DeepEqual(ast.AllFields(), expAll) {
		t.Errorf("Nested AllFields = %v; want %v", ast.AllFields(), expAll)
	}
	form := Form{"fM": "valM", "fO": "valO"}
	sel := ast.Selected(form)
	expSel := unique([]string{"fM", "fO"})
	if !reflect.DeepEqual(sel, expSel) {
		t.Errorf("Nested Selected = %v; want %v", sel, expSel)
	}
}

// Test with 1000 nested nodes to ensure performance and no stack overflow.
func TestDeepNestedTreeParallel(t *testing.T) {
	t.Parallel()
	const depth = 1000
	var root Node
	for i := 0; i < depth; i++ {
		label := fmt.Sprintf("Step%d", i)
		field := fmt.Sprintf("f%d", i)
		if root == nil {
			root = Container(label, Field(field))
		} else {
			root = Container(label, root, Field(field))
		}
	}
	ast, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST error on deep tree: %v", err)
	}
	form := make(Form)
	for i := 0; i < depth; i++ {
		form[fmt.Sprintf("f%d", i)] = fmt.Sprintf("v%d", i)
	}
	all := ast.AllFields()
	if len(all) != depth {
		t.Errorf("AllFields length = %d; want %d", len(all), depth)
	}
	sel := ast.Selected(form)
	if len(sel) != depth {
		t.Errorf("Selected length = %d; want %d", len(sel), depth)
	}
}

// Test a large mixed AST covering Field, Radio, Checkbox, Select, Container in parallel
// Test a large mixed AST covering Field, Radio, Checkbox, Select, Container in parallel
func TestMixedLargeAST_FieldsParallel(t *testing.T) {
	t.Parallel()
	// Build mixed nodes
	var texts []Node
	for i := 0; i < 50; i++ {
		texts = append(texts, Field(fmt.Sprintf("T%d", i)))
	}
	// Build Grp1 manually to avoid slice expansion
	grp1 := Container("Grp1")
	for i := 0; i < 10; i++ {
		grp1.ChildrenNodes = append(grp1.ChildrenNodes, texts[i])
	}
	radio := Choice("mode", false,
		Option("one", Field("R1")),
		Option("two", Field("R2")),
		Option("three", Field("R3")),
	)
	checkbox := Choice("opts", true,
		Option("a", Field("C1")),
		Option("b", Field("C2")),
		Option("c", Field("C3")),
	)
	selectNode := Choice("sel", true,
		Option("alpha", Field("S1")),
		Option("beta", Field("S2")),
	)
	container := Container("ContainerRoot", grp1, radio, checkbox, selectNode)
	// Build RootMixed manually
	rootMixed := Container("RootMixed", container)
	for i := 10; i < len(texts); i++ {
		rootMixed.ChildrenNodes = append(rootMixed.ChildrenNodes, texts[i])
	}
	ast, err := NewAST(rootMixed)
	if err != nil {
		t.Fatalf("NewAST error on mixed AST: %v", err)
	}
	// Prepare different forms to test
	forms := []Form{
		{},
		func() Form {
			f := make(Form)
			for i := 0; i < 10; i++ {
				f[fmt.Sprintf("T%d", i)] = fmt.Sprint(i)
			}
			return f
		}(),
		Form{"mode": "two", "R2": "val"},
		Form{"opts": []string{"a", "c"}, "C1": "v1", "C3": "v3"},
		Form{"sel": "beta", "S2": "val2"},
		func() Form {
			f := make(Form)
			f["mode"] = "three"
			f["R3"] = "v3"
			f["sel"] = "alpha"
			f["S1"] = "v1"
			f["opts"] = []string{"a", "b", "c"}
			f["C1"] = "c1"
			f["C2"] = "c2"
			f["C3"] = "c3"
			for i := 0; i < 50; i++ {
				f[fmt.Sprintf("T%d", i)] = fmt.Sprint(i)
			}
			return f
		}(),
	}
	for idx, form := range forms {
		idx, form := idx, form
		t.Run(fmt.Sprintf("form%d", idx), func(t *testing.T) {
			t.Parallel()
			sel := ast.Selected(form)
			// Ensure no duplicates
			seen := map[string]bool{}
			for _, f := range sel {
				if seen[f] {
					t.Errorf("duplicate field %s in form %d result", f, idx)
				}
				seen[f] = true
			}
			if sel == nil {
				t.Errorf("Selected returned nil slice for form %d", idx)
			}
		})
	}
}

func TestPrint(t *testing.T) {
	form := Form{
		"username":     "alice",
		"mode":         "advanced",
		"modeAdvFlag":  "f",
		"features":     []string{"f1", "f2", "f4"},
		"options":      []string{"o1", "o3"},
		"feat1":        "1",
		"feat2":        "2",
		"feat3":        "3",
		"level":        "gold",
		"singleSelect": "beta",
		"selAlpha":     "alpha",
		"selBeta":      "beta",
		"opt3Only":     "opt3Only",
	}

	root := Container("Form A",
		Container("dashboard",
			Field("username"),
			Choice("mode", false,
				Option("basic",
					Field("modeSimpleFlag"),
				),
				Option("advanced",
					Field("modeAdvFlag"),
					Choice("features", true,
						Option("f1", Field("feat1")),
						Option("f2", Field("feat2")),
						Option("f3", Field("feat3")),
					),
				),
			),
			Choice("options", true,
				Option("o1",
					Choice("singleSelect", true,
						Option("alpha", Field("selAlpha")),
						Option("beta", Field("selBeta")),
					),
				),
				Option("o2", Field("opt2Only")),
				Option("o3", Field("opt3Only")),
			),
		),
		Container("profile",
			Field("level"),
		),
	)

	ast, err := NewAST(root)
	if err != nil {
		panic(err)
	}

	fmt.Println("All fields:", ast.AllFields())
	fmt.Println("selected fields:", ast.Selected(form))

	ast.Print(os.Stdout)
}

// TestSplitArrowPath verifies parsing of arrow-delimited paths into segments.
func TestSplitArrowPath(t *testing.T) {
	cases := []struct {
		input string
		want  []PathSegment
	}{
		{"Level", []PathSegment{{Key: "Level", IsIndex: false}}},
		{"[Key1]", []PathSegment{{Key: "Key1", IsIndex: false}}},
		{`a -> [b] -> 0 -> c`, []PathSegment{
			{Key: "a", IsIndex: false},
			{Key: "b", IsIndex: false},
			{Key: "0", IsIndex: true},
			{Key: "c", IsIndex: false},
		}},
	}

	for _, c := range cases {
		got := splitArrowPath(c.input)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitArrowPath(%q) = %#v; want %#v", c.input, got, c.want)
		}
	}
}

// TestGetValueByKeyPath tests retrieval of values using arrow-split paths.
func TestGetValueByKeyPath_MapSliceMix(t *testing.T) {
	// Test map-only
	formMap := map[string]any{"foo": "bar", "num": 42}
	mapCases := []struct {
		path   string
		want   any
		wantOk bool
	}{
		{"[foo]", "bar", true},
		{"[num]", 42, true},
		{"baz", nil, false},
	}
	for _, tc := range mapCases {
		val, ok := GetValueByKeyPath(formMap, splitArrowPath(tc.path))
		if ok != tc.wantOk || val != tc.want {
			t.Errorf("GetValueByKeyPath(formMap, %q) = (%v, %v); want (%v, %v)", tc.path, val, ok, tc.want, tc.wantOk)
		}
	}

	// Test slice-only
	sliceData := []any{"zero", 1, map[string]any{"inner": "x"}}
	sliceCases := []struct {
		path   string
		want   any
		wantOk bool
	}{
		{"0", "zero", true},
		{"1", 1, true},
		{"2 -> inner", "x", true},
		{"3", nil, false},
	}
	for _, tc := range sliceCases {
		val, ok := GetValueByKeyPath(sliceData, splitArrowPath(tc.path))
		if !reflect.DeepEqual(val, tc.want) || ok != tc.wantOk {
			t.Errorf("GetValueByKeyPath(sliceData, %q) = (%#v, %v); want (%#v, %v)", tc.path, val, ok, tc.want, tc.wantOk)
		}
	}

	// Test nested mix
	form := map[string]any{
		"users": []any{
			map[string]any{"name": "alice"},
			map[string]any{"name": "bob"},
		},
		"settings": map[string]any{"levels": []any{10, 20, 30}},
	}
	mixCases := []struct {
		path   string
		want   any
		wantOk bool
	}{
		{"[users] -> 0 -> name", "alice", true},
		{"settings -> [levels] -> 2", 30, true},
		{"settings -> levels -> 3", nil, false},
	}
	for _, tc := range mixCases {
		val, ok := GetValueByKeyPath(form, splitArrowPath(tc.path))
		if !reflect.DeepEqual(val, tc.want) || ok != tc.wantOk {
			t.Errorf("GetValueByKeyPath(form, %q) = (%#v, %v); want (%#v, %v)", tc.path, val, ok, tc.want, tc.wantOk)
		}
	}
}

func TestGetValueByKeyPath_Expressions(t *testing.T) {
	sample := Form{
		"flat":      "v0",
		"user":      Form{"name": "alice", "age": 30},
		"profile":   map[string]interface{}{"active": true},
		"items":     []any{"zero", "one", Form{"sub": "deep"}},
		"matrix":    []any{[]any{1, 2}, []any{3, 4}},
		"mixed":     Form{"arr": []any{Form{"x": "X"}, Form{"x": "Y"}}, "ptr": Form{"p": "P"}},
		"bracket":   Form{"a": Form{"b": "B"}},
		"numkeys":   map[string]any{"123": "num", "456": 456, "789": 78.9},
		"empty":     Form{},
		"nilslice":  []any(nil),
		"deepNest":  Form{"a": Form{"b": Form{"c": "CCC"}}},
		"zeroValue": 0,
	}

	tests := []struct {
		expr   string
		want   any
		wantOk bool
	}{

		{"flat", "v0", true},
		{"[flat]", "v0", true},
		{"missing", nil, false},
		{"[missing]", nil, false},

		{"user->name", "alice", true},
		{"user->age", 30, true},
		{"profile->active", true, true},
		{"profile->missing", nil, false},

		{"items->0", "zero", true},
		{"items->1", "one", true},
		{"items->2", Form{"sub": "deep"}, true},
		{"items->3", nil, false},

		{"items->2->sub", "deep", true},
		{"items->2->missing", nil, false},

		{"matrix->0->0", 1, true},
		{"matrix->1->1", 4, true},
		{"matrix->1->2", nil, false},

		{"mixed->arr->0->x", "X", true},
		{"mixed->arr->1->x", "Y", true},
		{"mixed->ptr->p", "P", true},

		{"[bracket]->[a]->[b]", "B", true},

		{"numkeys->123", "num", true},
		{"numkeys->\"123\"", nil, false},
		{"numkeys->456", 456, true},
		{"numkeys->789", 78.9, true},

		{"empty->foo", nil, false},
		{"nilslice->0", nil, false},

		{"deepNest->a->b->c", "CCC", true},

		{"zeroValue", 0, true},

		{"", nil, false},
		{"->flat", nil, false},
		{"flat->", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			path := splitArrowPath(tt.expr)
			got, ok := GetValueByKeyPath(sample, path)
			if ok != tt.wantOk {
				t.Fatalf("expr %q: expected ok=%v, got %v", tt.expr, tt.wantOk, ok)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expr %q: expected value=%#v, got %#v", tt.expr, tt.want, got)
			}
		})
	}
}

func buildTestAST() Node {
	return Container("root",
		Field("username"),
		Choice("mode", false,
			Option("basic", Field("modeSimpleFlag")),
			Option("advanced", Field("modeAdvFlag")),
		),
		Choice("features", true,
			Option("f1", Field("feat1")),
			Option("f2", Field("feat2")),
			Option("f3", Field("feat3")),
		),
		Choice("singleSelect", false,
			Option("alpha", Field("selAlpha")),
			Option("beta", Field("selBeta")),
		),
	)
}

func TestKeyValue(t *testing.T) {
	astRoot := buildTestAST()

	tests := []struct {
		name     string
		form     Form
		expected map[string]any
	}{
		{
			name:     "empty form",
			form:     Form{},
			expected: map[string]any{},
		},
		{
			name: "only text",
			form: Form{
				"username": "alice",
			},
			expected: map[string]any{
				"username": "alice",
			},
		},
		{
			name: "radio basic",
			form: Form{
				"mode":           "basic",
				"modeSimpleFlag": "S",
			},
			expected: map[string]any{
				"mode":           "basic",
				"modeSimpleFlag": "S",
			},
		},
		{
			name: "radio advanced",
			form: Form{
				"mode":        "advanced",
				"modeAdvFlag": "A",
			},
			expected: map[string]any{
				"mode":        "advanced",
				"modeAdvFlag": "A",
			},
		},
		{
			name: "checkbox single",
			form: Form{
				"features": []string{"f2"},
				"feat2":    "2",
			},
			expected: map[string]any{
				"features": []string{"f2"},
				"feat2":    "2",
			},
		},
		{
			name: "checkbox multiple",
			form: Form{
				"features": []string{"f1", "f3"},
				"feat1":    "1",
				"feat3":    "3",
			},
			expected: map[string]any{
				"features": []string{"f1", "f3"},
				"feat1":    "1",
				"feat3":    "3",
			},
		},
		{
			name: "select single",
			form: Form{
				"singleSelect": "beta",
				"selBeta":      "B",
			},
			expected: map[string]any{
				"singleSelect": "beta",
				"selBeta":      "B",
			},
		},
		{
			name: "combined all",
			form: Form{
				"username":     "bob",
				"mode":         "advanced",
				"modeAdvFlag":  "ADV",
				"features":     []string{"f1", "f2"},
				"feat1":        "F1",
				"feat2":        "F2",
				"singleSelect": "alpha",
				"selAlpha":     "A",
			},
			expected: map[string]any{
				"username":     "bob",
				"mode":         "advanced",
				"modeAdvFlag":  "ADV",
				"features":     []string{"f1", "f2"},
				"feat1":        "F1",
				"feat2":        "F2",
				"singleSelect": "alpha",
				"selAlpha":     "A",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := astRoot.KeyValue(tt.form)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("KeyValue(%v) =\n  got: %#v\n want: %#v",
					tt.form, got, tt.expected)
			}
		})
	}
}

func buildNestedAST() Node {
	return Container("rootPage",
		Container("secA",
			Container("groupA",
				Container("step1",
					Field("f1"),
					Choice("r1", false,
						Option("A", Field("r1A")),
						Option("B", Field("r1B")),
					),
				),
				Container("step2",
					Choice("c1", true,
						Option("X", Field("c1X")),
						Option("Y", Field("c1Y")),
					),
				),
			),
			Container("groupB",
				Choice("s1", false,
					Option("opt1", Field("s1_1")),
					Option("opt2", Field("s1_2")),
				),
			),
		),
	)
}

func TestKeyValueNested(t *testing.T) {
	astRoot := buildNestedAST()

	tests := []struct {
		name     string
		form     Form
		expected map[string]any
	}{
		{
			name:     "empty form",
			form:     Form{},
			expected: map[string]any{},
		},
		{
			name: "only f1",
			form: Form{"f1": "v1"},
			expected: map[string]any{
				"f1": "v1",
			},
		},
		{
			name: "radio r1=B",
			form: Form{
				"r1":  "B",
				"r1B": "rb",
			},
			expected: map[string]any{
				"r1":  "B",
				"r1B": "rb",
			},
		},
		{
			name: "checkbox c1=[X,Y]",
			form: Form{
				"c1":  []string{"X", "Y"},
				"c1X": "cx",
				"c1Y": "cy",
			},
			expected: map[string]any{
				"c1":  []string{"X", "Y"},
				"c1X": "cx",
				"c1Y": "cy",
			},
		},
		{
			name: "select s1=opt2",
			form: Form{
				"s1":   "opt2",
				"s1_2": "s2",
			},
			expected: map[string]any{
				"s1":   "opt2",
				"s1_2": "s2",
			},
		},
		{
			name: "nodes",
			form: Form{
				"f1":   "v1",
				"r1":   "A",
				"r1A":  "ra",
				"c1":   []string{"Y"},
				"c1Y":  "cy",
				"s1":   "opt1",
				"s1_1": "s1v",
			},
			expected: map[string]any{
				"f1":   "v1",
				"r1":   "A",
				"r1A":  "ra",
				"c1":   []string{"Y"},
				"c1Y":  "cy",
				"s1":   "opt1",
				"s1_1": "s1v",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := astRoot.KeyValue(tt.form)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("KeyValue(%v) =\n  got: %#v\n want: %#v",
					tt.form, got, tt.expected)
			}
		})
	}
}

func buildArrowAST() Node {
	return Container("root",
		Field("user->name"),
		Field("[user]->[profile]->age"),
		Choice("[settings]->theme", false,
			Option("light", Field("settings->lightFlag")),
			Option("dark", Field("settings->darkFlag")),
		),
		Choice("prefs->options", true,
			Option("optA", Field("prefs->A")),
			Option("optB", Field("prefs->B")),
		),
	)
}

func TestArrowPathKeyValue(t *testing.T) {
	astRoot := buildArrowAST()

	form := Form{
		"user": Form{
			"name": "alice",
			"profile": Form{
				"age": 30,
			},
		},
		"settings": Form{
			"theme":    "dark",
			"darkFlag": "D",
		},
		"prefs": Form{
			"options": []string{"optA", "optB"},
			"A":       "VA",
			"B":       "VB",
		},
	}

	expected := map[string]any{
		"user->name":             "alice",
		"[user]->[profile]->age": 30,
		"[settings]->theme":      "dark",
		"settings->darkFlag":     "D",
		"prefs->options":         []string{"optA", "optB"},
		"prefs->A":               "VA",
		"prefs->B":               "VB",
	}

	got := astRoot.KeyValue(form)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("KeyValue with arrow paths =\n got: %#v\nwant: %#v", got, expected)
	}

	ast, err := NewAST(astRoot)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	selected := ast.Selected(form)
	wantSelected := []string{
		"user->name",
		"[user]->[profile]->age",
		"[settings]->theme",
		"settings->darkFlag",
		"prefs->options",
		"prefs->A",
		"prefs->B",
	}
	if !reflect.DeepEqual(selected, wantSelected) {
		t.Errorf("Selected(form) = %v, want %v", selected, wantSelected)
	}
}

func buildDeepArrowAST() Node {
	return Container("rootPage",
		Container("sec1",
			Container("group1",
				Container("step1",
					Field("user->id"),
					Field("user->profile->email"),
					Choice("settings->mode", false,
						Option("on", Field("settings->onFlag")),
						Option("off", Field("settings->offFlag")),
					),
				),
				Container("group2",
					Choice("prefs->[notifications]->types", true,
						Option("email", Field("prefs->notifications->emailFlag")),
						Option("sms", Field("prefs->notifications->smsFlag")),
					),
				),
			),
		),
	)
}

func TestDeepArrowKeyValue(t *testing.T) {
	astRoot := buildDeepArrowAST()

	form := Form{
		"user": Form{
			"id": "u123",
			"profile": Form{
				"email": "a@example.com",
			},
		},
		"settings": Form{
			"mode":    "off",
			"offFlag": true,
		},
		"prefs": Form{
			"notifications": Form{
				"types":     []string{"email", "sms"},
				"emailFlag": "E_OK",
				"smsFlag":   "S_OK",
			},
		},
	}

	expected := map[string]any{
		"user->id":                        "u123",
		"user->profile->email":            "a@example.com",
		"settings->mode":                  "off",
		"settings->offFlag":               true,
		"prefs->[notifications]->types":   []string{"email", "sms"},
		"prefs->notifications->emailFlag": "E_OK",
		"prefs->notifications->smsFlag":   "S_OK",
	}

	got := astRoot.KeyValue(form)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("KeyValue(form) =\n got: %#v\nwant: %#v", got, expected)
	}

	ast, err := NewAST(astRoot)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	selected := ast.Selected(form)

	wantSel := []string{
		"user->id",
		"user->profile->email",
		"settings->mode",
		"settings->offFlag",
		"prefs->[notifications]->types",
		"prefs->notifications->emailFlag",
		"prefs->notifications->smsFlag",
	}
	if !reflect.DeepEqual(selected, wantSel) {
		t.Errorf("Selected(form) = %v, want %v", selected, wantSel)
	}
}

func TestShortKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"username", "username"},
		{"user->name", "name"},
		{"[user]->[profile]->age", "age"},
		{"settings->mode", "mode"},
		{"prefs->options->[0]", "0"},
		{"multi->level->[deep]->leaf", "leaf"},
		{"onlyBracket->[foo]", "foo"},
		{"trailingBracket->bar]", "bar]"},
		{"[unclosed", "[unclosed"},
		{"", ""},
	}

	for _, c := range cases {
		if got := ShortKey(c.in); got != c.want {
			t.Errorf("ShortKey(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestChoiceNodeFieldsWithAnySlice(t *testing.T) {
	choice := Choice("colors", true,
		Option("red", Field("shade_red")),
		Option("blue", Field("shade_blue")),
	)

	form := Form{
		"colors":     []any{"red", 123, "blue"},
		"shade_red":  "dark",
		"shade_blue": "light",
	}

	fields := choice.Fields(form)
	expected := []string{"colors", "shade_red", "shade_blue"}
	found := make(map[string]bool)
	for _, f := range fields {
		found[f] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("Expected field %q in result, but missing; got fields: %v", name, fields)
		}
	}
}

func TestChoiceNodeKeyValueWithAnySlice(t *testing.T) {
	choice := Choice("items", true,
		Option("one", Field("val1")),
		Option("two", Field("val2")),
	)

	form := Form{
		"items": []any{"one", "two", nil, 3.14},
		"val1":  100,
		"val2":  200,
	}

	kv := choice.KeyValue(form)

	raw, ok := kv["items"].([]any)
	if !ok {
		t.Fatalf("Expected kv[\"items\"] to be []any, got %T", kv["items"])
	}

	expRaw := []any{"one", "two", nil, 3.14}
	if !reflect.DeepEqual(raw, expRaw) {
		t.Errorf("Raw slice mismatch: got %v, want %v", raw, expRaw)
	}

	if v1, ok := kv["val1"]; !ok || v1 != 100 {
		t.Errorf("Expected kv[\"val1\"]==100, got %v", kv["val1"])
	}
	if v2, ok := kv["val2"]; !ok || v2 != 200 {
		t.Errorf("Expected kv[\"val2\"]==200, got %v", kv["val2"])
	}
}

func TestParseExpr_ArithmeticPrecedence(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"10 / 2 + 3", 8},
		{"10 / (2 + 3)", 2},
		{"-5 + 2", -3},
		{"-(5 + 2)", -7},
	}
	for _, c := range cases {
		e, err := ParseExpr(c.expr)
		if err != nil {
			t.Fatalf("ParseExpr(%q) err=%v", c.expr, err)
		}
		v, err := e.Eval(Form{})
		if err != nil {
			t.Fatalf("Eval(%q) err=%v", c.expr, err)
		}
		got, _ := toFloat(v)
		if got != c.want {
			t.Errorf("Eval(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestParseExpr_LogicalAndCompare(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"1 < 2 && 3 > 1", true},
		{"1 < 2 && 3 < 1", false},
		{"1 == 1 || 2 == 3", true},
		{"!(2 > 1)", false},
		{"(2 > 1) && !(3 == 4)", true},
	}
	for _, c := range cases {
		e, err := ParseExpr(c.expr)
		if err != nil {
			t.Fatalf("ParseExpr(%q) err=%v", c.expr, err)
		}
		got, err := EvalBool(e, Form{})
		if err != nil {
			t.Fatalf("EvalBool(%q) err=%v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("EvalBool(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestParseExpr_Functions(t *testing.T) {
	RegisterFunc("years_since", func(args []any) (any, error) {
		if len(args) != 1 {
			return float64(0), fmt.Errorf("years_since(x): invalid number of arguments: %d", len(args))
		}

		v := args[0]

		if v == nil {
			return float64(0), fmt.Errorf("years_since(x): x is nil")
		}

		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			return float64(0), fmt.Errorf("years_since(x): empty date string")
		}

		var tm time.Time
		var parsed bool

		switch val := v.(type) {
		case time.Time:
			tm = val
			parsed = true

		case string:
			timeValue := val
			layouts := []string{
				time.RFC3339,
				"2006-01-02",
				"2006/01/02",
				"2006-01-02 15:04:05",
			}
			for _, ly := range layouts {
				if t, err := time.Parse(ly, timeValue); err == nil {
					tm = t
					parsed = true
					break
				}
			}

		default:
			return float64(0), fmt.Errorf("years_since(x): unsupported type %T", v)
		}

		if !parsed {
			return float64(0), fmt.Errorf("years_since(x): failed to parse date from %v", v)
		}

		now := time.Now()
		y := now.Year() - tm.Year()
		if now.YearDay() < tm.YearDay() {
			y--
		}
		return float64(y), nil
	})
	e1, _ := ParseExpr(`int(3.7)`)
	v1, _ := e1.Eval(Form{})
	if f, _ := toFloat(v1); f != 3 {
		t.Errorf("int(3.7) = %v, want 3", f)
	}

	e2, _ := ParseExpr(`float("2.5")`)
	v2, _ := e2.Eval(Form{})
	if f, _ := toFloat(v2); f != 2.5 {
		t.Errorf(`float("2.5") = %v, want 2.5`, f)
	}

	e3, _ := ParseExpr(`years_since("2000-01-02")`)
	v3, _ := e3.Eval(Form{})

	if date := v3.(float64); date < 25 {
		t.Errorf(`invalid years_since result: %v`, date)
	}
}

func TestParseExpr_PathLookup(t *testing.T) {
	form := Form{
		"a": Form{
			"b": []any{
				Form{"c": 42},
			},
		},
	}
	e, err := ParseExpr(`[a]->[b]->0->[c]`)
	if err != nil {
		t.Fatalf("ParseExpr err=%v", err)
	}
	v, err := e.Eval(form)
	if err != nil {
		t.Fatalf("Eval err=%v", err)
	}
	if f, _ := toFloat(v); f != 42 {
		t.Errorf("path eval = %v, want 42", f)
	}

	paths := e.CollectPaths()
	if len(paths) != 1 || paths[0] != `[a]->[b]->0->[c]` {
		t.Errorf("CollectPaths = %#v, want single path", paths)
	}
}

func TestParseExpr_UnaryNotAndNegative(t *testing.T) {
	e, _ := ParseExpr(`!false`)
	b, _ := EvalBool(e, Form{})
	if b != true {
		t.Errorf("!false = %v, want true", b)
	}
	e2, _ := ParseExpr(`-5 + 2`)
	v2, _ := e2.Eval(Form{})
	if f, _ := toFloat(v2); f != -3 {
		t.Errorf("-5 + 2 = %v, want -3", f)
	}
}

func TestParseExpr_DivByZero(t *testing.T) {
	e, _ := ParseExpr(`10 / 0`)
	v, _ := e.Eval(Form{})
	if f, _ := toFloat(v); f != 0 {
		t.Errorf("10/0 = %v, want 0", f)
	}
}

func TestParseExpr_NowAndYearIntegration(t *testing.T) {
	form := Form{"step": Form{"A": "2023-01-01"}}
	e, _ := ParseExpr(`years_since([step]->A) >= 1`)
	ok, _ := EvalBool(e, form)
	if !ok {
		t.Errorf("condition should be true for 2023-01-01")
	}
}

func sliceHasAll(got []string, want ...string) bool {
	set := map[string]bool{}
	for _, s := range got {
		set[s] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func TestIfNode_ThenElseSelection(t *testing.T) {
	root := Container("Root",
		If(`years_since([step]->[A]) >= 1`,
			Container("Then", Field("[more]->[X]")),
			Container("Else", Field("[more]->[Y]")),
		),
	)
	astTree, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST err=%v", err)
	}

	// True branch
	formTrue := Form{
		"step": Form{"A": "2024-08-10"},
		"more": Form{"X": "then!", "Y": "else!"},
	}
	selTrue := astTree.Selected(formTrue)
	if !sliceHasAll(selTrue, "[step]->[A]", "[more]->[X]") {
		t.Errorf("Selected(true) = %#v, want include [step]->[A] and [more]->[X]", selTrue)
	}
	kvTrue := astTree.KeyValue(formTrue)
	if v, ok := kvTrue["[more]->[X]"]; !ok || v != "then!" {
		t.Errorf("KeyValue(true) = %#v, want X=then!", kvTrue)
	}

	// False branch
	formFalse := Form{
		"step": Form{"A": "2025-05-05"},
		"more": Form{"X": "then!", "Y": "else!"},
	}
	selFalse := astTree.Selected(formFalse)
	if !sliceHasAll(selFalse, "[step]->[A]", "[more]->[Y]") {
		t.Errorf("Selected(false) = %#v, want include [step]->[A] and [more]->[Y]", selFalse)
	}
	kvFalse := astTree.KeyValue(formFalse)
	if v, ok := kvFalse["[more]->[Y]"]; !ok || v != "else!" {
		t.Errorf("KeyValue(false) = %#v, want Y=else!", kvFalse)
	}
}

func TestIfNode_AllFieldsAndFieldsContainExprPaths(t *testing.T) {
	root := Container("Root",
		If(`now() - year([step]->[A]) > 1`,
			Container("Then", Field("[more]->[X]")),
			Container("Else", Field("[more]->[Y]")),
		),
	)
	astTree, _ := NewAST(root)

	all := astTree.AllFields()
	wantAll := []string{"[step]->[A]", "[more]->[X]", "[more]->[Y]"}
	for _, w := range wantAll {
		found := false
		for _, g := range all {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllFields missing %q, got %#v", w, all)
		}
	}

	form := Form{"step": Form{}, "more": Form{"Y": "ok"}}
	fields := astTree.Selected(form)
	if !sliceHasAll(fields, "[step]->[A]", "[more]->[Y]") {
		t.Errorf("Selected = %#v, want include expr path and selected branch field", fields)
	}
}

func TestIfNode_WithChoiceIntegration(t *testing.T) {
	root := Container("Root",
		Choice("type", false,
			Option("A",
				Field("[step]->[A]"),
				If(`years_since([step]->[A]) >= 1`,
					Container("ThenA", Field("[more]->[X]")),
					Container("ElseA", Field("[more]->[Y]")),
				),
			),
			Option("B",
				Field("[step]->[B]"),
			),
		),
	)

	astTree, _ := NewAST(root)
	form := Form{
		"type": "A",
		"step": Form{"A": "2024-08-20"},
		"more": Form{"X": "hitX", "Y": "hitY"},
	}
	sel := astTree.Selected(form)
	if !sliceHasAll(sel, "type", "[step]->[A]", "[more]->[X]") {
		t.Errorf("Selected = %#v, want include type, [step]->[A], [more]->[X]", sel)
	}
	kv := astTree.KeyValue(form)
	if kv["type"] != "A" || kv["[more]->[X]"] != "hitX" {
		t.Errorf("KeyValue = %#v, want type=A and [more]->X=hitX", kv)
	}
}

func TestGetValueByKeyPath_IndexAndMap(t *testing.T) {
	form := Form{
		"arr": []any{
			Form{"k": "v0"},
			Form{"k": "v1"},
		},
		"map": Form{"0": "zero", "k": "val"},
	}
	if v, ok := GetValueByKeyPath(form, splitArrowPath("arr->1->k")); !ok || v != "v1" {
		t.Errorf("GetValueByKeyPath arr->1->k = %v,%v want v1,true", v, ok)
	}
	if v, ok := GetValueByKeyPath(form, splitArrowPath("map->0")); !ok || v != "zero" {
		t.Errorf("GetValueByKeyPath map->0 = %v,%v want zero,true", v, ok)
	}
}

func TestShortKey1(t *testing.T) {
	if got := ShortKey("[a]->[b]->0->c"); got != "c" {
		t.Errorf("ShortKey = %q, want c", got)
	}
	if got := ShortKey("[x]"); got != "x" {
		t.Errorf("ShortKey = %q, want x", got)
	}
	if got := ShortKey("  foo  "); got != "foo" {
		t.Errorf("ShortKey = %q, want foo", got)
	}
}

func TestValidateNoCycles1(t *testing.T) {
	c := &ContainerNode{Label: "Loop"}
	c.ChildrenNodes = []Node{c}
	if err := ValidateNoCycles(c); err == nil {
		t.Errorf("ValidateNoCycles should detect cycle")
	}

	ok := Container("Root", Field("a"), Field("b"))
	if err := ValidateNoCycles(ok); err != nil {
		t.Errorf("ValidateNoCycles non-cyclic err=%v", err)
	}
}

func TestTreePrinter_Golden(t *testing.T) {
	root := Container("Root",
		Field("[step]->A"),
		If(`now() - year([step]->A) > 1`,
			Container("Then", Field("[more]->X")),
			Container("Else", Field("[more]->Y")),
		),
	)
	var buf bytes.Buffer
	p := NewTreePrinter(&buf)
	if err := p.Print(root); err != nil {
		t.Fatalf("Print err=%v", err)
	}
	got := buf.String()
	wantContains := []string{
		`(Container) name="Root"`,
		`├── (Field) field="[step]->A"`,
		`└── (If) expr="now() - year([step]->A) > 1"`,
		`    ├── (Container) name="Then"`,
		`    │   └── (Field) field="[more]->X"`,
		`    └── (Container) name="Else"`,
		`        └── (Field) field="[more]->Y"`,
	}
	for _, w := range wantContains {
		if !strings.Contains(got, w) {
			t.Errorf("printer output missing:\n%s\n--- got ---\n%s", w, got)
		}
	}
}

func TestAllFields_StableCopy(t *testing.T) {
	root := Container("Root", Field("a"), If("true", Field("b"), Field("c")))
	astTree, _ := NewAST(root)
	all := astTree.AllFields()
	cp := astTree.AllFields()
	if &all[0] == &cp[0] {
		t.Errorf("AllFields should return a copy, not same backing array")
	}
}

func TestIfNode_CollectPaths_NoExistStillIncludedInFields(t *testing.T) {
	root := If(`year([missing]->[X]) > 2000`, Field("a"), Field("b"))
	astTree, _ := NewAST(root)
	fields := astTree.Selected(Form{})
	if !sliceHasAll(fields, "[missing]->[X]") {
		t.Errorf("Selected should include expr path even if value missing, got %#v", fields)
	}
}

func TestChoice_MultipleSelections_WithIf(t *testing.T) {
	root := Choice("choices", true,
		Option("foo", If(`years_since([step]->[A]) > 1`, Field("X1"), Field("Y1"))),
		Option("bar", If(`years_since([step]->[A]) <= 1`, Field("X2"), Field("Y2"))),
	)
	astTree, _ := NewAST(root)
	form := Form{
		"choices": []any{"foo", "bar"},
		"step":    Form{"A": time.Now().Format("2006/01/02")},
		"X1":      "hit",
		"X2":      "miss",
		"Y1":      "miss",
		"Y2":      "miss",
	}
	sel := astTree.Selected(form)
	if !sliceHasAll(sel, "choices", "Y1", "X2") {
		t.Errorf("Selected = %#v, want include choices X1 Y2", sel)
	}
}

func snapshotRegistry() map[string]CustomFunc {
	funcRegistryMutex.Lock()
	defer funcRegistryMutex.Unlock()

	cp := make(map[string]CustomFunc, len(funcRegistry))
	for k, v := range funcRegistry {
		cp[k] = v
	}
	return cp
}

func restoreRegistry(snapshot map[string]CustomFunc) {
	funcRegistryMutex.Lock()
	defer funcRegistryMutex.Unlock()

	funcRegistry = make(map[string]CustomFunc, len(snapshot))
	for k, v := range snapshot {
		funcRegistry[k] = v
	}
}

func withRegistrySnapshot(t *testing.T, fn func()) {
	t.Helper()
	snap := snapshotRegistry()
	defer restoreRegistry(snap)
	fn()
}

func TestRegisterFunc_ConcurrentDistinctNames(t *testing.T) {
	withRegistrySnapshot(t, func() {
		const N = 500

		var wg sync.WaitGroup
		wg.Add(N)

		for i := 0; i < N; i++ {
			i := i
			go func() {
				defer wg.Done()

				name := fmt.Sprintf("F_%d", i)
				RegisterFunc(name, func(args []any) (any, error) {
					return float64(i), nil
				})
			}()
		}

		wg.Wait()

		for i := 0; i < N; i++ {
			name := fmt.Sprintf("f_%d", i)
			got, err := evalCall(name, nil, nil)
			if err != nil {
				t.Fatalf("evalCall(%q) error: %v", name, err)
			}
			if got != float64(i) {
				t.Fatalf("evalCall(%q)=%v, want %v", name, got, float64(i))
			}
		}
	})
}

func TestRegisterFunc_ConcurrentSameName_NoPanic(t *testing.T) {
	withRegistrySnapshot(t, func() {
		const N = 200

		var wg sync.WaitGroup
		wg.Add(N)

		for i := 0; i < N; i++ {
			i := i
			go func() {
				defer wg.Done()
				RegisterFunc("SAME_NAME", func(args []any) (any, error) {
					return float64(i), nil
				})
			}()
		}
		wg.Wait()

		got, err := evalCall("same_name", nil, nil)
		if err != nil {
			t.Fatalf("evalCall error: %v", err)
		}
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("unexpected type: %T (%v)", got, got)
		}
		if f < 0 || f >= float64(N) {
			t.Fatalf("got %v out of range [0,%d)", f, N)
		}
	})
}

func TestRegisterAndEval_Concurrent_UnsafeDemo(t *testing.T) {
	//t.Skip("Unskip to reproduce race/panic: evalCall reads funcRegistry without lock; run with `go test -race`")

	withRegistrySnapshot(t, func() {
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < 200000; i++ {
				RegisterFunc("RACE_FN", func(args []any) (any, error) { return float64(i), nil })
			}
			close(stop)
		}()

		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = evalCall("race_fn", nil, nil)
				}
			}
		}()

		wg.Wait()
	})
}

type failWriter struct{}

func (f *failWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write failed")
}

func TestTreePrinter_WriteError(t *testing.T) {
	// Test that Print propagates errors from the io.Writer
	node := Field("test")
	printer := NewTreePrinter(&failWriter{})
	err := printer.Print(node)
	if err == nil || err.Error() != "write failed" {
		t.Errorf("Expected 'write failed' error, got %v", err)
	}
}

func TestIfNode_NilBranches(t *testing.T) {
	// Construct an IfNode manually with nil Then/Else to ensure no panic
	node := &IfNode{
		ExprSrc: "true",
		Expr:    &BoolExpr{Value: true},
		Then:    nil, // Explicitly nil
		Else:    nil, // Explicitly nil
	}

	// 1. Test Fields (Should be safe)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("IfNode.Fields panicked on nil children: %v", r)
		}
	}()
	fields := node.Fields(Form{})
	if len(fields) != 0 {
		t.Errorf("Expected 0 fields, got %v", fields)
	}

	// 2. Test KeyValue (Should be safe)
	kv := node.KeyValue(Form{})
	if len(kv) != 0 {
		t.Errorf("Expected empty KV, got %v", kv)
	}

	// 3. Test Children (Should return empty slice, not contain nils)
	children := node.Children()
	if len(children) != 0 {
		t.Errorf("Expected 0 children, got %d", len(children))
	}
}

func TestChoiceNode_DataMismatch(t *testing.T) {
	// Case: Multiple=true, but Form contains a simple string (not slice)
	// The implementation currently has a check for this, let's verify it acts as expected (ignores or handles).
	chk := Choice("tags", true,
		Option("a", Field("sub_a")),
	)

	form := Form{
		"tags":  "invalid_string_instead_of_slice",
		"sub_a": "val_a",
	}

	// Should not panic, should probably return just the main field, or empty selection for children
	fields := chk.Fields(form)
	expected := []string{"tags"} // It should at least return itself
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("Choice(Multiple=true) with string data: got %v, want %v", fields, expected)
	}

	kv := chk.KeyValue(form)
	if len(kv) != 1 || kv["tags"] != "invalid_string_instead_of_slice" {
		t.Errorf("KeyValue mismatch on invalid data: %v", kv)
	}
}

func TestEval_NoShortCircuit(t *testing.T) {
	// The current implementation evaluates BOTH left and right before checking && / ||.
	// This test confirms that behavior (or fails if you optimize it later).
	// "false && (1/0)" -> If short-circuiting, this is false. If eager, this is division by zero error.

	expr, err := ParseExpr("false && (1 / 0)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	_, err = expr.Eval(Form{})
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("Expected division by zero error (eager evaluation), got: %v", err)
	}
}

func TestEval_TypeCoercion(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		// String + Number
		// "foo" fails to parse, so toFloat returns false.
		// evalInfix sees !lok and returns 0 immediately (aborts operation).
		{`"foo" + 5`, "foo5"},

		// "10" parses successfully as 10.0, so 10 + 5 = 15
		{`"10" + 5`, 15.0},

		// Comparison degeneracy
		{`"a" == "a"`, true},
		{`"a" == "b"`, false},
		{`"a" != "b"`, true},

		// Unary minus on string "hello" fails to parse, returns 0.0
		{`-"hello"`, 0.0},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEval_CustomFuncError(t *testing.T) {
	// Test propagation of errors from custom functions
	funcName := "error_trigger"
	RegisterFunc(funcName, func(args []any) (any, error) {
		return nil, errors.New("custom boom")
	})

	expr, _ := ParseExpr("error_trigger()")
	_, err := expr.Eval(Form{})
	if err == nil || err.Error() != "custom boom" {
		t.Errorf("Expected 'custom boom', got %v", err)
	}
}

func TestLexer_ScientificNotation(t *testing.T) {
	// The lexer implementation handles 'e'/'E'. Let's verify.
	tests := []struct {
		input string
		want  float64
	}{
		{"1.5e2", 150},
		{"1e+2", 100},
		{"1E-1", 0.1},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.input)
		if err != nil {
			t.Errorf("ParseExpr(%q) error: %v", tt.input, err)
			continue
		}
		res, _ := e.Eval(Form{})
		if f, ok := toFloat(res); !ok || f != tt.want {
			t.Errorf("Lexing scientific %q = %v, want %v", tt.input, res, tt.want)
		}
	}
}

func TestParser_Errors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		errSnippet string
	}{
		{"Unterminated string", `"hello`, "unterminated string"},
		{"Unterminated path", `[user`, "unterminated [path] key"},
		{"Invalid token", `@`, "unexpected token"},          // @ is not handled
		{"Unbalanced parens", `(1 + 2`, "unexpected token"}, // EOF instead of )
		{"Missing operand", `1 +`, "unexpected token"},
		{"Bad path segment", `path -> "quoted"`, "invalid path segment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseExpr(tt.input)
			if err == nil {
				t.Errorf("Expected error for %q, got nil", tt.input)
			} else if !strings.Contains(err.Error(), tt.errSnippet) && !strings.Contains(err.Error(), "expect") {
				// "expect" covers p.expect() errors
				t.Errorf("Error for %q was %q, expected containing %q", tt.input, err.Error(), tt.errSnippet)
			}
		})
	}
}

// --- 6. Manual Struct Construction Safety ---

func TestPrefixExpr_UnknownOp(t *testing.T) {
	// Impossible via parser, but possible via struct literal
	e := &PrefixExpr{Op: "???", Right: &NumberExpr{Value: 1}}
	_, err := e.Eval(Form{})
	if err == nil || !strings.Contains(err.Error(), "unknown prefix op") {
		t.Errorf("Expected unknown prefix op error, got %v", err)
	}
}

func TestInfixExpr_UnknownOp(t *testing.T) {
	// Impossible via parser, but possible via struct literal
	e := &InfixExpr{Left: &NumberExpr{Value: 1}, Op: "???", Right: &NumberExpr{Value: 1}}
	_, err := e.Eval(Form{})
	if err == nil || !strings.Contains(err.Error(), "unknown infix op") {
		t.Errorf("Expected unknown infix op error, got %v", err)
	}
}

func TestEval_StringComparison(t *testing.T) {
	// evalInfix falls back to string comparison if operands are not floats.
	// We need to verify that "b" > "a" works.
	tests := []struct {
		expr string
		want bool
	}{
		{`"b" > "a"`, true},
		{`"a" > "b"`, false},
		{`"a" < "b"`, true},
		{`"a" >= "a"`, true},
		{`"b" <= "a"`, false},
		// Compare mixed types (converts to string)
		{`"2" > 1`, true}, // "2" > "1" is true
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := EvalBool(e, Form{})
		if err != nil {
			t.Errorf("EvalBool(%q) failed: %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("EvalBool(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestGetValueByKeyPath_SpecificTypedSlices(t *testing.T) {
	// The implementation has specific switch cases for []string and []map[string]any.
	// We must ensure these specific types (not just []any) are handled.

	// 1. Test []string
	formStringSlice := Form{
		"tags": []string{"tagA", "tagB"},
	}
	v, ok := GetValueByKeyPath(formStringSlice, splitArrowPath("tags->1"))
	if !ok || v != "tagB" {
		t.Errorf("GetValueByKeyPath with []string failed, got %v, %v", v, ok)
	}

	// 2. Test []map[string]any
	formMapSlice := Form{
		"users": []map[string]any{
			{"name": "alice"},
			{"name": "bob"},
		},
	}
	v2, ok2 := GetValueByKeyPath(formMapSlice, splitArrowPath("users->0->name"))
	if !ok2 || v2 != "alice" {
		t.Errorf("GetValueByKeyPath with []map[string]any failed, got %v, %v", v2, ok2)
	}

	// 3. Test Negative Index (should fail gracefully)
	v3, ok3 := GetValueByKeyPath(formStringSlice, splitArrowPath("tags->-1"))
	if ok3 {
		t.Errorf("Expected negative index to return false, got value %v", v3)
	}
}

func TestBoolLikeness(t *testing.T) {
	// toBool supports "yes", "y", "1".
	tests := []struct {
		val  any
		want bool
	}{
		{"yes", true},
		{"YES", true},
		{"y", true},
		{"1", true},
		{"no", false},
		{"0", false},
		{"random", false},
	}

	for _, tt := range tests {
		if got := toBool(tt.val); got != tt.want {
			t.Errorf("toBool(%v) = %v, want %v", tt.val, got, tt.want)
		}
	}
}

func TestIfNode_ErrorSwallowing(t *testing.T) {
	// If an expression errors (e.g. division by zero), IfNode treats it as false (Else branch).
	// It should NOT panic or return the error up the stack during Fields/KeyValue extraction.

	// Expression: 1 / 0 (Division by zero error)
	node := If("1 / 0", Field("IsTrue"), Field("IsFalse"))

	// 1. Test Fields
	// FIX: Provide a form containing "IsFalse" so FieldNode.Fields returns it.
	form := Form{"IsFalse": "exists"}
	fields := node.Fields(form)

	if len(fields) != 1 || fields[0] != "IsFalse" {
		t.Errorf("Fields() with error expr should select Else branch, got %v", fields)
	}

	// 2. Test KeyValue
	form2 := Form{"IsTrue": "T", "IsFalse": "F"}
	kv := node.KeyValue(form2)
	if kv["IsFalse"] != "F" {
		t.Errorf("KeyValue() with error expr should select Else branch, got %v", kv)
	}
}

func TestChoice_MixedTypeFiltering(t *testing.T) {
	// In Multiple mode, we iterate []any. If an element is NOT a string, it should be skipped.
	choice := Choice("items", true,
		Option("valid", Field("F_Valid")),
	)

	// Form has "valid" string, but also an int and a bool
	form := Form{
		"items":   []any{123, "valid", true},
		"F_Valid": "found",
	}

	// FIX: Use .Fields(form) instead of .Selected(form)
	sel := choice.Fields(form)

	if len(sel) != 2 { // ["items", "F_Valid"]
		t.Errorf("Expected 2 selected fields, got %v", sel)
	}
	if !sliceHasAll(sel, "items", "F_Valid") {
		t.Errorf("Failed to filter mixed types in choice array. Got %v", sel)
	}
}
func TestNilFormSafety(t *testing.T) {
	// Verify that calling Fields(nil) works safely on all node types
	nodes := []Node{
		Field("f"),
		Choice("c", false, Option("o", Field("sub"))),
		Container("g", Field("child")),
		If("true", Field("then"), Field("else")),
	}

	for i, n := range nodes {
		f := n.Fields(nil)
		if len(f) != 0 {
			t.Errorf("Node %d: Expected empty fields for nil form, got %v", i, f)
		}
	}
}

func TestCallExpr_CollectPaths(t *testing.T) {
	// Verify that function arguments are recursed for path collection
	expr, err := ParseExpr(`myFunc([path]->[A], [path]->[B])`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	paths := expr.CollectPaths()
	expected := []string{"[path]->[A]", "[path]->[B]"}

	if len(paths) != 2 {
		t.Errorf("CollectPaths failed, got %v", paths)
	}
	// Order isn't guaranteed by unique(), so check existence
	pMap := make(map[string]bool)
	for _, p := range paths {
		pMap[p] = true
	}
	for _, e := range expected {
		if !pMap[e] {
			t.Errorf("Missing path %q", e)
		}
	}
}

func TestLexer_StartingDot(t *testing.T) {
	// Code handles: case unicode.IsDigit(r) || (r == '.' && unicode.IsDigit(l.peek())):
	// We need to test .5
	e, err := ParseExpr(".5 + .5")
	if err != nil {
		t.Fatalf("Parse .5 failed: %v", err)
	}
	v, _ := e.Eval(Form{})
	if f, _ := toFloat(v); f != 1.0 {
		t.Errorf(".5 + .5 = %v, want 1.0", f)
	}
}

func TestEval_StringConcatenation(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{`"Hello" + " " + "World"`, "Hello World"},

		{`"User id: " + 42`, "User id: 42"},

		{`100 + " is the score"`, "100 is the score"},

		{`"Level " + 2.5`, "Level 2.5"},

		{`"Is active? " + true`, "Is active? true"},

		{`"Total: " + (10 + 5)`, "Total: 15"},

		// "Res: " + 10 -> "Res: 10"
		// "Res: 10" + 5 -> "Res: 105"
		{`"Res: " + 10 + 5`, "Res: 105"},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}

		s, ok := got.(string)
		if !ok {
			t.Errorf("Eval(%q) returned type %T, want string", tt.expr, got)
			continue
		}

		if s != tt.want {
			t.Errorf("Eval(%q) = %q, want %q", tt.expr, s, tt.want)
		}
	}
}

func TestEval_Modulo(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{

		{"10 % 3", 1},
		{"10 % 2", 0},

		{"5.5 % 2", 1.5},

		// 2 + 10 % 3 => 2 + 1 => 3
		{"2 + 10 % 3", 3},

		// 10 % 4 % 3 => 2 % 3 => 2
		{"10 % 4 % 3", 2},

		// -10 % 4 % 3 => -2 % 3 => -2
		{"-10 % 4 % 3", -2},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}

		f, ok := toFloat(got)
		if !ok {
			t.Errorf("Eval(%q) result not number: %v", tt.expr, got)
			continue
		}

		if f != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, f, tt.want)
		}
	}
}

func TestEval_Bitwise(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		// AND
		{"5 & 3", 1}, // 101 & 011 = 001
		// OR
		{"5 | 3", 7}, // 101 | 011 = 111
		// XOR
		{"5 ^ 3", 6}, // 101 ^ 011 = 110
		// Shift Left
		{"1 << 2", 4}, // 1 * 2^2
		// Shift Right
		{"8 >> 2", 2}, // 8 / 2^2
		{"5 & 3 + 1", 4},
		{"5 | 4 == 5", 1},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}

		f, ok := toFloat(got)
		if !ok || f != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEval_BitwiseNot(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		// ~0 = -1
		{"~0", -1},

		{"~1", -2},

		// ~5 = -6
		{"~5", -6},

		{"~5 + 1", -5},

		// ~~5 = 5
		{"~~5", 5},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}

		f, ok := toFloat(got)
		if !ok || f != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEval_Ternary(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		{"true ? 10 : 20", 10.0},
		{"false ? 10 : 20", 20.0},

		// (1==1) ? "yes" : "no"
		{"1==1 ? \"yes\" : \"no\"", "yes"},

		// true ? 1+1 : 0  => 2
		{"true ? 1+1 : 0", 2.0},

		// true ? 1 : true ? 2 : 3  =>  true ? 1 : (true ? 2 : 3) => 1
		{"true ? 1 : true ? 2 : 3", 1.0},
		// false ? 1 : true ? 2 : 3 =>  false ? 1 : (true ? 2 : 3) => 2
		{"false ? 1 : true ? 2 : 3", 2.0},

		{"true ? 100 : 1/0", 100.0},
		{"false ? 1/0 : 200", 200.0},
	}

	for _, tt := range tests {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Errorf("ParseExpr(%q) failed: %v", tt.expr, err)
			continue
		}
		got, err := e.Eval(Form{})
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", tt.expr, err)
			continue
		}

		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tt.want) {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}
