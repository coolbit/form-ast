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
  // Define your form AST
  form := ast.Form{
    "username": "alice",
    "mode":     "advanced",
    // ... other form data ...
  }

  root := ast.Group("Form A",
    ast.Step("dashboard",
      ast.Text("username"),
      ast.Radio("mode",
        ast.Option("basic", ast.Text("modeSimpleFlag")),
        ast.Option("advanced", ast.Text("modeAdvFlag")),
      ),
      // ... more nodes ...
    ),
  )

  formAst, err := ast.NewAST(root)
  if err != nil {
    panic(err)
  }

  fmt.Println("All fields:", formAst.AllFields())
  fmt.Println("Selected fields:", formAst.Selected(form))

  // Print only selected branches
  formAst.Print(os.Stdout, form)
  // Print entire tree
  formAst.Print(os.Stdout, nil)
}
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
