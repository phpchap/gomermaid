# GoMermaid

GoMermaid is a self-contained Golang port of SpaceXAI's terminal renderer for Mermaid diagrams. It transforms Mermaid source code (flowcharts, sequence diagrams, state diagrams, and class diagrams) into beautifully laid out Unicode box-drawing art that can be rendered directly to the terminal!

It implements a native recursive descent parser, a Sugiyama layered graph drawing layout algorithm with orthogonal edge routing, and a canvas painter specifically designed for the monospace grid of a terminal.

### Subgraph / group rendering

Flowchart `subgraph` blocks are rendered as nested, titled frames so the terminal output mirrors the grouped structure shown in Markdown/GitHub renderers. Groups are laid out hierarchically (arbitrary nesting is supported): each subgraph is recursively laid out into its own sub-canvas and embedded as a frame, and edges are routed at the lowest common scope of their endpoints. `classDef`/`class` colours are preserved on the nodes inside groups.

> Tip: apply `class` styling to **real node IDs** (e.g. `class EB eventBus`), not subgraph IDs — a `class <subgraphId> ...` line creates a stray node, since class assignment auto-creates any unknown identifier.

## Requirements
* Go 1.21+

## Installation
Currently you can simply pull it into your Go modules if it is hosted or compile from source. It requires `github.com/mattn/go-runewidth` to correctly measure CJK and wide emoji characters in the terminal layout grid.

```bash
go mod tidy
```

## Running Examples

There are a few examples included to demonstrate the layout engine. You can run them via:

### 1. Flowchart Example
A standard Directed Acyclic Graph rendering.
```bash
go run examples/flowchart.go
```

### 2. Sequence Diagram Example
Renders actor lifelines, messages, dividers, and note alignments.
```bash
go run examples/sequence.go
```

### 3. State Diagram Example
Demonstrates start/end nodes and cyclic graphs.
```bash
go run examples/state.go
```

### 4. Class Diagram Example
Demonstrates fallback simple node-relationship mapping.
```bash
go run examples/class.go
```

### 5. Colored Output Example
Demonstrates inline `classDef` styling mapped to 24-bit ANSI colors.
```bash
go run examples/colored.go
```

### 6. Markdown Renderer Example (GitHub Strategy)
Demonstrates reading a Markdown file containing standard ` ```mermaid ` blocks and rendering them to the terminal.
```bash
go run examples/markdown.go examples/test.md
```

## Using in your own project

```go
package main

import (
	"fmt"
	"github.com/phpchap/gomermaid"
)

func main() {
	src := `graph LR
	A[Go Parser] --> B[Sugiyama Layout]
	B --> C(Terminal String Canvas)`

	// Default unstyled (plain text ascii)
	styles := &mermaid.MermaidStyles{}
	
	// Max width pointer (nil = infinite)
	art, err := mermaid.Render(src, styles, nil)
	if err != nil {
		panic(err)
	}

	for _, line := range art.PlainLines {
		fmt.Println(line)
	}
}
```