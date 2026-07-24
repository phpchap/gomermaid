package main

import (
	"fmt"
	"gomermaid"
)

func main() {
	src := `classDiagram
    Animal <|-- Duck
    Animal <|-- Fish
    Animal <|-- Zebra
    Animal : +int age
    Animal : +String gender
    Animal: +isMammal()
    Animal: +mate()
    class Duck{
      +String beakColor
      +swim()
      +quack()
    }
    class Fish{
      -int sizeInFeet
      -canEat()
    }
    class Zebra{
      +bool is_wild
      +run()
    }`

	fmt.Println("=== Class Diagram ===")
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