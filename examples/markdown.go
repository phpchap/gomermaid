package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/phpchap/gomermaid"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run markdown.go <file-or-directory>")
		os.Exit(1)
	}

	target := os.Args[1]

	stat, err := os.Stat(target)
	if err != nil {
		fmt.Printf("Error accessing path: %v\n", err)
		os.Exit(1)
	}

	styles := &mermaid.MermaidStyles{
		Border:   "\033[90m",
		NodeText: "\033[97m",
		Edge:     "\033[90m",
	}

	if stat.IsDir() {
		err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && filepath.Ext(path) == ".md" {
				renderFile(path, styles)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking directory: %v\n", err)
		}
	} else {
		renderFile(target, styles)
	}
}

func renderFile(filename string, styles *mermaid.MermaidStyles) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	re := regexp.MustCompile("(?s)```mermaid\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(string(content), -1)

	if len(matches) == 0 {
		return
	}

	for i, match := range matches {
		src := match[1]
		fmt.Printf("\n\033[1;36m=== Rendering Diagram %d from %s ===\033[0m\n\n", i+1, filename)
		
		art, err := mermaid.Render(src, styles, nil)
		if err != nil {
			fmt.Printf("Error rendering diagram %d: %v\n", i+1, err)
			continue
		}
		if art == nil {
			fmt.Printf("\033[33mDiagram renderer returned nil. This may be an unsupported diagram type (e.g. pie, gantt, er).\033[0m\n")
			continue
		}

		for _, line := range art.StyledLines {
			fmt.Println(line)
		}
		fmt.Println()
	}
}
