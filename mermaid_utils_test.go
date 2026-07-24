package mermaid

import (
	"strings"
	"testing"
)

func TestCharWidth(t *testing.T) {
	tests := []struct {
		char rune
		want int
	}{
		{'A', 1},
		{'中', 2},
		{'a', 1},
		{'🚀', 2},
	}
	for _, tc := range tests {
		if got := charWidth(tc.char); got != tc.want {
			t.Errorf("charWidth(%q) = %d, want %d", tc.char, got, tc.want)
		}
	}
}

func TestExtractStyleDef(t *testing.T) {
	styles := "color:#ff0000 stroke:#00ff00 fill:#0000ff"
	def := extractStyleDef(styles)
	if def.Color != "\033[38;2;255;0;0m" {
		t.Errorf("expected color 38;2;255;0;0, got %q", def.Color)
	}
	if def.Stroke != "\033[38;2;0;255;0m" {
		t.Errorf("expected stroke 38;2;0;255;0, got %q", def.Stroke)
	}
	if def.Fill != "\033[48;2;0;0;255m" {
		t.Errorf("expected fill 48;2;0;0;255, got %q", def.Fill)
	}

	invalid := "color:red stroke:#12"
	def2 := extractStyleDef(invalid)
	if def2.Color != "" {
		t.Errorf("expected empty color, got %q", def2.Color)
	}
	if def2.Stroke != "" {
		t.Errorf("expected empty stroke, got %q", def2.Stroke)
	}
}

func TestStyleFor(t *testing.T) {
	styles := &MermaidStyles{
		Border:    "border_style",
		NodeText:  "text_style",
		Edge:      "edge_style",
		EdgeLabel: "label_style",
	}

	custom := NodeStyle{
		Color:  "custom_color",
		Stroke: "custom_stroke",
		Fill:   "custom_fill",
	}
	emptyCustom := NodeStyle{}

	if got := styleFor(ClsBorder, styles, emptyCustom); got != "border_style" {
		t.Errorf("expected border_style, got %q", got)
	}
	if got := styleFor(ClsBorder, styles, custom); got != "custom_stroke" {
		t.Errorf("expected custom_stroke, got %q", got)
	}
	if got := styleFor(ClsText, styles, emptyCustom); got != "text_style" {
		t.Errorf("expected text_style, got %q", got)
	}
	if got := styleFor(ClsText, styles, custom); got != "custom_color" {
		t.Errorf("expected custom_color, got %q", got)
	}
	if got := styleFor(ClsEdge, styles, emptyCustom); got != "edge_style" {
		t.Errorf("expected edge_style, got %q", got)
	}
	if got := styleFor(ClsEdge, styles, custom); got != "custom_stroke" {
		t.Errorf("expected custom_stroke, got %q", got)
	}
	if got := styleFor(ClsEdgeLabel, styles, emptyCustom); got != "label_style" {
		t.Errorf("expected label_style, got %q", got)
	}
	if got := styleFor(ClsEmpty, styles, emptyCustom); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestWrapLabel(t *testing.T) {
	text := "This is a long text that needs wrapping"
	wrapped := wrapLabel(text, 10, 3)
	if len(wrapped) > 3 {
		t.Errorf("wrapLabel exceeded max lines: %d", len(wrapped))
	}
	if !strings.Contains(wrapped[0], "This is") {
		t.Errorf("unexpected wrap output: %v", wrapped)
	}
	
	textNoSpace := "123456789012345"
	wrappedNoSpace := wrapLabel(textNoSpace, 10, 2)
	if len(wrappedNoSpace) > 2 {
		t.Errorf("wrapLabel exceeded max lines for no space: %v", wrappedNoSpace)
	}
}

func TestFitLabel(t *testing.T) {
	text := "Very long label that shouldn't fit"
	fitted := fitLabel(text, 10)
	if len(fitted) > 10 { // length in chars may vary, but let's just check it trimmed
		if !strings.Contains(fitted, "…") {
			t.Errorf("expected truncation with …, got %q", fitted)
		}
	}
}

func TestCanvasOps(t *testing.T) {
	c := newCanvas(10, 10)
	if c.w != 10 || c.h != 10 {
		t.Errorf("canvas dimensions wrong")
	}

	// Test boundary checks
	c.setColor(15, 15, 'x', ClsText, NodeStyle{})
	c.addBitsColor(15, 15, L, NodeStyle{})

	c.setColor(5, 5, 'A', ClsText, NodeStyle{Color: "red"})
	if c.ch[c.idx(5, 5)] != 'A' {
		t.Errorf("setColor failed")
	}
	if c.customStyle[c.idx(5, 5)].Color != "red" {
		t.Errorf("setColor style failed")
	}

	c.addBitsColor(6, 6, L|R, NodeStyle{Stroke: "blue"})
	if c.mask[c.idx(6, 6)] != L|R {
		t.Errorf("addBits failed")
	}
	if c.customStyle[c.idx(6, 6)].Stroke != "blue" {
		t.Errorf("addBitsColor style failed")
	}
	
	// Occupy test
	c.occupied[c.idx(6, 6)] = true
	c.addBitsColor(6, 6, U|D, NodeStyle{})
	if c.mask[c.idx(6, 6)] != L|R {
		t.Errorf("addBits modified occupied cell")
	}

	sub := newCanvas(2, 2)
	sub.setColor(0, 0, 'B', ClsText, NodeStyle{})
	c.blit(sub, 1, 1)
	if c.ch[c.idx(1, 1)] != 'B' {
		t.Errorf("blit failed")
	}
}

func TestParseGraph_EdgeCases(t *testing.T) {
	// Let's test a deeply nested subgraph
	src := `graph TD
	subgraph A
	  subgraph B
	    c
	  end
	end
	`
	Render(src, &MermaidStyles{}, nil)

	// Subgraph with string label
	src2 := `graph TD
	subgraph "Label with space"
	  d
	end
	`
	Render(src2, &MermaidStyles{}, nil)
}

func TestParseSequence_EdgeCases(t *testing.T) {
	src := `sequenceDiagram
	Note over A, B, C: Invalid multiple note anchors
	loop
		A->B: msg
	end
	par
		A->B: msg
	and
		B->C: msg
	end
	rect rgb(200, 200, 200)
		A->B: msg
	end
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestParseClass_EdgeCases(t *testing.T) {
	src := `classDiagram
	class EmptyClass
	class ~Generic~Type~ {
		+method(param1, param2)
	}
	A --|> B : inheritance
	C *-- D : composition
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestParseState_EdgeCases(t *testing.T) {
	src := `stateDiagram
	state "Complex Name" as C
	C --> [*]
	[*] --> C
	state A {
		--
		B --> C
	}
	note right of C
	multiline
	note
	end note
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestRouteBack_Self(t *testing.T) {
	src := `graph TD
	A --> A
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestRouteBack_Reverse(t *testing.T) {
	src := `graph TD
	A --> B
	B --> A
	A --> C
	C --> B
	C -.-> A
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestParseLink_EdgeCases(t *testing.T) {
	src := `graph TD
	A x--x B
	A o--o B
	A <--> B
	A <==> B
	A <-.-> B
	A == text ==> B
	`
	Render(src, &MermaidStyles{}, nil)
}

func TestLayoutSequence_Errors(t *testing.T) {
	// Empty sequence diagram
	Render(`sequenceDiagram`, &MermaidStyles{}, nil)
}

func TestLayoutGraph_Errors(t *testing.T) {
	// Invalid link styles
	Render(`graph TD
	linkStyle 0 stroke:#ff3,stroke-width:4px
	style A fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5
	A --> B
	`, &MermaidStyles{}, nil)
}
