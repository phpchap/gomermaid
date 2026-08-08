package main

import (
	"fmt"
	"github.com/phpchap/gomermaid"
)

func main() {
	src := `stateDiagram-v2
    [*] --> Still
    Still --> [*]

    Still --> Moving
    Moving --> Still
    Moving --> Crash
    Crash --> [*]`

	fmt.Println("=== State Diagram ===")
	fmt.Println("Source:")
	fmt.Println(src)
	fmt.Println("\nRendered:")

	styles := &mermaid.MermaidStyles{}
	art, err := mermaid.Render(src, styles, nil)
	if err != nil {
		panic(err)
	}
	if art == nil {
		fmt.Println("No diagram rendered")
		return
	}

	for _, line := range art.PlainLines {
		fmt.Println(line)
	}
}