# form-ast
A Go library for representing form structures as an Abstract Syntax Tree (AST), with support for field extraction, path parsing, and key–value mapping.

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
	root := ast.Container("Form A",
		ast.Container("dashboard",
			ast.Field("username"),
			ast.Choice("mode", false,
				ast.Option("basic",
					ast.Field("modeSimpleFlag"),
				),
				ast.Option("advanced",
					ast.Field("modeAdvFlag"),
					ast.Choice("features", true,
						ast.Option("f1", ast.Field("feat1")),
						ast.Option("f2", ast.Field("feat2")),
						ast.Option("f3", ast.Field("feat3")),
					),
				),
			),
			ast.Container("options",
				ast.Option("o1",
					ast.Choice("singleSelect", false,
						ast.Option("alpha", ast.Field("selAlpha")),
						ast.Option("beta", ast.Field("selBeta")),
					),
				),
				ast.Option("o2", ast.Field("opt2Only")),
				ast.Option("o3", ast.Field("opt3Only")),
			),
		),
		ast.Container("profile",
			ast.Field("singleSelect"),
		),
		// ... more nodes ...
	)

	formAst, err := ast.NewAST(root)
	if err != nil {
		panic(err)
	}

	fmt.Println("All fields:", formAst.AllFields())
	fmt.Println("selected fields:", formAst.Selected(form))
	fmt.Println("key value:", formAst.KeyValue(form))

	// Print entire tree
	formAst.Print(os.Stdout)
}
```
### Output
```text
All fields: [username mode modeSimpleFlag modeAdvFlag features feat1 feat2 feat3 singleSelect selAlpha selBeta opt2Only opt3Only]
selected fields: [username mode modeAdvFlag features feat1 feat2 singleSelect selBeta opt3Only]
key value: map[feat1:1 feat2:2 features:[f1 f2 f4] mode:advanced modeAdvFlag:f opt3Only:opt3Only selBeta:beta singleSelect:beta username:alice]
(Container) name="Form A"
├── (Container) name="dashboard"
│   ├── (Field) field="username"
│   ├── (Choice) field="mode" multiple=false
│   │   ├── (Option) option="basic"
│   │   │   └── (Field) field="modeSimpleFlag"
│   │   └── (Option) option="advanced"
│   │       ├── (Field) field="modeAdvFlag"
│   │       └── (Choice) field="features" multiple=true
│   │           ├── (Option) option="f1"
│   │           │   └── (Field) field="feat1"
│   │           ├── (Option) option="f2"
│   │           │   └── (Field) field="feat2"
│   │           └── (Option) option="f3"
│   │               └── (Field) field="feat3"
│   └── (Container) name="options"
│       ├── (Option) option="o1"
│       │   └── (Choice) field="singleSelect" multiple=false
│       │       ├── (Option) option="alpha"
│       │       │   └── (Field) field="selAlpha"
│       │       └── (Option) option="beta"
│       │           └── (Field) field="selBeta"
│       ├── (Option) option="o2"
│       │   └── (Field) field="opt2Only"
│       └── (Option) option="o3"
│           └── (Field) field="opt3Only"
└── (Container) name="profile"
    └── (Field) field="singleSelect"
```

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
