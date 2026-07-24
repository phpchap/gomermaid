package mermaid

import (
	"testing"
)

func TestRender_Flowchart(t *testing.T) {
	diagrams := []string{
		`graph TD
		A[Hard edge] -->|Link text| B(Round edge)
		B --> C{Decision}
		C -->|One| D[Result one]
		C -->|Two| E[Result two]
		`,
		`graph LR
		A[Node] --- B[Node]
		A -.- C
		A === D
		A ==> E
		A -.-> F
		`,
		`graph TD
		subgraph One
		a1-->a2
		end
		subgraph Two
		b1-->b2
		end
		subgraph Three
		c1-->c2
		end
		c1-->a2
		`,
		`graph TD
		A((Circle))
		B>Asymmetric]
		C{Rhombus}
		D{{Hexagon}}
		E[/Parallelogram/]
		F[\Parallelogram alt\]
		G[/Trapezoid\]
		H[\Trapezoid alt/]
		I(((Double circle)))
		`,
		`graph LR
		classDef example1 color:#ff0000 stroke:#00ff00 fill:#0000ff
		test1:::example1 --> test2
		class test2 example1
		`,
		`graph BT
		A --> B
		`,
		`graph RL
		A --> B
		`,
		`graph TD
		A["A &amp; B &lt; C &#x2665;"] --> B
		`,
		`graph TD
		subgraph A[My Label]
		  b
		end
		`,
	}

	styles := &MermaidStyles{}
	for i, src := range diagrams {
		art, err := Render(src, styles, nil)
		if err != nil {
			t.Errorf("Failed to render flowchart %d: %v", i, err)
		}
		if art == nil {
			t.Errorf("Rendered flowchart %d is nil", i)
		}
	}
}

func TestRender_Sequence(t *testing.T) {
	diagrams := []string{
		`sequenceDiagram
		participant Alice
		participant Bob
		Alice->>Bob: Hello Bob, how are you?
		Bob-->>Alice: I am good thanks!
		`,
		`sequenceDiagram
		actor A as Alice
		actor B as Bob
		A->B: Normal line
		A-->B: Dotted line
		A->>B: Arrow line
		A-->>B: Dotted arrow
		A-xB: Cross
		A--xB: Dotted cross
		A-)B: Open arrow
		A--)B: Dotted open arrow
		`,
		`sequenceDiagram
		Alice->>Bob: Hello
		activate Bob
		Bob-->>Alice: Hi
		deactivate Bob
		Alice->>Bob: + Hello again
		Bob-->>Alice: - Hi again
		`,
		`sequenceDiagram
		Note right of Alice: Alice thinks
		Note left of Bob: Bob thinks
		Note over Alice,Bob: Both think
		`,
		`sequenceDiagram
		loop Every minute
			Alice->>Bob: Ping
		end
		alt is sick
			Bob-->>Alice: NACK
		else is healthy
			Bob-->>Alice: ACK
		end
		opt Extra
			Alice->>Bob: Extra
		end
		`,
		`sequenceDiagram
		autonumber
		Alice->>Bob: Hello
		Bob-->>Alice: Hi
		`,
	}

	styles := &MermaidStyles{}
	for i, src := range diagrams {
		art, err := Render(src, styles, nil)
		if err != nil {
			t.Errorf("Failed to render sequence %d: %v", i, err)
		}
		if art == nil {
			t.Errorf("Rendered sequence %d is nil", i)
		}
	}
}

func TestRender_State(t *testing.T) {
	diagrams := []string{
		`stateDiagram-v2
		[*] --> First
		First --> Second
		First --> Third
		Second --> [*]
		`,
		`stateDiagram
		state "This is a state" as s1
		s1 --> s2
		s2 --> s1
		`,
		`stateDiagram-v2
		direction LR
		A --> B
		`,
		`stateDiagram-v2
		direction RL
		A --> B
		`,
		`stateDiagram-v2
		direction BT
		A --> B
		`,
		`stateDiagram-v2
		state if_state <<choice>>
		[*] --> IsPositive
		IsPositive --> if_state
		if_state --> False: if n < 0
		if_state --> True : if n >= 0
		`,
		`stateDiagram-v2
		[*] --> Active
		state Active {
			[*] --> Num1
			Num1 --> Num2
			--
			[*] --> Letter1
			Letter1 --> Letter2
		}
		`,
		`stateDiagram
		First --> Second
		Note right of First: This is a note
		Note left of Second: Another note
		`,
	}

	styles := &MermaidStyles{}
	for i, src := range diagrams {
		art, err := Render(src, styles, nil)
		if err != nil {
			t.Errorf("Failed to render state %d: %v", i, err)
		}
		if art == nil {
			t.Errorf("Rendered state %d is nil", i)
		}
	}
}

func TestRender_Class(t *testing.T) {
	diagrams := []string{
		`classDiagram
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
		}
		`,
		`classDiagram
		Class01 <|-- Class02
		Class03 *-- Class04
		Class05 o-- Class06
		Class07 .. Class08
		Class09 -- Class10
		Class11 <|.. Class12
		Class13 --> Class14
		Class15 ..> Class16
		Class17 ..|> Class18
		`,
		`classDiagram
		Customer "1" --> "*" Ticket
		Student "1" --> "1..*" Course
		Galaxy --> "many" Star : Contains
		`,
		`classDiagram
		direction RL
		class BankAccount
		BankAccount : +String owner
		BankAccount : +Bigdecimal balance
		BankAccount : +deposit(amount)
		BankAccount : +withdrawal(amount)
		`,
		`classDiagram
		direction BT
		class Shape
		<<interface>> Shape
		Shape : noOfVertices
		Shape : draw()
		`,
	}

	styles := &MermaidStyles{}
	for i, src := range diagrams {
		art, err := Render(src, styles, nil)
		if err != nil {
			t.Errorf("Failed to render class diagram %d: %v", i, err)
		}
		if art == nil {
			t.Errorf("Rendered class diagram %d is nil", i)
		}
	}
}

func TestRender_ErrorsAndEdgeCases(t *testing.T) {
	diagrams := []string{
		``, // Empty
		`invalid diagram type`,
		`graph LR
		A --> `, // Incomplete
		`sequenceDiagram
		participant A`, // Missing messages
		`graph TD
		subgraph A
		end`,
	}

	styles := &MermaidStyles{}
	for _, src := range diagrams {
		Render(src, styles, nil)
	}
}