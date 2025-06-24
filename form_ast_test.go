package ast

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
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
	step := Step("S")
	group := Group("G", step)
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
		Option("x", Text("fX")),
		Option("y", Text("fY")),
	}
	r := Radio("mode", radioOpts...)
	root := Group("Root", Text("u"), r)
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
	chk := Checkbox("opts",
		Option("a", Text("fA")),
		Option("b", Text("fB")),
		Option("c", Text("fC")),
	)
	root := Group("ChkRoot", chk)
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
	sel1 := Select("sel1",
		Option("x", Text("fX1")),
		Option("y", Text("fY1")),
	)
	sel2 := Select("sel2",
		Option("m", Text("fM2")),
		Option("n", Text("fN2")),
	)
	root := Group("SelRoot", sel1, sel2)
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

func TestNestedSteps(t *testing.T) {
	t.Parallel()
	inner := Step("Inner", Text("fI"))
	mid := Step("Mid", inner, Text("fM"))
	outer := Step("Outer", mid, Text("fO"))
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

func TestPrintNoSelection(t *testing.T) {
	root := Group("Empty", Text("fX"))
	ast, err := NewAST(root)
	if err != nil {
		t.Fatalf("NewAST error: %v", err)
	}
	buf := &bytes.Buffer{}
	err = ast.Print(buf, Form{"other": "v"})
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Expected no output, got %q", buf.String())
	}
}

func TestTreePrinterOutputOrder(t *testing.T) {
	root := Group("L1",
		Group("L2", Text("a")),
		Text("b"),
	)
	ast, _ := NewAST(root)
	buf := &bytes.Buffer{}
	ast.Print(buf, Form{"a": "1", "b": "2"})
	out := buf.String()
	fmt.Println(">>>>>>>")
	fmt.Println(buf.String())
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("Expected 4 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "(Group) label=\"L1\"") {
		t.Errorf("Line1 = %s", lines[0])
	}
	if !strings.Contains(lines[1], "(Group) label=\"L2\"") {
		t.Errorf("Line2 = %s", lines[1])
	}
	if !strings.Contains(lines[2], "(Text) name=\"a\" value=\"1\"") {
		t.Errorf("Line3 = %s", lines[2])
	}
	if !strings.Contains(lines[3], "(Text) name=\"b\" value=\"2\"") {
		t.Errorf("Line4 = %s", lines[3])
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
			root = Step(label, Text(field))
		} else {
			root = Step(label, root, Text(field))
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

// Test a large mixed AST covering Text, Radio, Checkbox, Select, Step, Group in parallel
// Test a large mixed AST covering Text, Radio, Checkbox, Select, Step, Group in parallel
func TestMixedLargeAST_FieldsParallel(t *testing.T) {
	t.Parallel()
	// Build mixed nodes
	var texts []Node
	for i := 0; i < 50; i++ {
		texts = append(texts, Text(fmt.Sprintf("T%d", i)))
	}
	// Build Grp1 manually to avoid slice expansion
	grp1 := Group("Grp1")
	for i := 0; i < 10; i++ {
		grp1.ChildrenNodes = append(grp1.ChildrenNodes, texts[i])
	}
	radio := Radio("mode",
		Option("one", Text("R1")),
		Option("two", Text("R2")),
		Option("three", Text("R3")),
	)
	checkbox := Checkbox("opts",
		Option("a", Text("C1")),
		Option("b", Text("C2")),
		Option("c", Text("C3")),
	)
	selectNode := Select("sel",
		Option("alpha", Text("S1")),
		Option("beta", Text("S2")),
	)
	step := Step("StepRoot", grp1, radio, checkbox, selectNode)
	// Build RootMixed manually
	rootMixed := Group("RootMixed", step)
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

	root := Group("Form A",
		Step("dashboard",
			Text("username"),
			Radio("mode",
				Option("basic",
					Text("modeSimpleFlag"),
				),
				Option("advanced",
					Text("modeAdvFlag"),
					Checkbox("features",
						Option("f1", Text("feat1")),
						Option("f2", Text("feat2")),
						Option("f3", Text("feat3")),
					),
				),
			),
			Checkbox("options",
				Option("o1",
					Select("singleSelect",
						Option("alpha", Text("selAlpha")),
						Option("beta", Text("selBeta")),
					),
				),
				Option("o2", Text("opt2Only")),
				Option("o3", Text("opt3Only")),
			),
		),
		Step("profile",
			Text("level"),
		),
	)

	ast, err := NewAST(root)
	if err != nil {
		panic(err)
	}

	fmt.Println("All fields:", ast.AllFields())
	fmt.Println("selected fields:", ast.Selected(form))

	ast.Print(os.Stdout, nil)
	ast.Print(os.Stdout, form)

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
	return Group("root",
		Text("username"),
		Radio("mode",
			Option("basic", Text("modeSimpleFlag")),
			Option("advanced", Text("modeAdvFlag")),
		),
		Checkbox("features",
			Option("f1", Text("feat1")),
			Option("f2", Text("feat2")),
			Option("f3", Text("feat3")),
		),
		Select("singleSelect",
			Option("alpha", Text("selAlpha")),
			Option("beta", Text("selBeta")),
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
	return Page("rootPage",
		Section("secA",
			Group("groupA",
				Step("step1",
					Text("f1"),
					Radio("r1",
						Option("A", Text("r1A")),
						Option("B", Text("r1B")),
					),
				),
				Step("step2",
					Checkbox("c1",
						Option("X", Text("c1X")),
						Option("Y", Text("c1Y")),
					),
				),
			),
			Group("groupB",
				Select("s1",
					Option("opt1", Text("s1_1")),
					Option("opt2", Text("s1_2")),
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
	return Group("root",
		Text("user->name"),
		Text("[user]->[profile]->age"),
		Radio("[settings]->theme",
			Option("light", Text("settings->lightFlag")),
			Option("dark", Text("settings->darkFlag")),
		),
		Checkbox("prefs->options",
			Option("optA", Text("prefs->A")),
			Option("optB", Text("prefs->B")),
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
	return Page("rootPage",
		Section("sec1",
			Group("group1",
				Step("step1",
					Text("user->id"),
					Text("user->profile->email"),
					Radio("settings->mode",
						Option("on", Text("settings->onFlag")),
						Option("off", Text("settings->offFlag")),
					),
				),
				Group("group2",
					Checkbox("prefs->[notifications]->types",
						Option("email", Text("prefs->notifications->emailFlag")),
						Option("sms", Text("prefs->notifications->smsFlag")),
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
