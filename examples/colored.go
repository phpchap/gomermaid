package main

import (
	"fmt"
	"github.com/phpchap/gomermaid"
)

func main() {
	src := `graph LR
classDef example1 color:#ff0000
classDef example2 color:#00ff00
classDef example3 color:#0000ff
test1:::example1 --> test2
test2:::example2 --> test3:::example3`

	fmt.Println("=== Colored Diagram ===")
	styles := &mermaid.MermaidStyles{
		Border: "\033[90m",
		NodeText: "\033[97m",
		Edge: "\033[90m",
	}
	art, _ := mermaid.Render(src, styles, nil)
	for _, line := range art.StyledLines {
		fmt.Println(line)
	}
}
