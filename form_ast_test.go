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
