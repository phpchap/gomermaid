package main

import (
	"fmt"
	"gomermaid"
)

func main() {
	src := `sequenceDiagram
    autonumber
    Alice->>John: Hello John, how are you?
    loop Healthcheck
        John->>John: Fight against hypochondria
    end
    Note right of John: Rational thoughts!
    John-->>Alice: Great!
    John->>Bob: How about you?
    Bob-->>John: Jolly good!`

	fmt.Println("=== Sequence Diagram ===")
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