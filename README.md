# form-ast

A flexible, tree-based form AST library in Go, for defining form structures, traversing fields, and printing trees based on user input.

The AST supports text inputs, radio buttons, checkboxes, selects, grouping steps, and custom option nodes, with built-in cycle detection and field-selection logic.

## Features

* **Node Types**: `TextNode`, `RadioNode`, `CheckboxNode`, `SelectNode`, `StepNode`, `GroupNode`, `OptionNode`.
* **Form Traversal**: Extract selected fields via `Fields(form Form)` and all possible fields via `AllFields()`.
* **Cycle Detection**: Prevents infinite recursion using `ValidateNoCycles`.
* **Tree Printing**: Renders the form AST as a tree, highlighting selected nodes and values.
* **Unique Field Collection**: Ensures deduplicated lists of all and selected fields.

## Installation

```bash
go get github.com/coolbit/form-ast
```

## Usage

```go
package main

import (
	"fmt"
	"os"

	"github.com/coolbit/form-ast"
)

func main() {

	form := ast.Form{
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
		// ... other form data ...
	}

	// Define your form AST
	root := ast.Group("Form A",
		ast.Step("dashboard",
			ast.Text("username"),
			ast.Radio("mode",
				ast.Option("basic",
					ast.Text("modeSimpleFlag"),
				),
				ast.Option("advanced",
					ast.Text("modeAdvFlag"),
					ast.Checkbox("features",
						ast.Option("f1", ast.Text("feat1")),
						ast.Option("f2", ast.Text("feat2")),
						ast.Option("f3", ast.Text("feat3")),
					),
				),
			),
			ast.Checkbox("options",
				ast.Option("o1",
					ast.Select("singleSelect",
						ast.Option("alpha", ast.Text("selAlpha")),
						ast.Option("beta", ast.Text("selBeta")),
					),
				),
				ast.Option("o2", ast.Text("opt2Only")),
				ast.Option("o3", ast.Text("opt3Only")),
			),
		),
		ast.Step("profile",
			ast.Text("singleSelect"),
		),
		// ... more nodes ...
	)

	formAst, err := ast.NewAST(root)
	if err != nil {
		panic(err)
	}

	fmt.Println("All fields:", formAst.AllFields())
	fmt.Println("selected fields:", formAst.Selected(form))

	// Print entire tree
	formAst.Print(os.Stdout, nil)
	println()

	// Print only selected branches
	formAst.Print(os.Stdout, form)
}
```
### OUTPUT
```text
All fields: [username mode modeSimpleFlag modeAdvFlag features feat1 feat2 feat3 options singleSelect selAlpha selBeta opt2Only opt3Only]
selected fields: [username mode modeAdvFlag features feat1 feat2 options singleSelect selBeta opt3Only]
(Group) label="Form A"
├── (Step) label="dashboard"
│   ├── (Text) name="username"
│   ├── (Radio) name="mode"
│   │   ├── (Option) option="basic"
│   │   │   └── (Text) name="modeSimpleFlag"
│   │   └── (Option) option="advanced"
│   │       ├── (Text) name="modeAdvFlag"
│   │       └── (Checkbox) name="features"
│   │           ├── (Option) option="f1"
│   │           │   └── (Text) name="feat1"
│   │           ├── (Option) option="f2"
│   │           │   └── (Text) name="feat2"
│   │           └── (Option) option="f3"
│   │               └── (Text) name="feat3"
│   └── (Checkbox) name="options"
│       ├── (Option) option="o1"
│       │   └── (Select) name="singleSelect"
│       │       ├── (Option) option="alpha"
│       │       │   └── (Text) name="selAlpha"
│       │       └── (Option) option="beta"
│       │           └── (Text) name="selBeta"
│       ├── (Option) option="o2"
│       │   └── (Text) name="opt2Only"
│       └── (Option) option="o3"
│           └── (Text) name="opt3Only"
└── (Step) label="profile"
    └── (Text) name="singleSelect"

(Group) label="Form A"
├── (Step) label="dashboard"
│   ├── (Text) name="username" value="alice"
│   ├── (Radio) name="mode"
│   │   └── (Option) option="advanced"
│   │       ├── (Text) name="modeAdvFlag" value="f"
│   │       └── (Checkbox) name="features"
│   │           ├── (Option) option="f1"
│   │           │   └── (Text) name="feat1" value="1"
│   │           ├── (Option) option="f2"
│   │           │   └── (Text) name="feat2" value="2"
│   │           └── (Option) option="f3"
│   │               └── (Text) name="feat3" value="3"
│   └── (Checkbox) name="options"
│       ├── (Option) option="o1"
│       │   └── (Select) name="singleSelect"
│       │       ├── (Option) option="alpha"
│       │       │   └── (Text) name="selAlpha" value="alpha"
│       │       └── (Option) option="beta"
│       │           └── (Text) name="selBeta" value="beta"
│       └── (Option) option="o3"
│           └── (Text) name="opt3Only" value="opt3Only"
└── (Step) label="profile"
    └── (Text) name="singleSelect" value="beta"
```

## API

### `Node` Interface

```go
type Node interface {
  String() string
  Children() []Node
  Fields(form Form) []string
  AllFields() []string
}
```

### Common Functions

* `func NewAST(root Node) (*AST, error)`: Creates an AST, validating no cycles.
* `func (a *AST) AllFields() []string`: Returns all unique field names in the tree.
* `func (a *AST) Selected(form Form) []string`: Returns the unique fields selected by the given form.
* `func (a *AST) Print(w io.Writer, form Form) error`: Prints the tree; when `form` is non-nil, only selected nodes are printed.


## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
