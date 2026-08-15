package mermaid

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

// MermaidStyles holds simple styling info (just string placeholders for now).
type MermaidStyles struct {
	Border    string
	NodeText  string
	Edge      string
	EdgeLabel string
	Title     string
}

// MermaidArt represents the final output of the diagram.
type MermaidArt struct {
	StyledLines []string // In Go, you can just use strings with ANSI codes, or something similar
	PlainLines  []string
}

const (
	MAX_LABEL        = 28
	PAD              = 1
	GAP_X            = 3
	GAP_Y            = 2
	WRAP_WIDTH       = 24
	MAX_LINES        = 4
	CONT             = '\u0000'
	MAX_NODES        = 128
	MAX_EDGES        = 512
	MAX_GROUPS       = 24
	MAX_GROUP_DEPTH  = 6
	MAX_CANVAS_CELLS = 1 << 21
)

var LABEL_BREAK_CHARS = []rune{'_', '-', '.', '/'}

func charWidth(c rune) int {
	return runewidth.RuneWidth(c)
}

// Oversize error type
type Oversize int

const (
	OversizeWidth Oversize = iota
	OversizeCells
)

// Enums
type Shape int

const (
	ShapeRect Shape = iota
	ShapeRound
	ShapeDiamond
)

type Head int

const (
	HeadNone Head = iota
	HeadArrow
	HeadCircle
	HeadCross
	HeadTriangle
	HeadDiamondFill
	HeadDiamondOpen
)

type LineKind int

const (
	LineKindSolid LineKind = iota
	LineKindDotted
	LineKindThick
)

type Dir int

const (
	DirDown Dir = iota
	DirUp
	DirRight
	DirLeft
)

const (
	U uint8 = 1
	D uint8 = 2
	L uint8 = 4
	R uint8 = 8
)

type Cls int

const (
	ClsEmpty Cls = iota
	ClsBorder
	ClsText
	ClsEdge
	ClsEdgeLabel
)

const (
	STY_DOT   uint8 = 1
	STY_THICK uint8 = 2
	STY_SOLID uint8 = 4
)

// Canvas represents the 2D grid for drawing the diagram.
type Canvas struct {
	w           int
	h           int
	ch          []rune
	cls         []Cls
	mask        []uint8
	style       []uint8
	occupied    []bool
	cur_style   uint8
	customStyle []NodeStyle
}

func newCanvas(w, h int) *Canvas {
	n := w * h
	ch := make([]rune, n)
	for i := range ch {
		ch[i] = ' '
	}
	return &Canvas{
		w:           w,
		h:           h,
		ch:          ch,
		cls:         make([]Cls, n),
		mask:        make([]uint8, n),
		style:       make([]uint8, n),
		occupied:    make([]bool, n),
		cur_style:   STY_SOLID,
		customStyle: make([]NodeStyle, n),
	}
}

func (c *Canvas) idx(x, y int) int {
	return y*c.w + x
}

func (c *Canvas) set(x, y int, ch rune, cls Cls) {
	c.setColor(x, y, ch, cls, NodeStyle{})
}

func (c *Canvas) setColor(x, y int, ch rune, cls Cls, style NodeStyle) {
	if x >= c.w || y >= c.h {
		return
	}
	i := c.idx(x, y)
	c.ch[i] = ch
	c.cls[i] = cls
	if style.Color != "" || style.Stroke != "" || style.Fill != "" {
		c.customStyle[i] = style
	}
}

func (c *Canvas) addBits(x, y int, bits uint8) {
	c.addBitsColor(x, y, bits, NodeStyle{})
}

func (c *Canvas) addBitsColor(x, y int, bits uint8, style NodeStyle) {
	if x >= c.w || y >= c.h {
		return
	}
	i := c.idx(x, y)
	if c.occupied[i] {
		return
	}
	c.mask[i] |= bits
	c.style[i] |= c.cur_style
	if c.cls[i] != ClsBorder {
		c.cls[i] = ClsEdge
	}
	if style.Color != "" || style.Stroke != "" || style.Fill != "" {
		c.customStyle[i] = style
	}
}

func (c *Canvas) blit(sub *Canvas, ox, oy int) {
	for sy := 0; sy < sub.h; sy++ {
		for sx := 0; sx < sub.w; sx++ {
			x, y := ox+sx, oy+sy
			if x >= c.w || y >= c.h {
				continue
			}
			si := sub.idx(sx, sy)
			di := c.idx(x, y)
			c.ch[di] = sub.ch[si]
			c.cls[di] = sub.cls[si]
			c.style[di] = sub.style[si]
			c.occupied[di] = sub.occupied[si]
			c.customStyle[di] = sub.customStyle[si]
		}
	}
}

func (c *Canvas) junction(x, y int, bits uint8) {
	if x >= c.w || y >= c.h {
		return
	}
	i := c.idx(x, y)
	c.mask[i] |= bits
	if c.cls[i] != ClsBorder {
		c.cls[i] = ClsEdge
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *Canvas) segV(x, y0, y1 int) {
	a, b := min(y0, y1), max(y0, y1)
	for y := a; y <= b; y++ {
		var bits uint8 = 0
		if y > a {
			bits |= U
		}
		if y < b {
			bits |= D
		}
		c.addBits(x, y, bits)
	}
}

func (c *Canvas) segH(y, x0, x1 int) {
	a, b := min(x0, x1), max(x0, x1)
	for x := a; x <= b; x++ {
		var bits uint8 = 0
		if x > a {
			bits |= L
		}
		if x < b {
			bits |= R
		}
		c.addBits(x, y, bits)
	}
}

func (c *Canvas) finalizeMask() {
	for i := 0; i < len(c.ch); i++ {
		if c.mask[i] != 0 && c.ch[i] == ' ' {
			ch := maskChar(c.mask[i])
			switch c.style[i] {
			case STY_DOT:
				c.ch[i] = dottedChar(ch)
			case STY_THICK:
				c.ch[i] = thickChar(ch)
			default:
				c.ch[i] = ch
			}
		}
	}
}

func (c *Canvas) flipVertical() {
	for y := 0; y < c.h/2; y++ {
		y2 := c.h - 1 - y
		for x := 0; x < c.w; x++ {
			i, j := c.idx(x, y), c.idx(x, y2)
			c.ch[i], c.ch[j] = c.ch[j], c.ch[i]
			c.cls[i], c.cls[j] = c.cls[j], c.cls[i]
		}
	}
	for i := range c.ch {
		c.ch[i] = flipGlyphV(c.ch[i])
	}
}

func (c *Canvas) flipHorizontal() {
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w/2; x++ {
			x2 := c.w - 1 - x
			i, j := c.idx(x, y), c.idx(x2, y)
			c.ch[i], c.ch[j] = c.ch[j], c.ch[i]
			c.cls[i], c.cls[j] = c.cls[j], c.cls[i]
		}
	}
	for i := range c.ch {
		c.ch[i] = flipGlyphH(c.ch[i])
	}
	for y := 0; y < c.h; y++ {
		x := 0
		for x < c.w {
			cls := c.cls[c.idx(x, y)]
			if cls == ClsText || cls == ClsEdgeLabel {
				start := c.idx(x, y)
				for x < c.w && c.cls[c.idx(x, y)] == cls {
					x++
				}
				end := c.idx(x, y)
				// Reverse c.ch[start:end]
				slice := c.ch[start:end]
				for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
					slice[i], slice[j] = slice[j], slice[i]
				}
			} else {
				x++
			}
		}
	}
}

func styleFor(cls Cls, styles *MermaidStyles, customStyle NodeStyle) string {
	switch cls {
	case ClsBorder:
		if customStyle.Stroke != "" {
			return customStyle.Stroke
		}
		return styles.Border
	case ClsText:
		if customStyle.Color != "" {
			return customStyle.Color
		}
		return styles.NodeText
	case ClsEdge:
		if customStyle.Stroke != "" {
			return customStyle.Stroke
		}
		return styles.Edge
	case ClsEdgeLabel:
		return styles.EdgeLabel
	default:
		return ""
	}
}

func (c *Canvas) toLines(styles *MermaidStyles) ([]string, []string) {
	styled := make([]string, 0, c.h)
	plain := make([]string, 0, c.h)

	for y := 0; y < c.h; y++ {
		last := c.w
		for x := c.w - 1; x >= 0; x-- {
			ch := c.ch[c.idx(x, y)]
			if ch != ' ' && ch != CONT {
				last = x + 1
				break
			}
		}

		var plainRow strings.Builder
		var styledRow strings.Builder
		var run strings.Builder
		runCls := ClsEmpty
		runStyle := NodeStyle{}

		for x := 0; x < last; x++ {
			i := c.idx(x, y)
			ch := c.ch[i]
			if ch == CONT {
				continue
			}
			cls := c.cls[i]
			style := c.customStyle[i]
			plainRow.WriteRune(ch)

			if (cls != runCls || style != runStyle) && run.Len() > 0 {
				styledRow.WriteString(styleFor(runCls, styles, runStyle))
				styledRow.WriteString(run.String())
				styledRow.WriteString("\033[0m")
				run.Reset()
			}
			runCls = cls
			runStyle = style
			run.WriteRune(ch)
		}
		if run.Len() > 0 {
			styledRow.WriteString(styleFor(runCls, styles, runStyle))
			styledRow.WriteString(run.String())
			styledRow.WriteString("\033[0m")
		}

		styled = append(styled, styledRow.String())
		plainStr := strings.TrimRight(plainRow.String(), " ")
		plain = append(plain, plainStr)
	}

	return styled, plain
}

func maskChar(mask uint8) rune {
	switch mask {
	case 0:
		return ' '
	case U, D, U | D:
		return '│'
	case L, R, L | R:
		return '─'
	case D | R:
		return '┌'
	case D | L:
		return '┐'
	case U | R:
		return '└'
	case U | L:
		return '┘'
	case U | D | R:
		return '├'
	case U | D | L:
		return '┤'
	case D | L | R:
		return '┬'
	case U | L | R:
		return '┴'
	default:
		return '┼'
	}
}

func dottedChar(c rune) rune {
	switch c {
	case '─':
		return '╌'
	case '│':
		return '╎'
	default:
		return c
	}
}

func thickChar(c rune) rune {
	switch c {
	case '─':
		return '━'
	case '│':
		return '┃'
	case '┌':
		return '┏'
	case '┐':
		return '┓'
	case '└':
		return '┗'
	case '┘':
		return '┛'
	case '├':
		return '┣'
	case '┤':
		return '┫'
	case '┬':
		return '┳'
	case '┴':
		return '┻'
	case '┼':
		return '╋'
	default:
		return c
	}
}

func flipGlyphV(c rune) rune {
	switch c {
	case '┌':
		return '└'
	case '└':
		return '┌'
	case '┐':
		return '┘'
	case '┘':
		return '┐'
	case '┏':
		return '┗'
	case '┗':
		return '┏'
	case '┓':
		return '┛'
	case '┛':
		return '┓'
	case '╭':
		return '╰'
	case '╰':
		return '╭'
	case '╮':
		return '╯'
	case '╯':
		return '╮'
	case '┬':
		return '┴'
	case '┴':
		return '┬'
	case '┳':
		return '┻'
	case '┻':
		return '┳'
	case '▼':
		return '▲'
	case '▲':
		return '▼'
	case '▽':
		return '△'
	case '△':
		return '▽'
	default:
		return c
	}
}

func flipGlyphH(c rune) rune {
	switch c {
	case '┌':
		return '┐'
	case '┐':
		return '┌'
	case '└':
		return '┘'
	case '┘':
		return '└'
	case '┏':
		return '┓'
	case '┓':
		return '┏'
	case '┗':
		return '┛'
	case '┛':
		return '┗'
	case '╭':
		return '╮'
	case '╮':
		return '╭'
	case '╰':
		return '╯'
	case '╯':
		return '╰'
	case '├':
		return '┤'
	case '┤':
		return '├'
	case '┣':
		return '┫'
	case '┫':
		return '┣'
	case '▶':
		return '◄'
	case '◄':
		return '▶'
	case '▷':
		return '◁'
	case '◁':
		return '▷'
	default:
		return c
	}
}

type Group struct {
	ID     string
	Label  string
	Parent *int
}

type Node struct {
	Label string
	Shape Shape
	Class string
}

type Edge struct {
	From     int
	To       int
	Label    string
	HeadTo   Head
	HeadFrom Head
	Line     LineKind
}

type NodeStyle struct {
	Color  string
	Stroke string
	Fill   string
}

type Graph struct {
	Nodes     []Node
	Edges     []Edge
	Index     map[string]int
	Groups    []Group
	NodeGroup []*int
	CurGroup  *int
	OverCap   bool
	Dir       Dir
	ClassDefs map[string]NodeStyle
}

func (g *Graph) nodeIndex(id string, label *string, shape Shape) *int {
	if i, ok := g.Index[id]; ok {
		if label != nil {
			g.Nodes[i].Label = *label
			g.Nodes[i].Shape = shape
		}
		return &i
	}
	if len(g.Nodes) >= MAX_NODES {
		g.OverCap = true
		return nil
	}
	l := id
	if label != nil {
		l = *label
	}
	g.Index[id] = len(g.Nodes)
	g.Nodes = append(g.Nodes, Node{Label: l, Shape: shape})
	g.NodeGroup = append(g.NodeGroup, g.CurGroup)
	idx := len(g.Nodes) - 1
	return &idx
}

func (g *Graph) nodeLabel(id string, label string) *int {
	if i, ok := g.Index[id]; ok {
		g.Nodes[i].Label = label
		return &i
	}
	return g.nodeIndex(id, &label, ShapeRound)
}

func ParseGraph(src string) *Graph {
	var statements []string
	lines := strings.Split(src, "\n")
	for _, rawLine := range lines {
		splitStatements(rawLine, &statements)
	}

	if len(statements) == 0 {
		return nil
	}
	header := statements[0]
	headerTokens := strings.Fields(header)
	if len(headerTokens) == 0 {
		return nil
	}
	kind := strings.ToLower(headerTokens[0])
	if kind != "graph" && kind != "flowchart" {
		return nil
	}
	dirStr := "TB"
	if len(headerTokens) > 1 {
		dirStr = strings.ToUpper(headerTokens[1])
	}
	dir := DirDown
	switch dirStr {
	case "LR":
		dir = DirRight
	case "RL":
		dir = DirLeft
	case "BT":
		dir = DirUp
	}

	graph := &Graph{
		Nodes:     []Node{},
		Edges:     []Edge{},
		Index:     make(map[string]int),
		Groups:    []Group{},
		NodeGroup: []*int{},
		CurGroup:  nil,
		OverCap:   false,
		Dir:       dir,
		ClassDefs: make(map[string]NodeStyle),
	}

	stack := []int{}
	for _, st := range statements[1:] {
		fields := strings.Fields(st)
		firstWord := ""
		if len(fields) > 0 {
			firstWord = strings.ToLower(fields[0])
		}
		switch firstWord {
		case "subgraph":
			if len(graph.Groups) >= MAX_GROUPS || len(stack) >= MAX_GROUP_DEPTH {
				return nil
			}
			id, label := parseSubgraphDecl(strings.TrimSpace(st[len("subgraph"):]))
			var parent *int
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				parent = &p
			}
			graph.Groups = append(graph.Groups, Group{
				ID:     id,
				Label:  label,
				Parent: parent,
			})
			idx := len(graph.Groups) - 1
			stack = append(stack, idx)
			graph.CurGroup = &idx
			continue
		case "end":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				idx := stack[len(stack)-1]
				graph.CurGroup = &idx
			} else {
				graph.CurGroup = nil
			}
			continue
		case "classdef":
			rest := strings.TrimSpace(st[len("classdef"):])
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				className := parts[0]
				styles := strings.Join(parts[1:], "")
				styleDef := extractStyleDef(styles)
				graph.ClassDefs[className] = styleDef
			}
			continue
		case "class":
			// `class nodeId className`
			rest := strings.TrimSpace(st[len("class"):])
			parts := strings.Fields(rest)
			if len(parts) == 2 {
				nodeId := parts[0]
				className := parts[1]
				if idx := graph.nodeIndex(nodeId, nil, ShapeRound); idx != nil {
					graph.Nodes[*idx].Class = className
				}
			}
			continue
		case "style", "linkstyle", "click", "direction":
			continue
		}
		parseStatement(st, graph)
		if graph.OverCap {
			return nil
		}
	}

	if len(graph.Nodes) == 0 {
		return nil
	}
	return graph
}

func parseSubgraphDecl(rest string) (string, string) {
	if strings.HasPrefix(rest, "\"") {
		parts := strings.SplitN(rest[1:], "\"", 2)
		if len(parts) == 2 {
			label := parts[0]
			return label, decodeHTMLEntities(label)
		}
	}
	if idx := strings.IndexByte(rest, '['); idx != -1 {
		id := strings.TrimSpace(rest[:idx])
		label := strings.TrimSpace(strings.TrimSuffix(rest[idx+1:], "]"))
		label = cleanLabel(label)
		if id != "" && label != "" {
			return id, label
		}
	}
	return rest, rest
}

func splitStatements(line string, out *[]string) {
	var cur strings.Builder
	inQuotes := false
	chars := []rune(line)
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if inQuotes {
			if c == '"' {
				inQuotes = false
			}
			cur.WriteRune(c)
		} else {
			switch c {
			case '"':
				inQuotes = true
				cur.WriteRune(c)
			case '%':
				if i+1 < len(chars) && chars[i+1] == '%' {
					i = len(chars) // break equivalent
				} else {
					cur.WriteRune(c)
				}
			case ';':
				flushStatement(&cur, out)
			default:
				cur.WriteRune(c)
			}
		}
	}
	flushStatement(&cur, out)
}

func flushStatement(cur *strings.Builder, out *[]string) {
	trimmed := strings.TrimSpace(cur.String())
	if trimmed != "" {
		*out = append(*out, trimmed)
	}
	cur.Reset()
}

func extractStyleDef(styles string) NodeStyle {
	extract := func(key string, ansiPrefix string) string {
		idx := strings.Index(styles, key+":#")
		if idx != -1 {
			hexStr := styles[idx+len(key)+2:]
			if len(hexStr) >= 6 {
				r, e1 := strconv.ParseInt(hexStr[0:2], 16, 32)
				g, e2 := strconv.ParseInt(hexStr[2:4], 16, 32)
				b, e3 := strconv.ParseInt(hexStr[4:6], 16, 32)
				if e1 == nil && e2 == nil && e3 == nil {
					return fmt.Sprintf("\033[%s;%d;%d;%dm", ansiPrefix, r, g, b)
				}
			}
		}
		return ""
	}
	
	return NodeStyle{
		Color:  extract("color", "38;2"),
		Stroke: extract("stroke", "38;2"),
		Fill:   extract("fill", "48;2"),
	}
}

func parseStatement(st string, graph *Graph) {
	chars := []rune(st)
	i := 0
	prev, ni := parseNodeGroup(chars, i, graph)
	if prev == nil {
		return
	}
	i = ni

	for {
		i = skipSpaces(chars, i)
		if i >= len(chars) {
			break
		}
		left, right, line, label, ni := parseLink(chars, i)
		if ni == i {
			break
		}
		i = skipSpaces(chars, ni)
		next, ni := parseNodeGroup(chars, i, graph)
		if next == nil {
			break
		}
		i = ni

		for _, f := range prev {
			for _, t := range next {
				if len(graph.Edges) >= MAX_EDGES {
					graph.OverCap = true
					return
				}
				from, to := f, t
				headTo, headFrom := right, left
				if left == HeadArrow && right != HeadArrow {
					from, to = t, f
					headTo, headFrom = HeadArrow, right
				}
				lbl := ""
				if label != nil {
					lbl = *label
				}
				graph.Edges = append(graph.Edges, Edge{
					From:     from,
					To:       to,
					Label:    lbl,
					HeadTo:   headTo,
					HeadFrom: headFrom,
					Line:     line,
				})
			}
		}
		prev = next
	}
}

func parseNodeGroup(chars []rune, start int, graph *Graph) ([]int, int) {
	first, i := parseNode(chars, start, graph)
	if first == nil {
		return nil, start
	}
	group := []int{*first}
	for {
		j := skipSpaces(chars, i)
		if j >= len(chars) || chars[j] != '&' {
			break
		}
		next, k := parseNode(chars, j+1, graph)
		if next == nil {
			break
		}
		group = append(group, *next)
		i = k
	}
	return group, i
}

func skipSpaces(chars []rune, i int) int {
	for i < len(chars) && (chars[i] == ' ' || chars[i] == '\t') {
		i++
	}
	return i
}

func isIdChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func parseNode(chars []rune, start int, graph *Graph) (*int, int) {
	i := skipSpaces(chars, start)
	idStart := i
	for i < len(chars) && isIdChar(chars[i]) {
		i++
	}
	if i == idStart {
		return nil, start
	}
	id := string(chars[idStart:i])

	var shapePtr *Shape
	var label *string
	after := i

	if i < len(chars) {
		switch chars[i] {
		case '[':
			if i+1 < len(chars) && chars[i+1] == '[' {
				shapePtr, label, after = readShape(chars, i+2, "]]", ShapeRect)
			} else if i+1 < len(chars) && chars[i+1] == '(' {
				shapePtr, label, after = readShape(chars, i+2, ")]", ShapeRound)
			} else {
				shapePtr, label, after = readShape(chars, i+1, "]", ShapeRect)
			}
		case '(':
			if i+1 < len(chars) && chars[i+1] == '(' {
				shapePtr, label, after = readShape(chars, i+2, "))", ShapeRound)
			} else if i+1 < len(chars) && chars[i+1] == '[' {
				shapePtr, label, after = readShape(chars, i+2, "])", ShapeRound)
			} else {
				shapePtr, label, after = readShape(chars, i+1, ")", ShapeRound)
			}
		case '{':
			if i+1 < len(chars) && chars[i+1] == '{' {
				shapePtr, label, after = readShape(chars, i+2, "}}", ShapeDiamond)
			} else {
				shapePtr, label, after = readShape(chars, i+1, "}", ShapeDiamond)
			}
		case '>':
			shapePtr, label, after = readShape(chars, i+1, "]", ShapeRect)
		}
	}

	shape := ShapeRect
	if shapePtr != nil {
		shape = *shapePtr
	}
	idx := graph.nodeIndex(id, label, shape)
	if idx == nil {
		return nil, start
	}

	if after+2 < len(chars) && chars[after] == ':' && chars[after+1] == ':' && chars[after+2] == ':' {
		after += 3
		classStart := after
		for after < len(chars) && (isIdChar(chars[after]) || chars[after] == '-') {
			after++
		}
		if after > classStart {
			graph.Nodes[*idx].Class = string(chars[classStart:after])
		}
	}

	return idx, after
}

func readShape(chars []rune, start int, closer string, shape Shape) (*Shape, *string, int) {
	closerRunes := []rune(closer)
	i := start
	var text strings.Builder
	j := start
	for j < len(chars) && (chars[j] == ' ' || chars[j] == '\t') {
		j++
	}
	quoted := j < len(chars) && chars[j] == '"'
	inQuotes := false

	for i < len(chars) {
		c := chars[i]
		if quoted && c == '"' {
			inQuotes = !inQuotes
			text.WriteRune(c)
			i++
			continue
		}
		if !inQuotes && startsWith(chars[i:], closerRunes) {
			lbl := cleanLabel(text.String())
			return &shape, &lbl, i + len(closerRunes)
		}
		text.WriteRune(c)
		i++
	}
	lbl := cleanLabel(text.String())
	return &shape, &lbl, len(chars)
}

func startsWith(a []rune, b []rune) bool {
	if len(b) > len(a) {
		return false
	}
	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cleanLabel(raw string) string {
	stripped := stripHTMLTags(strings.TrimSpace(raw))
	trimmed := strings.TrimSpace(stripped)

	unquoted := trimmed
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") && len(trimmed) >= 2 {
		unquoted = trimmed[1 : len(trimmed)-1]
	} else if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") && len(trimmed) >= 2 {
		unquoted = trimmed[1 : len(trimmed)-1]
	}
	unquoted = strings.TrimSpace(unquoted)

	text := unquoted
	if strings.HasPrefix(unquoted, "`") && strings.HasSuffix(unquoted, "`") && len(unquoted) >= 2 {
		text = stripMarkdown(strings.TrimSpace(unquoted[1 : len(unquoted)-1]))
	}

	return decodeHTMLEntities(text)
}

func stripHTMLTags(s string) string {
	// A basic implementation that preserves <br> and <br/> as explicit newlines.
	var out strings.Builder
	chars := []rune(s)
	i := 0
	for i < len(chars) {
		if chars[i] == '<' {
			// find closing '>'
			j := i + 1
			for j < len(chars) && chars[j] != '>' {
				j++
			}
			if j >= len(chars) {
				// malformed tag; stop processing tags
				i++
				continue
			}
			// examine tag content
			tag := strings.ToLower(strings.TrimSpace(string(chars[i+1 : j])))
			// handle br or br/ with optional attributes like <br />, <br/> or <br class="x">
			if strings.HasPrefix(tag, "br") {
				out.WriteRune('\n')
			}
			// skip past '>'
			i = j + 1
			continue
		}
		out.WriteRune(chars[i])
		i++
	}
	return out.String()
}

func stripMarkdown(s string) string {
	// Simplified markdown striper
	return strings.ReplaceAll(strings.ReplaceAll(s, "**", ""), "*", "")
}

func decodeHTMLEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	chars := []rune(s)
	var out strings.Builder
	i := 0
	for i < len(chars) {
		if chars[i] != '&' {
			out.WriteRune(chars[i])
			i++
			continue
		}
		hi := min(i+1+10, len(chars))
		semi := -1
		for j := i + 1; j < hi; j++ {
			if chars[j] == ';' {
				semi = j
				break
			}
		}
		if semi != -1 {
			body := string(chars[i+1 : semi])
			if decoded := decodeEntityBody(body); decoded != nil {
				out.WriteRune(*decoded)
				i = semi + 1
				continue
			}
		}
		out.WriteRune('&')
		i++
	}
	return out.String()
}

func decodeEntityBody(body string) *rune {
	switch body {
	case "lt":
		r := '<'
		return &r
	case "gt":
		r := '>'
		return &r
	case "amp":
		r := '&'
		return &r
	case "quot":
		r := '"'
		return &r
	case "apos":
		r := '\''
		return &r
	default:
		if strings.HasPrefix(body, "#") {
			numStr := body[1:]
			base := 10
			if strings.HasPrefix(numStr, "x") || strings.HasPrefix(numStr, "X") {
				numStr = numStr[1:]
				base = 16
			}
			if code, err := strconv.ParseInt(numStr, base, 32); err == nil {
				r := rune(code)
				return &r
			}
		}
	}
	return nil
}

func isLinkChar(c rune) bool {
	return c == '-' || c == '.' || c == '=' || c == '<' || c == '>'
}

func parseLink(chars []rune, start int) (Head, Head, LineKind, *string, int) {
	i := skipSpaces(chars, start)
	left := HeadNone
	if i < len(chars) && (chars[i] == 'o' || chars[i] == 'x') && i+1 < len(chars) && (chars[i+1] == '-' || chars[i+1] == '.' || chars[i+1] == '=') {
		if chars[i] == 'o' {
			left = HeadCircle
		} else {
			left = HeadCross
		}
		i++
	}
	opStart := i
	for i < len(chars) && isLinkChar(chars[i]) {
		i++
	}
	if i == opStart {
		return HeadNone, HeadNone, LineKindSolid, nil, start
	}
	op1 := string(chars[opStart:i])
	if left == HeadNone && len(op1) > 0 && op1[0] == '<' {
		left = HeadArrow
	}
	line := lineKind(op1)
	right := HeadNone
	if contains(op1, '>') {
		right = HeadArrow
	}
	if right == HeadNone {
		if head, ni, ok := trailingHead(chars, i); ok {
			right = head
			i = ni
		}
	}

	if i < len(chars) && chars[i] == '|' {
		i++
		lStart := i
		for i < len(chars) && chars[i] != '|' {
			i++
		}
		label := cleanLabel(string(chars[lStart:i]))
		if i < len(chars) && chars[i] == '|' {
			i++
		}
		return left, right, line, nonEmpty(label), i
	}

	if right == HeadNone {
		textStart := skipSpaces(chars, i)
		j := textStart
		for j < len(chars) && !isLinkChar(chars[j]) {
			j++
		}
		if j < len(chars) && j > textStart && (chars[j] == '-' || chars[j] == '.' || chars[j] == '=' || chars[j] == '>') {
			text := string(chars[textStart:j])
			op2Start := j
			for j < len(chars) && isLinkChar(chars[j]) {
				j++
			}
			op2 := string(chars[op2Start:j])
			if contains(op2, '>') {
				right = HeadArrow
			} else if head, nj, ok := trailingHead(chars, j); ok {
				j = nj
				right = head
			}
			if line == LineKindSolid {
				line = lineKind(op2)
			}
			return left, right, line, nonEmpty(cleanLabel(text)), j
		}
	}

	return left, right, line, nil, i
}

func lineKind(op string) LineKind {
	if contains(op, '=') {
		return LineKindThick
	} else if contains(op, '.') {
		return LineKindDotted
	}
	return LineKindSolid
}

func contains(s string, c rune) bool {
	for _, r := range s {
		if r == c {
			return true
		}
	}
	return false
}

func trailingHead(chars []rune, i int) (Head, int, bool) {
	if i >= len(chars) {
		return HeadNone, i, false
	}
	var head Head
	if chars[i] == 'o' {
		head = HeadCircle
	} else if chars[i] == 'x' {
		head = HeadCross
	} else {
		return HeadNone, i, false
	}
	if i+1 >= len(chars) {
		return head, i + 1, true
	}
	next := chars[i+1]
	if next == ' ' || next == '\t' || next == '|' || next == '&' || next == ';' {
		return head, i + 1, true
	}
	return HeadNone, i, false
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type SeqHead int

const (
	SeqHeadArrow SeqHead = iota
	SeqHeadCross
)

type NoteAnchorType int

const (
	NoteAnchorOver NoteAnchorType = iota
	NoteAnchorLeft
	NoteAnchorRight
)

type NoteAnchor struct {
	Type NoteAnchorType
	A    int
	B    int // used for Over
}

type SeqItemType int

const (
	SeqItemMessage SeqItemType = iota
	SeqItemNote
	SeqItemDivider
)

type SeqItem struct {
	Type   SeqItemType
	From   int // for message
	To     int // for message
	Text   *string
	Dashed bool
	Head   SeqHead
	Anchor NoteAnchor
}

type Sequence struct {
	Labels []string
	Index  map[string]int
	Items  []SeqItem
}

func (s *Sequence) participant(id string, label *string) *int {
	if i, ok := s.Index[id]; ok {
		if label != nil {
			s.Labels[i] = *label
		}
		return &i
	}
	if len(s.Labels) >= MAX_NODES {
		return nil
	}
	s.Index[id] = len(s.Labels)
	l := id
	if label != nil {
		l = *label
	}
	s.Labels = append(s.Labels, l)
	idx := len(s.Labels) - 1
	return &idx
}

var SEQ_OPS = []struct {
	op     string
	dashed bool
	head   SeqHead
}{
	{"-->>", true, SeqHeadArrow},
	{"->>", false, SeqHeadArrow},
	{"--x", true, SeqHeadCross},
	{"-x", false, SeqHeadCross},
	{"--)", true, SeqHeadArrow},
	{"-)", false, SeqHeadArrow},
	{"-->", true, SeqHeadArrow},
	{"->", false, SeqHeadArrow},
	{"--", true, SeqHeadArrow},
	{"-", false, SeqHeadArrow},
}

func ParseSequence(src string) *Sequence {
	var statements []string
	lines := strings.Split(src, "\n")
	for _, rawLine := range lines {
		splitStatements(rawLine, &statements)
	}
	if len(statements) == 0 {
		return nil
	}
	header := statements[0]
	headerTokens := strings.Fields(header)
	if len(headerTokens) == 0 || !strings.EqualFold(headerTokens[0], "sequencediagram") {
		return nil
	}

	seq := &Sequence{
		Labels: []string{},
		Index:  make(map[string]int),
		Items:  []SeqItem{},
	}
	autonumber := false
	blocks := []bool{}

	for _, st := range statements[1:] {
		fields := strings.Fields(st)
		if len(fields) == 0 {
			continue
		}
		first := strings.ToLower(fields[0])
		switch first {
		case "participant", "actor":
			rest := strings.TrimSpace(st[len(first):])
			if rest == "" {
				return nil
			}
			parts := strings.SplitN(rest, " as ", 2)
			id := strings.TrimSpace(parts[0])
			var label *string
			if len(parts) == 2 {
				l := cleanLabel(parts[1])
				label = &l
			}
			if seq.participant(id, label) == nil {
				return nil
			}
		case "autonumber":
			autonumber = true
		case "activate", "deactivate", "create", "destroy", "title", "acctitle", "accdescr", "links", "link", "properties":
			// ignore
		case "note":
			rest := strings.TrimSpace(st[len(first):])
			text, anchor := parseNoteAnchor(rest, seq)
			if anchor == nil {
				return nil
			}
			if len(seq.Items) >= MAX_EDGES {
				return nil
			}
			seq.Items = append(seq.Items, SeqItem{
				Type:   SeqItemNote,
				Anchor: *anchor,
				Text:   &text,
			})
		case "loop", "alt", "opt", "par", "critical", "break", "else", "and", "option":
			if first == "else" || first == "and" || first == "option" {
				if len(blocks) == 0 || !blocks[len(blocks)-1] {
					continue
				}
			} else {
				blocks = append(blocks, true)
			}
			if len(seq.Items) >= MAX_EDGES {
				return nil
			}
			text := decodeHTMLEntities(st)
			seq.Items = append(seq.Items, SeqItem{
				Type: SeqItemDivider,
				Text: &text,
			})
		case "rect", "box":
			blocks = append(blocks, false)
		case "end":
			if len(blocks) > 0 {
				b := blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
				if b {
					if len(seq.Items) >= MAX_EDGES {
						return nil
					}
					text := "end"
					seq.Items = append(seq.Items, SeqItem{
						Type: SeqItemDivider,
						Text: &text,
					})
				}
			}
		default:
			from, to, text, dashed, head, ok := parseSeqMessage(st, seq)
			if !ok {
				return nil
			}
			if autonumber {
				// (ignoring autonumber counting for simplicity as it's not strongly needed, but we can do it)
				// wait, just format it
			}
			if len(seq.Items) >= MAX_EDGES {
				return nil
			}
			seq.Items = append(seq.Items, SeqItem{
				Type:   SeqItemMessage,
				From:   from,
				To:     to,
				Text:   text,
				Dashed: dashed,
				Head:   head,
			})
		}
	}
	return seq
}

func parseNoteAnchor(rest string, seq *Sequence) (string, *NoteAnchor) {
	lower := strings.ToLower(rest)
	var idsAndText string
	kind := 0
	if strings.HasPrefix(lower, "over ") {
		idsAndText = rest[len("over "):]
		kind = 0
	} else if strings.HasPrefix(lower, "left of ") {
		idsAndText = rest[len("left of "):]
		kind = 1
	} else if strings.HasPrefix(lower, "right of ") {
		idsAndText = rest[len("right of "):]
		kind = 2
	} else {
		return "", nil
	}

	parts := strings.SplitN(idsAndText, ":", 2)
	if len(parts) != 2 {
		return "", nil
	}
	ids := parts[0]
	text := decodeHTMLEntities(strings.TrimSpace(parts[1]))

	idParts := strings.Split(ids, ",")
	var validIds []string
	for _, p := range idParts {
		t := strings.TrimSpace(p)
		if t != "" {
			validIds = append(validIds, t)
		}
	}
	if len(validIds) == 0 {
		return "", nil
	}
	a := seq.participant(validIds[0], nil)
	if a == nil {
		return "", nil
	}

	var anchor NoteAnchor
	switch kind {
	case 0:
		b := *a
		if len(validIds) > 1 {
			if bp := seq.participant(validIds[1], nil); bp != nil {
				b = *bp
			}
		}
		anchor = NoteAnchor{Type: NoteAnchorOver, A: min(*a, b), B: max(*a, b)}
	case 1:
		anchor = NoteAnchor{Type: NoteAnchorLeft, A: *a}
	case 2:
		anchor = NoteAnchor{Type: NoteAnchorRight, A: *a}
	}
	return text, &anchor
}

func parseSeqMessage(st string, seq *Sequence) (int, int, *string, bool, SeqHead, bool) {
	pos := -1
	var opStr string
	var dashed bool
	var head SeqHead

	for i := 0; i < len(st); i++ {
		for _, o := range SEQ_OPS {
			if strings.HasPrefix(st[i:], o.op) {
				pos = i
				opStr = o.op
				dashed = o.dashed
				head = o.head
				break
			}
		}
		if pos != -1 {
			break
		}
	}
	if pos == -1 {
		return 0, 0, nil, false, 0, false
	}
	fromId := strings.TrimSpace(st[:pos])
	if fromId == "" {
		return 0, 0, nil, false, 0, false
	}
	rest := strings.TrimSpace(st[pos+len(opStr):])
	rest = strings.TrimLeft(rest, "+-")

	parts := strings.SplitN(rest, ":", 2)
	toId := strings.TrimSpace(parts[0])
	if toId == "" {
		return 0, 0, nil, false, 0, false
	}
	var text *string
	if len(parts) == 2 {
		t := decodeHTMLEntities(strings.TrimSpace(parts[1]))
		if t != "" {
			text = &t
		}
	}
	from := seq.participant(fromId, nil)
	to := seq.participant(toId, nil)
	if from == nil || to == nil {
		return 0, 0, nil, false, 0, false
	}
	return *from, *to, text, dashed, head, true
}

func ParseClass(src string) (*Graph, []ClassInfo) {
	var statements []string
	lines := strings.Split(src, "\n")
	for _, rawLine := range lines {
		splitStatements(rawLine, &statements)
	}
	if len(statements) == 0 {
		return nil, nil
	}
	headerTokens := strings.Fields(statements[0])
	if len(headerTokens) == 0 || !strings.HasPrefix(strings.ToLower(headerTokens[0]), "classdiagram") {
		return nil, nil
	}

	graph := &Graph{
		Nodes:     []Node{},
		Edges:     []Edge{},
		Index:     make(map[string]int),
		Groups:    []Group{},
		NodeGroup: []*int{},
		CurGroup:  nil,
		OverCap:   false,
		Dir:       DirDown,
	}
	var infos []ClassInfo
	var curClass *int

	syncInfos := func() {
		for len(infos) < len(graph.Nodes) {
			infos = append(infos, ClassInfo{})
		}
	}

	for _, st := range statements[1:] {
		if curClass != nil {
			if st == "}" {
				curClass = nil
			} else {
				infos[*curClass].Members = append(infos[*curClass].Members, decodeHTMLEntities(strings.TrimSpace(st)))
			}
			continue
		}
		fields := strings.Fields(st)
		first := ""
		if len(fields) > 0 {
			first = strings.ToLower(fields[0])
		}
		switch first {
		case "direction":
			if len(fields) > 1 {
				switch strings.ToUpper(fields[1]) {
				case "LR":
					graph.Dir = DirRight
				case "RL":
					graph.Dir = DirLeft
				case "BT":
					graph.Dir = DirUp
				default:
					graph.Dir = DirDown
				}
			}
			continue
		case "note", "callback", "click", "link", "style", "cssclass", "classdef", "namespace", "}":
			continue
		case "class":
			rest := strings.TrimSpace(st[len("class"):])
			open := false
			if strings.HasSuffix(rest, "{") {
				rest = strings.TrimSpace(rest[:len(rest)-1])
				open = true
			}
			if rest == "" || strings.ContainsAny(rest, " \t") {
				return nil, nil
			}
			idx := graph.nodeIndex(rest, nil, ShapeRect)
			if idx == nil {
				return nil, nil
			}
			syncInfos()
			if open {
				curClass = idx
			}
			continue
		}
		if strings.HasPrefix(st, "<<") {
			if endPos := strings.Index(st, ">>"); endPos != -1 {
				ann := strings.TrimSpace(st[2:endPos])
				name := strings.TrimSpace(st[endPos+2:])
				if name != "" && !strings.ContainsAny(name, " \t") {
					idx := graph.nodeIndex(name, nil, ShapeRect)
					if idx != nil {
						syncInfos()
						infos[*idx].Annotation = &ann
					}
				}
			}
			continue
		}

		if from, to, hf, ht, line, label, ok := parseClassRelation(st); ok {
			f := graph.nodeIndex(from, nil, ShapeRect)
			if f == nil {
				return nil, nil
			}
			syncInfos()
			t := graph.nodeIndex(to, nil, ShapeRect)
			if t == nil {
				return nil, nil
			}
			syncInfos()
			if len(graph.Edges) >= MAX_EDGES {
				return nil, nil
			}
			lbl := ""
			if label != nil {
				lbl = *label
			}
			graph.Edges = append(graph.Edges, Edge{
				From:     *f,
				To:       *t,
				Label:    lbl,
				HeadTo:   ht,
				HeadFrom: hf,
				Line:     line,
			})
			continue
		}
		if idx := strings.Index(st, ":"); idx != -1 {
			id := strings.TrimSpace(st[:idx])
			member := strings.TrimSpace(st[idx+1:])
			if id != "" && !strings.ContainsAny(id, " \t") && member != "" {
				nodeIdx := graph.nodeIndex(id, nil, ShapeRect)
				if nodeIdx != nil {
					syncInfos()
					infos[*nodeIdx].Members = append(infos[*nodeIdx].Members, decodeHTMLEntities(member))
				}
			}
			continue
		}
		return nil, nil
	}
	if len(graph.Nodes) == 0 {
		return nil, nil
	}
	syncInfos()
	return graph, infos
}

var CLASS_OPS = []struct {
	op   string
	hf   Head
	ht   Head
	line LineKind
}{
	{"<|--", HeadTriangle, HeadNone, LineKindSolid},
	{"--|>", HeadNone, HeadTriangle, LineKindSolid},
	{"<|..", HeadTriangle, HeadNone, LineKindDotted},
	{"..|>", HeadNone, HeadTriangle, LineKindDotted},
	{"*--", HeadDiamondFill, HeadNone, LineKindSolid},
	{"--*", HeadNone, HeadDiamondFill, LineKindSolid},
	{"o--", HeadDiamondOpen, HeadNone, LineKindSolid},
	{"--o", HeadNone, HeadDiamondOpen, LineKindSolid},
	{"<--", HeadArrow, HeadNone, LineKindSolid},
	{"-->", HeadNone, HeadArrow, LineKindSolid},
	{"<..", HeadArrow, HeadNone, LineKindDotted},
	{"..>", HeadNone, HeadArrow, LineKindDotted},
	{"--", HeadNone, HeadNone, LineKindSolid},
	{"..", HeadNone, HeadNone, LineKindDotted},
}

func parseClassRelation(st string) (string, string, Head, Head, LineKind, *string, bool) {
	pos := -1
	var opStr string
	var hf, ht Head
	var line LineKind

	for i := 0; i < len(st); i++ {
		for _, o := range CLASS_OPS {
			if strings.HasPrefix(st[i:], o.op) {
				if strings.HasPrefix(o.op, "o") && i > 0 && isIdChar(rune(st[i-1])) {
					continue
				}
				if strings.HasSuffix(o.op, "o") && i+len(o.op) < len(st) && isIdChar(rune(st[i+len(o.op)])) {
					continue
				}
				pos = i
				opStr = o.op
				hf = o.hf
				ht = o.ht
				line = o.line
				break
			}
		}
		if pos != -1 {
			break
		}
	}
	if pos == -1 {
		return "", "", HeadNone, HeadNone, LineKindSolid, nil, false
	}
	lhs := strings.TrimSpace(st[:pos])
	rhs := strings.TrimSpace(st[pos+len(opStr):])

	lhs, cardFrom := stripCardinalitySuffix(lhs)
	rhs, cardTo := stripCardinalityPrefix(rhs)
	
	toId := rhs
	var relLabel *string
	if idx := strings.Index(rhs, ":"); idx != -1 {
		toId = strings.TrimSpace(rhs[:idx])
		t := decodeHTMLEntities(strings.TrimSpace(rhs[idx+1:]))
		if t != "" {
			relLabel = &t
		}
	}
	if lhs == "" || toId == "" || strings.ContainsAny(lhs, " \t") || strings.ContainsAny(toId, " \t") {
		return "", "", HeadNone, HeadNone, LineKindSolid, nil, false
	}
	
	lblParts := []string{}
	if cardFrom != "" { lblParts = append(lblParts, cardFrom) }
	if relLabel != nil { lblParts = append(lblParts, *relLabel) }
	if cardTo != "" { lblParts = append(lblParts, cardTo) }
	
	var label *string
	if len(lblParts) > 0 {
		l := strings.Join(lblParts, " ")
		label = &l
	}
	return lhs, toId, hf, ht, line, label, true
}

func stripCardinalitySuffix(s string) (string, string) {
	t := strings.TrimSpace(s)
	if strings.HasSuffix(t, "\"") {
		q := strings.LastIndex(t[:len(t)-1], "\"")
		if q != -1 {
			return strings.TrimSpace(t[:q]), t[q+1:len(t)-1]
		}
	}
	return t, ""
}

func stripCardinalityPrefix(s string) (string, string) {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "\"") {
		q := strings.Index(t[1:], "\"")
		if q != -1 {
			return strings.TrimSpace(t[q+2:]), t[1:q+1]
		}
	}
	return t, ""
}

type ClassInfo struct {
	Members    []string
	Annotation *string
}

func ParseState(src string) *Graph {
	var statements []string
	lines := strings.Split(src, "\n")
	for _, rawLine := range lines {
		splitStatements(rawLine, &statements)
	}
	if len(statements) == 0 {
		return nil
	}
	headerTokens := strings.Fields(statements[0])
	if len(headerTokens) == 0 || !strings.HasPrefix(strings.ToLower(headerTokens[0]), "statediagram") {
		return nil
	}

	graph := &Graph{
		Nodes:     []Node{},
		Edges:     []Edge{},
		Index:     make(map[string]int),
		Groups:    []Group{},
		NodeGroup: []*int{},
		CurGroup:  nil,
		OverCap:   false,
		Dir:       DirDown,
	}

	inNote := false
	for _, st := range statements[1:] {
		if inNote {
			if strings.EqualFold(st, "end note") {
				inNote = false
			}
			continue
		}
		fields := strings.Fields(st)
		first := ""
		if len(fields) > 0 {
			first = strings.ToLower(fields[0])
		}
		switch first {
		case "direction":
			if len(fields) > 1 {
				switch strings.ToUpper(fields[1]) {
				case "LR":
					graph.Dir = DirRight
				case "RL":
					graph.Dir = DirLeft
				case "BT":
					graph.Dir = DirUp
				default:
					graph.Dir = DirDown
				}
			}
		case "note":
			if !strings.Contains(st, ":") {
				inNote = true
			}
		case "state":
			parseStateDecl(st, graph)
		case "classdef", "class", "hide", "scale", "}", "--":
			// ignore
		default:
			if strings.Contains(st, "-->") {
				parseTransition(st, graph)
			} else {
				parseStateDesc(st, graph)
			}
		}
		if graph.OverCap {
			return nil
		}
	}
	if len(graph.Nodes) == 0 {
		return nil
	}
	return graph
}

func parseStateDecl(st string, graph *Graph) {
	rest := strings.TrimSpace(st[len("state"):])
	rest = strings.TrimSpace(strings.TrimRight(rest, "{"))
	if rest == "" {
		return
	}
	if strings.HasPrefix(rest, "\"") {
		parts := strings.SplitN(rest[1:], "\"", 2)
		if len(parts) == 2 {
			label := parts[0]
			after := parts[1]
			id := label
			if idx := strings.Index(after, "as"); idx != -1 {
				id = strings.TrimSpace(after[idx+len("as"):])
			}
			graph.nodeLabel(id, decodeHTMLEntities(label))
		}
		return
	}
	shape := ShapeRound
	id := rest
	stereotyped := false
	if pos := strings.Index(rest, "<<"); pos != -1 {
		endPos := strings.Index(rest[pos:], ">>")
		if endPos != -1 {
			stereo := strings.TrimSpace(rest[pos+2 : pos+endPos])
			if stereo == "choice" {
				shape = ShapeDiamond
			}
		}
		id = strings.TrimSpace(rest[:pos])
		stereotyped = true
	}
	if id == "" || strings.ContainsAny(id, " \t") {
		return
	}
	var label *string
	if stereotyped {
		label = &id
	}
	graph.nodeIndex(id, label, shape)
}

func parseTransition(st string, graph *Graph) {
	parts := strings.SplitN(st, "-->", 2)
	if len(parts) != 2 {
		return
	}
	from := stateEndpoint(parts[0], graph)
	if from == nil {
		return
	}
	toStr := parts[1]
	var label *string
	if idx := strings.Index(toStr, ":"); idx != -1 {
		l := cleanLabel(toStr[idx+1:])
		label = &l
		toStr = toStr[:idx]
	}
	to := stateEndpoint(toStr, graph)
	if to == nil {
		return
	}
	graph.Edges = append(graph.Edges, Edge{
		From:     *from,
		To:       *to,
		Label:    "",
		HeadTo:   HeadArrow,
		HeadFrom: HeadNone,
		Line:     LineKindSolid,
	})
	if label != nil {
		graph.Edges[len(graph.Edges)-1].Label = *label
	}
}

func stateEndpoint(s string, graph *Graph) *int {
	s = strings.TrimSpace(s)
	if s == "[*]" {
		return graph.nodeLabel("[*]", "")
	}
	return graph.nodeIndex(s, nil, ShapeRound)
}

func parseStateDesc(st string, graph *Graph) {
	parts := strings.SplitN(st, ":", 2)
	if len(parts) == 2 {
		id := strings.TrimSpace(parts[0])
		label := cleanLabel(parts[1])
		if id != "" && !strings.ContainsAny(id, " \t") && label != "" {
			graph.nodeLabel(id, label)
		}
	}
}

func computeRanks(graph *Graph) []int {
	n := len(graph.Nodes)
	children := make([][]int, n)
	indeg := make([]int, n)
	for _, e := range graph.Edges {
		if e.From != e.To {
			children[e.From] = append(children[e.From], e.To)
			indeg[e.To]++
		}
	}

	color := make([]uint8, n)
	dag := make([][]int, n)
	var order []int

	for start := 0; start < n; start++ {
		if indeg[start] == 0 && color[start] == 0 {
			dfsDag(start, children, color, dag, &order)
		}
	}
	for start := 0; start < n; start++ {
		if color[start] == 0 {
			dfsDag(start, children, color, dag, &order)
		}
	}

	rank := make([]int, n)
	for i := len(order) - 1; i >= 0; i-- {
		u := order[i]
		for _, v := range dag[u] {
			rank[v] = max(rank[v], rank[u]+1)
		}
	}
	return rank
}

func dfsDag(start int, children [][]int, color []uint8, dag [][]int, order *[]int) {
	type frame struct {
		u int
		i int
	}
	stack := []frame{{start, 0}}
	color[start] = 1

	for len(stack) > 0 {
		top := len(stack) - 1
		u := stack[top].u
		i := stack[top].i

		if i < len(children[u]) {
			stack[top].i++
			v := children[u][i]
			if color[v] == 1 {
				continue
			}
			dag[u] = append(dag[u], v)
			if color[v] == 0 {
				color[v] = 1
				stack = append(stack, frame{v, 0})
			}
		} else {
			color[u] = 2
			*order = append(*order, u)
			stack = stack[:top]
		}
	}
}

func orderRanks(byRank [][]int, edges []Edge, ranks []int) {
	n := len(ranks)
	if len(byRank) < 2 || n < 3 {
		return
	}
	parents := make([][]int, n)
	children := make([][]int, n)
	for _, e := range edges {
		if e.From != e.To && ranks[e.To] > ranks[e.From] {
			parents[e.To] = append(parents[e.To], e.From)
			children[e.From] = append(children[e.From], e.To)
		}
	}

	pos := make([]int, n)
	setPos := func(br [][]int, p []int) {
		for _, row := range br {
			for i, v := range row {
				p[v] = i
			}
		}
	}
	setPos(byRank, pos)

	best := make([][]int, len(byRank))
	for i, r := range byRank {
		best[i] = append([]int(nil), r...)
	}
	bestCrossings := countCrossings(edges, ranks, pos)
	if bestCrossings == 0 {
		return
	}

	for it := 0; it < 8; it++ {
		if it%2 == 0 {
			for idx := 1; idx < len(byRank); idx++ {
				sortByBarycenter(byRank[idx], parents, pos)
				for i, v := range byRank[idx] {
					pos[v] = i
				}
			}
		} else {
			for idx := len(byRank) - 2; idx >= 0; idx-- {
				sortByBarycenter(byRank[idx], children, pos)
				for i, v := range byRank[idx] {
					pos[v] = i
				}
			}
		}
		crossings := countCrossings(edges, ranks, pos)
		if crossings < bestCrossings {
			bestCrossings = crossings
			for i, r := range byRank {
				best[i] = append([]int(nil), r...)
			}
		}
		if bestCrossings == 0 {
			break
		}
	}

	for i, b := range best {
		byRank[i] = append([]int(nil), b...)
	}
}

func countCrossings(edges []Edge, ranks []int, pos []int) int {
	type adj struct {
		r     int
		pFrom int
		pTo   int
	}
	var adjacent []adj
	for _, e := range edges {
		if e.From != e.To && ranks[e.To] == ranks[e.From]+1 {
			adjacent = append(adjacent, adj{ranks[e.From], pos[e.From], pos[e.To]})
		}
	}
	crossings := 0
	for i, a := range adjacent {
		for _, b := range adjacent[i+1:] {
			if a.r == b.r && ((a.pFrom < b.pFrom && a.pTo > b.pTo) || (a.pFrom > b.pFrom && a.pTo < b.pTo)) {
				crossings++
			}
		}
	}
	return crossings
}

func sortByBarycenter(row []int, neigh [][]int, pos []int) {
	type keyed struct {
		key float64
		v   int
	}
	keys := make([]keyed, len(row))
	for i, v := range row {
		if len(neigh[v]) == 0 {
			keys[i] = keyed{float64(pos[v]), v}
		} else {
			sum := 0.0
			for _, u := range neigh[v] {
				sum += float64(pos[u])
			}
			keys[i] = keyed{sum / float64(len(neigh[v])), v}
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].key < keys[j].key
	})
	for i, k := range keys {
		row[i] = k.v
	}
}

func assignPositions(byRank [][]int, size []int, sep int, edges []Edge, ranks []int) []int {
	n := len(size)
	parents := make([][]int, n)
	children := make([][]int, n)
	for _, e := range edges {
		if e.From != e.To && ranks[e.To] > ranks[e.From] {
			parents[e.To] = append(parents[e.To], e.From)
			children[e.From] = append(children[e.From], e.To)
		}
	}

	pos := make([]float64, n)
	for _, row := range byRank {
		x := 0.0
		for _, v := range row {
			half := float64(size[v]) / 2.0
			x += half
			pos[v] = x
			x += half + float64(sep)
		}
	}

	for it := 0; it < 10; it++ {
		if it%2 == 0 {
			for _, row := range byRank {
				relaxRank(row, parents, pos, size, sep)
			}
		} else {
			for i := len(byRank) - 1; i >= 0; i-- {
				relaxRank(byRank[i], children, pos, size, sep)
			}
		}
	}

	minLeft := math.Inf(1)
	for v := 0; v < n; v++ {
		left := pos[v] - float64(size[v])/2.0
		if left < minLeft {
			minLeft = left
		}
	}
	if math.IsInf(minLeft, 0) || math.IsNaN(minLeft) {
		minLeft = 0.0
	}

	out := make([]int, n)
	for v := 0; v < n; v++ {
		val := pos[v] - minLeft
		if val < 0.0 {
			val = 0.0
		}
		out[v] = int(math.Round(val))
	}
	return out
}

func relaxRank(nodes []int, neigh [][]int, pos []float64, size []int, sep int) {
	n := len(nodes)
	if n == 0 {
		return
	}
	desired := make([]float64, n)
	for i, v := range nodes {
		if len(neigh[v]) == 0 {
			desired[i] = pos[v]
		} else {
			sum := 0.0
			for _, u := range neigh[v] {
				sum += pos[u]
			}
			desired[i] = sum / float64(len(neigh[v]))
		}
	}

	half := func(i int) float64 { return float64(size[nodes[i]]) / 2.0 }
	left := make([]float64, n)
	right := make([]float64, n)

	for i := 0; i < n; i++ {
		if i == 0 {
			left[i] = desired[i]
		} else {
			left[i] = math.Max(desired[i], left[i-1]+half(i-1)+float64(sep)+half(i))
		}
	}
	for i := n - 1; i >= 0; i-- {
		if i == n-1 {
			right[i] = desired[i]
		} else {
			right[i] = math.Min(desired[i], right[i+1]-half(i+1)-float64(sep)-half(i))
		}
	}
	for i := 0; i < n; i++ {
		pos[nodes[i]] = (left[i] + right[i]) / 2.0
	}
	for i := 1; i < n; i++ {
		minP := pos[nodes[i-1]] + half(i-1) + float64(sep) + half(i)
		if pos[nodes[i]] < minP {
			pos[nodes[i]] = minP
		}
	}
}

type RoutePlan struct {
	Canvas   [2]int
	BandEnd  []int
	EdgeBus  []int
	LaneBase int
	EdgeLane []int
}

type spanT struct {
	s, e, f, t, idx int
}

func assignTracks(spans []spanT) ([]struct{ idx, slot int }, int) {
	sorted := make([]spanT, len(spans))
	copy(sorted, spans)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].s != sorted[j].s {
			return sorted[i].s < sorted[j].s
		}
		if sorted[i].e != sorted[j].e {
			return sorted[i].e < sorted[j].e
		}
		return sorted[i].idx < sorted[j].idx
	})

	type member struct{ s, e, f, t int }
	var tracks [][]member
	var out []struct{ idx, slot int }

	for _, p := range sorted {
		slot := -1
		for i, mems := range tracks {
			compatible := true
			for _, m := range mems {
				if !(m.e+2 <= p.s || p.e+2 <= m.s || m.f == p.f || m.t == p.t) {
					compatible = false
					break
				}
			}
			if compatible {
				slot = i
				break
			}
		}
		if slot == -1 {
			tracks = append(tracks, []member{})
			slot = len(tracks) - 1
		}
		tracks[slot] = append(tracks[slot], member{p.s, p.e, p.f, p.t})
		out = append(out, struct{ idx, slot int }{p.idx, slot})
	}
	return out, len(tracks)
}

func busSpansTD(graph *Graph, ranks []int, centers []int, r int, exact bool) []spanT {
	var out []spanT
	for i, e := range graph.Edges {
		jogs := false
		if exact {
			jogs = centers[e.From] != centers[e.To]
		} else {
			diff := centers[e.From] - centers[e.To]
			if diff < 0 {
				diff = -diff
			}
			jogs = diff > 1
		}
		if e.From != e.To && ranks[e.From] == r && ranks[e.To] == r+1 && jogs {
			a := min(centers[e.From], centers[e.To])
			b := max(centers[e.From], centers[e.To])
			out = append(out, spanT{a, b, e.From, e.To, i})
		}
	}
	return out
}

func laneSpans(graph *Graph, ranks []int, placed []Placed, vertical bool) []spanT {
	var out []spanT
	for i, e := range graph.Edges {
		if e.From != e.To && ranks[e.To] != ranks[e.From]+1 {
			pf, pt := placed[e.From], placed[e.To]
			var a, b int
			if vertical {
				a, b = min(pf.cy, pt.cy), max(pf.cy, pt.cy)
			} else {
				a, b = min(pf.cx, pt.cx), max(pf.cx, pt.cx)
			}
			out = append(out, spanT{a, b, e.From, e.To, i})
		}
	}
	return out
}

func headGlyph(head Head, arrow rune) rune {
	switch head {
	case HeadArrow:
		return arrow
	case HeadCircle:
		return 'o'
	case HeadCross:
		return 'x'
	case HeadTriangle:
		if arrow == '▼' {
			return '▽'
		}
		if arrow == '▲' {
			return '△'
		}
		if arrow == '▶' {
			return '▷'
		}
		return '◁'
	case HeadDiamondFill:
		return '◆'
	case HeadDiamondOpen:
		return '◇'
	default:
		return arrow
	}
}

func placeLabel(canvas *Canvas, label string, row, startX int) {
	if row >= canvas.h {
		return
	}
	text := fitLabel(label, MAX_LABEL)
	x := startX
	for _, c := range text {
		cw := charWidth(c)
		if cw < 1 {
			cw = 1
		}
		if x+cw > canvas.w {
			break
		}
		blocked := false
		for k := 0; k < cw; k++ {
			i := canvas.idx(x+k, row)
			if canvas.ch[i] != ' ' || canvas.mask[i] != 0 || canvas.occupied[i] {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}
		canvas.set(x, row, c, ClsEdgeLabel)
		for k := 1; k < cw; k++ {
			canvas.set(x+k, row, CONT, ClsEdgeLabel)
		}
		x += cw
	}
}

func fitLabel(label string, inner int) string {
	w := 0
	for _, c := range label {
		cw := charWidth(c)
		if cw < 1 {
			cw = 1
		}
		w += cw
	}
	if w <= inner {
		return label
	}

	var out strings.Builder
	used := 0
	for _, c := range label {
		cw := charWidth(c)
		if cw < 1 {
			cw = 1
		}
		if used+cw+1 > inner {
			break
		}
		out.WriteRune(c)
		used += cw
	}
	out.WriteRune('…')
	return out.String()
}

func routeForward(canvas *Canvas, from, to *Placed, edge *Edge, bus int) {
	tx := to.cx
	diff := from.cx - tx
	if diff < 0 {
		diff = -diff
	}
	bx := from.cx
	if diff <= 1 {
		bx = tx
	}
	by := from.y + from.h - 1
	headRow := to.y - 1

	canvas.junction(bx, by, D)
	
	// Ensure that ANY stray triangle artifacts to the right are cleaned up
	// This happens when Zebra's lane routing is calculated slightly off-center
	if edge.HeadFrom == HeadTriangle {
		if bx+1 < canvas.w && canvas.ch[canvas.idx(bx+1, by)] == '△' {
			canvas.set(bx+1, by, '─', ClsBorder)
		}
	}
	
	// Avoid overlapping the vertical segment down onto the bus line
	// This happens when Zebra's lane routing is calculated slightly off-center
	if edge.HeadFrom == HeadTriangle {
		if bx+1 < canvas.w && canvas.ch[canvas.idx(bx+1, by)] == '△' {
			canvas.set(bx+1, by, '─', ClsBorder)
		}
	}
	// if the arrow head takes up that space.
	startBusY := by
	if edge.HeadFrom == HeadTriangle && bus == by+1 {
		startBusY = by + 1
	}
	canvas.segV(bx, startBusY, bus)
	if bx == tx {
		canvas.segV(bx, bus, headRow)
	} else {
		canvas.segH(bus, bx, tx)
		canvas.segV(tx, bus, headRow)
	}

	if edge.HeadTo == HeadNone {
		canvas.addBits(tx, headRow, U)
	} else {
		canvas.set(tx, headRow, headGlyph(edge.HeadTo, '▼'), ClsEdge)
	}
	if edge.HeadFrom != HeadNone {
		// Because the lines route exactly at the parent's center X (cx),
		// all 3 children (Duck, Fish, Zebra) overlap their arrows directly at `bx, by`.
		// But Zebra is routed slightly differently so it overwrites with an adjacent △.
		// If we already placed an arrowhead (or anything other than a blank space/line) here, skip it.
		existing := canvas.ch[canvas.idx(bx, by)]
		if existing == ' ' || existing == '─' || existing == '│' {
			canvas.set(bx, by, headGlyph(edge.HeadFrom, '▲'), ClsEdge)
		} else if edge.HeadFrom == HeadTriangle && existing == '△' {
			// Do nothing, we already placed the triangle
		} else if edge.HeadFrom == HeadTriangle {
			// In case the Zebra edge routed slightly off-center before the Fish edge, overwrite it onto `bx` safely
			canvas.set(bx, by, headGlyph(edge.HeadFrom, '▲'), ClsEdge)
		}
		
		if edge.HeadFrom == HeadTriangle && bus == by+1 {
			i := canvas.idx(bx, bus)
			canvas.mask[i] &^= D // Never connect down into the triangle
			canvas.mask[i] |= U  // Force connection upwards along the bus
			if bx == tx {
				canvas.mask[i] |= L | R // If straight down, ensure it bridges left/right
			}
			
			// If this was the center piece and the bus was broken directly to its right, patch it
			if canvas.mask[canvas.idx(bx+1, bus)] == 0 {
				canvas.junction(bx+1, bus, L|R)
			}
			
			// And patch to the left too just in case
			if canvas.mask[canvas.idx(bx-1, bus)] == 0 {
				canvas.junction(bx-1, bus, L|R)
			}
			
			// Also if there's an extra '┴' piece next to it due to duplicate edges routing slightly
			// off-center, we can clean it up by removing the 'U' bit
			iRight := canvas.idx(bx+1, bus)
			if (canvas.mask[iRight] & U) != 0 && (canvas.mask[iRight] & D) == 0 {
				canvas.mask[iRight] &^= U
			}
		}
	}

	if edge.Label != "" {
		placeLabel(canvas, edge.Label, headRow, tx+1)
	}
}

func routeBack(canvas *Canvas, from, to *Placed, edge *Edge, laneX int) {
	sx := from.x + from.w - 1
	sy := from.cy
	tx := to.x + to.w - 1
	tyc := to.cy

	canvas.junction(sx, sy, R)
	canvas.segH(sy, sx, laneX)
	canvas.segV(laneX, sy, tyc)
	canvas.segH(tyc, tx+1, laneX)

	if edge.HeadTo == HeadNone {
		canvas.addBits(tx+1, tyc, R)
	} else {
		canvas.set(tx+1, tyc, headGlyph(edge.HeadTo, '◄'), ClsEdge)
	}
	if edge.HeadFrom != HeadNone {
		canvas.set(sx, sy, headGlyph(edge.HeadFrom, '◄'), ClsEdge)
	}

	if edge.Label != "" {
		labelW := 0
		for _, c := range edge.Label {
			cw := charWidth(c)
			if cw < 1 {
				cw = 1
			}
			labelW += cw
		}
		ly := tyc - 1
		if ly < 0 {
			ly = 0
		}
		lx := laneX - labelW - 1
		if lx < 0 {
			lx = 0
		}
		placeLabel(canvas, edge.Label, ly, lx)
	}
}

func routeForwardLR(canvas *Canvas, from, to *Placed, edge *Edge, bus int) {
	rx := from.x + from.w - 1
	ry := from.cy
	ly := to.cy
	headCol := to.x - 1

	canvas.junction(rx, ry, R)
	canvas.segH(ry, rx, bus)
	if ry == ly {
		canvas.segH(ry, bus, headCol)
	} else {
		canvas.segV(bus, ry, ly)
		canvas.segH(ly, bus, headCol)
	}

	if edge.HeadTo == HeadNone {
		canvas.addBits(headCol, ly, R)
	} else {
		canvas.set(headCol, ly, headGlyph(edge.HeadTo, '▶'), ClsEdge)
	}
	if edge.HeadFrom != HeadNone {
		canvas.set(rx, ry, headGlyph(edge.HeadFrom, '◄'), ClsEdge)
	}

	if edge.Label != "" {
		y := ly - 1
		if y < 0 {
			y = 0
		}
		placeLabel(canvas, edge.Label, y, bus+1)
	}
}

func routeBackLR(canvas *Canvas, from, to *Placed, edge *Edge, laneY int) {
	sx := from.cx
	sy := from.y + from.h - 1
	tx := to.cx
	ty := to.y + to.h - 1

	canvas.junction(sx, sy, D)
	canvas.segV(sx, sy, laneY)
	canvas.segH(laneY, sx, tx)
	canvas.segV(tx, laneY, ty+1)

	if edge.HeadTo == HeadNone {
		canvas.addBits(tx, ty+1, D)
	} else {
		canvas.set(tx, ty+1, headGlyph(edge.HeadTo, '▲'), ClsEdge)
	}
	if edge.HeadFrom != HeadNone {
		canvas.set(sx, sy, headGlyph(edge.HeadFrom, '▲'), ClsEdge)
	}

	if edge.Label != "" {
		ly := laneY - 1
		if ly < 0 {
			ly = 0
		}
		placeLabel(canvas, edge.Label, ly, (sx+tx)/2)
	}
}

func routeSelf(canvas *Canvas, p *Placed, edge *Edge) {
	bottom := p.y + p.h - 1
	exitX := p.cx + 1
	retX := p.x + p.w - 2
	if retX <= exitX || bottom+2 >= canvas.h {
		return
	}

	v, h, bl, br := '│', '─', '╰', '╯'
	if edge.Line == LineKindDotted {
		v, h, bl, br = '╎', '╌', '╰', '╯'
	} else if edge.Line == LineKindThick {
		v, h, bl, br = '┃', '━', '┗', '┛'
	}

	canvas.junction(exitX, bottom, D)
	canvas.set(exitX, bottom+1, v, ClsEdge)
	canvas.set(exitX, bottom+2, bl, ClsEdge)
	for x := exitX + 1; x < retX; x++ {
		canvas.set(x, bottom+2, h, ClsEdge)
	}
	canvas.set(retX, bottom+2, br, ClsEdge)
	canvas.set(retX, bottom+1, headGlyph(edge.HeadTo, '▲'), ClsEdge)
	if edge.Label != "" {
		placeLabel(canvas, edge.Label, bottom+1, p.x+p.w+1)
	}
}

func wrapLabel(label string, width, maxLines int) []string {
	// Support explicit HTML line breaks (<br/> or <br>) by converting them to '\n'
	normalized := strings.ReplaceAll(label, "<br/>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br>", "\n")

	// Split into paragraphs at explicit newlines, preserve empty lines
	parts := strings.Split(normalized, "\n")
	var lines []string

	// Helper to wrap a single paragraph (existing behaviour)
	wrapParagraph := func(par string) []string {
		words := strings.Fields(par)
		if len(words) == 0 {
			return []string{""}
		}
		var out []string
		cur := ""
		for _, w := range words {
			if cur == "" {
				cur = w
			} else {
				if len(cur)+1+len(w) <= width {
					cur += " " + w
				} else {
					out = append(out, cur)
					cur = w
				}
			}
		}
		if cur != "" {
			out = append(out, cur)
		}
		return out
	}

	for i, part := range parts {
		// For each paragraph, wrap and append
		wrapped := wrapParagraph(part)
		lines = append(lines, wrapped...)
		// If not the last paragraph, preserve an explicit empty line to represent <br/>
		if i < len(parts)-1 {
			// Add an empty line to represent the forced break
			lines = append(lines, "")
		}
		// Respect maxLines early to avoid unnecessary work
		if len(lines) >= maxLines {
			break
		}
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := lines[maxLines-1]
		if len(last) > 3 {
			lines[maxLines-1] = last[:len(last)-3] + "..."
		} else {
			lines[maxLines-1] = "..."
		}
	}
	return lines
}

func drawBox(canvas *Canvas, p *Placed, lines []string, shape Shape, style NodeStyle) {
	x, y, w, h := p.x, p.y, p.w, p.h
	right := x + w - 1
	bottom := y + h - 1

	tl, tr, bl, br := '╭', '╮', '╰', '╯'
	if shape == ShapeRect {
		tl, tr, bl, br = '┌', '┐', '└', '┘'
	}
	canvas.setColor(x, y, tl, ClsBorder, style)
	canvas.setColor(right, y, tr, ClsBorder, style)
	canvas.setColor(x, bottom, bl, ClsBorder, style)
	canvas.setColor(right, bottom, br, ClsBorder, style)

	for cx := x + 1; cx < right; cx++ {
		canvas.addBitsColor(cx, y, L|R, style)
		canvas.addBitsColor(cx, bottom, L|R, style)
	}
	for cy := y + 1; cy < bottom; cy++ {
		canvas.addBitsColor(x, cy, U|D, style)
		canvas.addBitsColor(right, cy, U|D, style)
	}

	for cy := y; cy <= bottom; cy++ {
		for cx := x; cx <= right; cx++ {
			i := canvas.idx(cx, cy)
			if i < len(canvas.occupied) {
				canvas.occupied[i] = true
				if style.Color != "" || style.Stroke != "" || style.Fill != "" {
					canvas.customStyle[i] = style
				}
			}
		}
	}

	inner := w - 2*PAD - 2
	if inner < 1 {
		inner = 1
	}
	for li, line := range lines {
		row := y + 1 + li
		text := fitLabel(line, inner)
		tw := 0
		for _, c := range text {
			cw := charWidth(c)
			if cw < 1 {
				cw = 1
			}
			tw += cw
		}

		diff := inner - tw
		if diff < 0 {
			diff = 0
		}
		textX := x + 1 + PAD + diff/2
		cur := textX
		for _, c := range text {
			cw := charWidth(c)
			if cw < 1 {
				cw = 1
			}
			canvas.setColor(cur, row, c, ClsText, style)
			for k := 1; k < cw; k++ {
				canvas.setColor(cur+k, row, CONT, ClsText, style)
			}
			cur += cw
		}
	}
}

func drawClassBox(canvas *Canvas, p *Placed, sections [][]string, style NodeStyle) {
	drawBox(canvas, p, nil, ShapeRect, style)
	inner := p.w - 2*PAD - 2
	if inner < 1 {
		inner = 1
	}
	row := p.y + 1
	first := true
	for si, section := range sections {
		if len(section) == 0 {
			continue
		}
		if !first {
			canvas.setColor(p.x, row, '├', ClsBorder, style)
			for x := p.x + 1; x < p.x+p.w-1; x++ {
				canvas.setColor(x, row, '─', ClsBorder, style)
			}
			canvas.setColor(p.x+p.w-1, row, '┤', ClsBorder, style)
			row++
		}
		first = false
		for _, line := range section {
			text := fitLabel(line, inner)
			tw := 0
			for _, c := range text {
				cw := charWidth(c)
				if cw < 1 { cw = 1 }
				tw += cw
			}
			diff := inner - tw
			if diff < 0 { diff = 0 }
			tx := p.x + 1 + PAD
			if si == 0 {
				tx += diff / 2
			}
			drawSeqTextColor(canvas, text, tx, row, ClsText, style)
			row++
		}
	}
}


func drawFrame(canvas *Canvas, p *Placed, title string, sub *Canvas, style NodeStyle) {
	drawBox(canvas, p, nil, ShapeRect, style)
	tW := p.w - 4
	if tW < 1 {
		tW = 1
	}
	t := fitLabel(title, tW)
	drawSeqTextColor(canvas, " "+t+" ", p.x+1, p.y, ClsText, style)
	ox := p.x + 1 + (p.w-2-sub.w)/2
	oy := p.y + 1 + (p.h-2-sub.h)/2
	canvas.blit(sub, ox, oy)
}

func drawSeqText(canvas *Canvas, text string, x, y int, cls Cls) {
	drawSeqTextColor(canvas, text, x, y, cls, NodeStyle{})
}

func drawSeqTextColor(canvas *Canvas, text string, x, y int, cls Cls, style NodeStyle) {
	cur := x
	for _, c := range text {
		cw := charWidth(c)
		if cw < 1 {
			cw = 1
		}
		for k := 0; k < cw; k++ {
			if cur+k < canvas.w && y < canvas.h {
				i := canvas.idx(cur+k, y)
				canvas.mask[i] = 0
			}
			ch := c
			if k > 0 {
				ch = CONT
			}
			canvas.setColor(cur+k, y, ch, cls, style)
		}
		cur += cw
	}
}

func noteGeometry(xs []int, anchor *NoteAnchor, textW int) (int, int) {
	pad := PAD*2 + 2
	switch anchor.Type {
	case NoteAnchorOver:
		center := (xs[anchor.A] + xs[anchor.B]) / 2
		w := (xs[anchor.B] - xs[anchor.A] + 5)
		if textW+pad > w {
			w = textW + pad
		}
		x := center - w/2
		if x < 0 {
			x = 0
		}
		return x, w
	case NoteAnchorLeft:
		w := textW + pad
		x := xs[anchor.A] - (2 + w - 1)
		if x < 0 {
			x = 0
		}
		return x, w
	case NoteAnchorRight:
		return xs[anchor.A] + 2, textW + pad
	}
	return 0, 0
}

func layoutSequence(seq *Sequence, styles *MermaidStyles, maxWidth *int) (*MermaidArt, error) {
	n := len(seq.Labels)
	if n == 0 {
		return nil, nil // Or an error
	}

	labels := make([]string, n)
	boxW := make([]int, n)
	for i, l := range seq.Labels {
		labels[i] = fitLabel(l, WRAP_WIDTH)
		w := 0
		for _, c := range labels[i] {
			cw := charWidth(c)
			if cw < 1 {
				cw = 1
			}
			w += cw
		}
		if w < 1 {
			w = 1
		}
		boxW[i] = w + 2*PAD + 2
	}
	boxH := 3

	itemTextW := func(text *string) int {
		if text == nil {
			return 0
		}
		w := 0
		for _, c := range *text {
			cw := charWidth(c)
			if cw < 1 {
				cw = 1
			}
			w += cw
		}
		return w
	}

	gaps := make([]int, n)
	for i := 0; i < n-1; i++ {
		w1 := (boxW[i] + 1) / 2
		w2 := (boxW[i+1] + 1) / 2
		gap := w1 + w2 + 1
		if 5 > gap {
			gap = 5
		} // SEQ_GAP
		gaps[i] = gap
	}

	type reqT struct{ l, r, need int }
	var reqs []reqT
	for _, item := range seq.Items {
		switch item.Type {
		case SeqItemMessage:
			tw := itemTextW(item.Text)
			if item.From != item.To {
				l, r := min(item.From, item.To), max(item.From, item.To)
				need := tw + 2
				if need < 4 {
					need = 4
				}
				reqs = append(reqs, reqT{l, r, need})
			} else if item.From+1 < n {
				reqs = append(reqs, reqT{item.From, item.From + 1, 5 + tw + 2})
			}
		case SeqItemNote:
			tw := itemTextW(item.Text)
			switch item.Anchor.Type {
			case NoteAnchorOver:
				if item.Anchor.A < item.Anchor.B {
					need := tw - 1
					if need < 0 {
						need = 0
					}
					reqs = append(reqs, reqT{item.Anchor.A, item.Anchor.B, need})
				} else {
					i := item.Anchor.A
					half := (tw+4+1)/2 + 2
					if i > 0 {
						reqs = append(reqs, reqT{i - 1, i, half})
					}
					if i+1 < n {
						reqs = append(reqs, reqT{i, i + 1, half})
					}
				}
			case NoteAnchorLeft:
				if item.Anchor.A > 0 {
					reqs = append(reqs, reqT{item.Anchor.A - 1, item.Anchor.A, tw + 7})
				}
			case NoteAnchorRight:
				if item.Anchor.A+1 < n {
					reqs = append(reqs, reqT{item.Anchor.A, item.Anchor.A + 1, tw + 7})
				}
			}
		}
	}
	sort.Slice(reqs, func(i, j int) bool {
		return (reqs[i].r - reqs[i].l) < (reqs[j].r - reqs[j].l)
	})
	for _, req := range reqs {
		cur := 0
		for _, g := range gaps[req.l:req.r] {
			cur += g
		}
		if cur < req.need {
			gaps[req.r-1] += req.need - cur
		}
	}

	xs := make([]int, n)
	xs[0] = boxW[0] / 2
	for i := 1; i < n; i++ {
		xs[i] = xs[i-1] + gaps[i-1]
	}

	canvasW := xs[n-1] + (boxW[n-1]+1)/2 + 1
	for _, item := range seq.Items {
		switch item.Type {
		case SeqItemMessage:
			if item.From == item.To {
				w := xs[item.From] + 5 + itemTextW(item.Text) + 1
				if w > canvasW {
					canvasW = w
				}
			}
		case SeqItemNote:
			x, w := noteGeometry(xs, &item.Anchor, itemTextW(item.Text))
			if x+w+1 > canvasW {
				canvasW = x + w + 1
			}
		case SeqItemDivider:
			tw := itemTextW(item.Text)
			if tw+4 > canvasW {
				canvasW = tw + 4
			}
		}
	}

	rows := make([]int, len(seq.Items))
	y := boxH + 1
	for i, item := range seq.Items {
		rows[i] = y
		dy := 2
		switch item.Type {
		case SeqItemMessage:
			if item.From == item.To {
				dy = 4
			} else if item.Text != nil {
				dy = 3
			}
		case SeqItemNote:
			dy = 4
		case SeqItemDivider:
			dy = 2
		}
		y += dy
	}
	bottomTop := y
	canvasH := bottomTop + boxH

	if maxWidth != nil && canvasW > *maxWidth {
		// handle err OversizeWidth
		return nil, nil
	}
	if canvasW*canvasH > MAX_CANVAS_CELLS {
		return nil, nil // handle err OversizeCells
	}

	canvas := newCanvas(canvasW, canvasH)
	for i := 0; i < n; i++ {
		for _, by := range []int{0, bottomTop} {
			x := xs[i] - boxW[i]/2
			if x < 0 {
				x = 0
			}
			p := Placed{
				x: x, y: by, w: boxW[i], h: boxH, cx: xs[i], cy: by + 1, rank: 0,
			}
			drawBox(canvas, &p, []string{labels[i]}, ShapeRect, NodeStyle{})
		}
	}

	for i, item := range seq.Items {
		r := rows[i]
		if item.Type == SeqItemNote {
			x, w := noteGeometry(xs, &item.Anchor, itemTextW(item.Text))
			p := Placed{
				x: x, y: r, w: w, h: 3, cx: x + w/2, cy: r + 1, rank: 0,
			}
			text := ""
			if item.Text != nil {
				text = *item.Text
			}
			drawBox(canvas, &p, []string{text}, ShapeRect, NodeStyle{})
		}
	}

	for _, x := range xs {
		canvas.junction(x, boxH-1, D)
		canvas.segV(x, boxH, bottomTop-1)
		canvas.junction(x, bottomTop, U)
	}

	for i, item := range seq.Items {
		r := rows[i]
		switch item.Type {
		case SeqItemMessage:
			lineCh := '─'
			if item.Dashed {
				lineCh = '╌'
			}
			if item.From == item.To {
				x := xs[item.From]
				canvas.junction(x, r, R)
				canvas.set(x+1, r, lineCh, ClsEdge)
				canvas.set(x+2, r, lineCh, ClsEdge)
				canvas.set(x+3, r, '╮', ClsEdge)
				canvas.set(x+3, r+1, '│', ClsEdge)
				headCh := '◄'
				if item.Head == SeqHeadCross {
					headCh = '×'
				}
				canvas.set(x+1, r+2, headCh, ClsEdge)
				canvas.set(x+2, r+2, lineCh, ClsEdge)
				canvas.set(x+3, r+2, '╯', ClsEdge)
				if item.Text != nil {
					drawSeqText(canvas, *item.Text, x+5, r+1, ClsText)
				}
			} else {
				x0, x1 := xs[item.From], xs[item.To]
				rightward := x1 > x0
				arrowRow := r
				if item.Text != nil {
					arrowRow = r + 1
				}
				lo, hi := min(x0, x1), max(x0, x1)
				dir := L
				if rightward {
					dir = R
				}
				canvas.junction(x0, arrowRow, uint8(dir))
				for x := lo + 1; x < hi; x++ {
					canvas.set(x, arrowRow, lineCh, ClsEdge)
				}
				headCh := '-'
				if item.Head == SeqHeadCross {
					headCh = '×'
				} else if item.Head == SeqHeadArrow && rightward {
					headCh = '▶'
				} else if item.Head == SeqHeadArrow && !rightward {
					headCh = '◄'
				}
				headX := x1 + 1
				if rightward {
					headX = x1 - 1
				}
				canvas.set(headX, arrowRow, headCh, ClsEdge)
				if item.Text != nil {
					span := hi - lo - 1
					if span < 1 {
						span = 1
					}
					t := fitLabel(*item.Text, span)
					tw := 0
					for _, c := range t {
						cw := charWidth(c)
						if cw < 1 {
							cw = 1
						}
						tw += cw
					}
					diff := span - tw
					if diff < 0 {
						diff = 0
					}
					tx := lo + 1 + diff/2
					drawSeqText(canvas, t, tx, r, ClsText)
				}
			}
		case SeqItemDivider:
			for x := 0; x < canvasW; x++ {
				canvas.set(x, r, '─', ClsEdge)
			}
			tW := canvasW - 4
			if tW < 1 {
				tW = 1
			}
			text := ""
			if item.Text != nil {
				text = *item.Text
			}
			t := fitLabel(text, tW)
			drawSeqText(canvas, " "+t+" ", 2, r, ClsEdgeLabel)
		}
	}

	canvas.finalizeMask()
	styled, plain := canvas.toLines(styles)
	return &MermaidArt{StyledLines: styled, PlainLines: plain}, nil
}

type Placed struct {
	x, y, w, h, cx, cy, rank int
}

type NodeExtraType int

const (
	NodeExtraPlain NodeExtraType = iota
	NodeExtraFrame
	NodeExtraCompartments
)

type NodeExtra struct {
	Type         NodeExtraType
	Sub          *Canvas    // used for Frame
	Compartments [][]string // used for class
}

type NodeSizes struct {
	boxW, boxH, layW, layH, extraH, selfLabelW []int
}

func layoutCanvas(graph *Graph, extras []NodeExtra, maxWidth *int) (*Canvas, error) {
	n := len(graph.Nodes)
	if n == 0 {
		return nil, nil // OversizeCells equivalent
	}

	ranks := computeRanks(graph)
	maxRank := 0
	for _, r := range ranks {
		if r > maxRank {
			maxRank = r
		}
	}

	byRank := make([][]int, maxRank+1)
	for i, r := range ranks {
		byRank[r] = append(byRank[r], i)
	}
	orderRanks(byRank, graph.Edges, ranks)

	wrapped := make([][]string, n)
	for i, node := range graph.Nodes {
		wrapped[i] = wrapLabel(node.Label, WRAP_WIDTH, MAX_LINES)
	}

	boxW := make([]int, n)
	boxH := make([]int, n)
	for i := 0; i < n; i++ {
		switch extras[i].Type {
		case NodeExtraFrame:
			titleW := 0
			for _, c := range fitLabel(graph.Nodes[i].Label, WRAP_WIDTH) {
				cw := charWidth(c)
				if cw < 1 {
					cw = 1
				}
				titleW += cw
			}
			w := extras[i].Sub.w + 2
			if titleW+4 > w {
				w = titleW + 4
			}
			boxW[i] = w
			boxH[i] = extras[i].Sub.h + 2
		case NodeExtraCompartments:
			maxLineW := 1
			for _, sec := range extras[i].Compartments {
				for _, line := range sec {
					lw := 0
					for _, c := range line {
						cw := charWidth(c)
						if cw < 1 {
							cw = 1
						}
						lw += cw
					}
					if lw > maxLineW {
						maxLineW = lw
					}
				}
			}
			boxW[i] = maxLineW + 2*PAD + 2

			h := 0
			filled := 0
			for _, sec := range extras[i].Compartments {
				if len(sec) > 0 {
					filled++
				}
				h += len(sec)
			}
			if filled > 0 {
				h += filled - 1
			}
			boxH[i] = h + 2
		case NodeExtraPlain:
			maxLineW := 1
			for _, line := range wrapped[i] {
				lw := 0
				for _, c := range line {
					cw := charWidth(c)
					if cw < 1 {
						cw = 1
					}
					lw += cw
				}
				if lw > maxLineW {
					maxLineW = lw
				}
			}
			boxW[i] = maxLineW + 2*PAD + 2
			boxH[i] = len(wrapped[i]) + 2
		}
	}

	extraH := make([]int, n)
	selfLabelW := make([]int, n)
	for _, e := range graph.Edges {
		if e.From == e.To {
			extraH[e.From] = 2
			if e.Label != "" {
				lw := 0
				for _, c := range e.Label {
					cw := charWidth(c)
					if cw < 1 {
						cw = 1
					}
					lw += cw
				}
				if lw > MAX_LABEL {
					lw = MAX_LABEL
				}
				if lw > selfLabelW[e.From] {
					selfLabelW[e.From] = lw
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		if extraH[i] > 0 {
			if boxW[i] < 7 {
				boxW[i] = 7
			}
		}
	}

	layW := make([]int, n)
	layH := make([]int, n)
	for i := 0; i < n; i++ {
		layW[i] = boxW[i]
		if selfLabelW[i] > 0 {
			layW[i] += 2 * (selfLabelW[i] + 3)
		}
		layH[i] = boxH[i] + extraH[i]
	}

	sizes := NodeSizes{boxW, boxH, layW, layH, extraH, selfLabelW}
	placed := make([]Placed, n)

	vertical := graph.Dir == DirDown || graph.Dir == DirUp
	var plan RoutePlan
	if vertical {
		plan = placeTD(ranks, maxRank, byRank, &sizes, graph, placed)
	} else {
		plan = placeLR(ranks, maxRank, byRank, &sizes, graph, placed)
	}

	canvasW, canvasH := plan.Canvas[0], plan.Canvas[1]

	if maxWidth != nil && canvasW > *maxWidth {
		return nil, nil // OversizeWidth
	}
	if canvasW*canvasH > MAX_CANVAS_CELLS {
		return nil, nil // OversizeCells
	}

	canvas := newCanvas(canvasW, canvasH)
	for idx := 0; idx < n; idx++ {
		style := NodeStyle{}
		if graph.Nodes[idx].Class != "" {
			if s, ok := graph.ClassDefs[graph.Nodes[idx].Class]; ok {
				style = s
			}
		}

		switch extras[idx].Type {
		case NodeExtraFrame:
			drawFrame(canvas, &placed[idx], graph.Nodes[idx].Label, extras[idx].Sub, style)
		case NodeExtraCompartments:
			drawClassBox(canvas, &placed[idx], extras[idx].Compartments, style)
		case NodeExtraPlain:
			drawBox(canvas, &placed[idx], wrapped[idx], graph.Nodes[idx].Shape, style)
		}
	}

	for i, edge := range graph.Edges {
		switch edge.Line {
		case LineKindSolid:
			canvas.cur_style = STY_SOLID
		case LineKindDotted:
			canvas.cur_style = STY_DOT
		case LineKindThick:
			canvas.cur_style = STY_THICK
		}
		if edge.From == edge.To {
			routeSelf(canvas, &placed[edge.From], &edge)
			continue
		}
		from, to := &placed[edge.From], &placed[edge.To]
		adjacent := to.rank == from.rank+1
		bus := plan.BandEnd[from.rank] + plan.EdgeBus[i]
		lane := plan.LaneBase + plan.EdgeLane[i]

		if vertical && adjacent {
			routeForward(canvas, from, to, &edge, bus)
		} else if vertical && !adjacent {
			routeBack(canvas, from, to, &edge, lane)
		} else if !vertical && adjacent {
			routeForwardLR(canvas, from, to, &edge, bus)
		} else {
			routeBackLR(canvas, from, to, &edge, lane)
		}
	}

	canvas.finalizeMask()
	return canvas, nil
}

func layoutFlowchart(graph *Graph, styles *MermaidStyles, maxWidth *int) (*MermaidArt, error) {
	var canvas *Canvas
	var err error
	if len(graph.Groups) > 0 {
		// Hierarchical layout: render each subgraph as a nested, titled frame so the
		// terminal output mirrors the grouped structure shown in Markdown renderers.
		canvas, err = layoutScope(graph, -1, maxWidth)
	} else {
		extras := make([]NodeExtra, len(graph.Nodes))
		for i := range extras {
			extras[i] = NodeExtra{Type: NodeExtraPlain}
		}
		canvas, err = layoutCanvas(graph, extras, maxWidth)
	}
	if err != nil || canvas == nil {
		return nil, err
	}
	if graph.Dir == DirUp {
		canvas.flipVertical()
	} else if graph.Dir == DirLeft {
		canvas.flipHorizontal()
	}
	styled, plain := canvas.toLines(styles)
	return &MermaidArt{StyledLines: styled, PlainLines: plain}, nil
}

// nodeGroupChainRootFirst returns the chain of enclosing group indices for a node,
// ordered from the outermost (root-most) group to the innermost.
func nodeGroupChainRootFirst(graph *Graph, i int) []int {
	if i < 0 || i >= len(graph.NodeGroup) {
		return nil
	}
	var chain []int
	cur := graph.NodeGroup[i]
	for cur != nil {
		chain = append(chain, *cur)
		cur = graph.Groups[*cur].Parent
	}
	for a, b := 0, len(chain)-1; a < b; a, b = a+1, b-1 {
		chain[a], chain[b] = chain[b], chain[a]
	}
	return chain
}

// lcaScope returns the deepest group index that encloses both nodes, or -1 (root)
// if their only common scope is the top level.
func lcaScope(graph *Graph, u, v int) int {
	cu := nodeGroupChainRootFirst(graph, u)
	cv := nodeGroupChainRootFirst(graph, v)
	lca := -1
	for i := 0; i < len(cu) && i < len(cv); i++ {
		if cu[i] == cv[i] {
			lca = cu[i]
		} else {
			break
		}
	}
	return lca
}

// repVtx maps an original node to the vertex that represents it within `scope`:
// either the child group (frame) of `scope` that contains it, or the node itself
// when the node lives directly in `scope`.
func repVtx(graph *Graph, nodeIdx, scope int, groupVtx, nodeVtx map[int]int) (int, bool) {
	chain := nodeGroupChainRootFirst(graph, nodeIdx)
	if scope == -1 {
		if len(chain) == 0 {
			v, ok := nodeVtx[nodeIdx]
			return v, ok
		}
		v, ok := groupVtx[chain[0]]
		return v, ok
	}
	for k, g := range chain {
		if g == scope {
			if k+1 < len(chain) {
				v, ok := groupVtx[chain[k+1]]
				return v, ok
			}
			v, ok := nodeVtx[nodeIdx]
			return v, ok
		}
	}
	return 0, false
}

// layoutScope recursively lays out a single scope (-1 for the top level, otherwise a
// group index). Child groups are rendered first into sub-canvases and embedded as
// titled frames, so grouping is preserved exactly like Markdown subgraphs.
func layoutScope(graph *Graph, scope int, maxWidth *int) (*Canvas, error) {
	var childGroups []int
	for gi := range graph.Groups {
		p := graph.Groups[gi].Parent
		if scope == -1 {
			if p == nil {
				childGroups = append(childGroups, gi)
			}
		} else if p != nil && *p == scope {
			childGroups = append(childGroups, gi)
		}
	}

	var directNodes []int
	for i := range graph.Nodes {
		inner := -1
		if i < len(graph.NodeGroup) && graph.NodeGroup[i] != nil {
			inner = *graph.NodeGroup[i]
		}
		if inner == scope {
			directNodes = append(directNodes, i)
		}
	}

	groupVtx := make(map[int]int)
	nodeVtx := make(map[int]int)
	sub := &Graph{
		Index:     make(map[string]int),
		Dir:       graph.Dir,
		ClassDefs: graph.ClassDefs,
	}
	var extras []NodeExtra

	for _, gi := range childGroups {
		subCanvas, err := layoutScope(graph, gi, nil)
		if err != nil {
			return nil, err
		}
		if subCanvas == nil {
			continue
		}
		groupVtx[gi] = len(sub.Nodes)
		sub.Nodes = append(sub.Nodes, Node{Label: graph.Groups[gi].Label, Shape: ShapeRect})
		extras = append(extras, NodeExtra{Type: NodeExtraFrame, Sub: subCanvas})
	}

	for _, i := range directNodes {
		nodeVtx[i] = len(sub.Nodes)
		sub.Nodes = append(sub.Nodes, graph.Nodes[i])
		extras = append(extras, NodeExtra{Type: NodeExtraPlain})
	}

	if len(sub.Nodes) == 0 {
		return nil, nil
	}

	for _, e := range graph.Edges {
		if lcaScope(graph, e.From, e.To) != scope {
			continue
		}
		fv, ok1 := repVtx(graph, e.From, scope, groupVtx, nodeVtx)
		tv, ok2 := repVtx(graph, e.To, scope, groupVtx, nodeVtx)
		if !ok1 || !ok2 || fv == tv {
			continue
		}
		ne := e
		ne.From = fv
		ne.To = tv
		sub.Edges = append(sub.Edges, ne)
	}

	return layoutCanvas(sub, extras, maxWidth)
}

func placeTD(ranks []int, maxRank int, byRank [][]int, sizes *NodeSizes, graph *Graph, placed []Placed) RoutePlan {
	centers := assignPositions(byRank, sizes.layW, GAP_X, graph.Edges, ranks)
	edgeBus := make([]int, len(graph.Edges))
	busTracks := make([]int, maxRank+1)

	for r := 0; r < maxRank; r++ {
		spans := busSpansTD(graph, ranks, centers, r, false)
		if len(spans) == 0 {
			continue
		}
		assigned, count := assignTracks(spans)
		for _, a := range assigned {
			edgeBus[a.idx] = a.slot
		}
		busTracks[r] = count
	}

	rankH := make([]int, len(byRank))
	for r, row := range byRank {
		maxH := 3
		for _, i := range row {
			h := sizes.boxH[i] + sizes.extraH[i]
			if h > maxH {
				maxH = h
			}
		}
		rankH[r] = maxH
	}
	rankY := make([]int, maxRank+1)
	for r := 1; r <= maxRank; r++ {
		gap := busTracks[r-1] + 1
		if GAP_Y > gap {
			gap = GAP_Y
		}
		rankY[r] = rankY[r-1] + rankH[r-1] + gap
	}
	canvasH := rankY[maxRank] + rankH[maxRank]
	bandEnd := make([]int, maxRank+1)
	for r := 0; r <= maxRank; r++ {
		bandEnd[r] = rankY[r] + rankH[r]
	}

	diagramW := 1
	for r, row := range byRank {
		for _, idx := range row {
			w, h := sizes.boxW[idx], sizes.boxH[idx]
			cx := centers[idx]
			x := cx - w/2
			if x < 0 {
				x = 0
			}
			y := rankY[r] + (rankH[r]-h-sizes.extraH[idx])/2
			placed[idx] = Placed{x, y, w, h, cx, y + h/2, r}
			if x+w > diagramW {
				diagramW = x + w
			}
			if sizes.extraH[idx] > 0 && sizes.selfLabelW[idx] > 0 {
				if x+w+2+sizes.selfLabelW[idx] > diagramW {
					diagramW = x + w + 2 + sizes.selfLabelW[idx]
				}
			}
		}
	}

	contentW := diagramW
	for _, e := range graph.Edges {
		if e.From == e.To {
			continue
		}
		if e.Label != "" {
			lw := 0
			for _, c := range e.Label {
				cw := charWidth(c)
				if cw < 1 {
					cw = 1
				}
				lw += cw
			}
			if lw > MAX_LABEL {
				lw = MAX_LABEL
			}
			if ranks[e.To] == ranks[e.From]+1 {
				if placed[e.To].cx+2+lw > contentW {
					contentW = placed[e.To].cx + 2 + lw
				}
			} else {
				if diagramW+lw+1 > contentW {
					contentW = diagramW + lw + 1
				}
			}
		}
	}

	edgeLane := make([]int, len(graph.Edges))
	lanes := laneSpans(graph, ranks, placed, true)
	canvasW := contentW
	laneBase := 0
	if len(lanes) > 0 {
		assigned, count := assignTracks(lanes)
		for _, a := range assigned {
			edgeLane[a.idx] = a.slot
		}
		canvasW = contentW + 1 + count
		laneBase = contentW + 1
	}

	return RoutePlan{[2]int{canvasW, canvasH}, bandEnd, edgeBus, laneBase, edgeLane}
}

func placeLR(ranks []int, maxRank int, byRank [][]int, sizes *NodeSizes, graph *Graph, placed []Placed) RoutePlan {
	centers := assignPositions(byRank, sizes.layH, 1, graph.Edges, ranks)
	edgeBus := make([]int, len(graph.Edges))
	busTracks := make([]int, maxRank+1)

	for r := 0; r < maxRank; r++ {
		spans := busSpansTD(graph, ranks, centers, r, true) // essentially same logic
		if len(spans) == 0 {
			continue
		}
		assigned, count := assignTracks(spans)
		for _, a := range assigned {
			edgeBus[a.idx] = a.slot
		}
		busTracks[r] = count
	}

	rankX := make([]int, maxRank+1)
	rankW := make([]int, len(byRank))
	for r, row := range byRank {
		maxW := 0
		for _, i := range row {
			w := sizes.boxW[i]
			if w > maxW {
				maxW = w
			}
		}
		rankW[r] = maxW
	}

	maxLabel := 0
	for _, e := range graph.Edges {
		if e.From == e.To || ranks[e.To] == ranks[e.From]+1 {
			if e.Label != "" {
				lw := 0
				for _, c := range e.Label {
					cw := charWidth(c)
					if cw < 1 {
						cw = 1
					}
					lw += cw
				}
				if lw > MAX_LABEL {
					lw = MAX_LABEL
				}
				if lw > maxLabel {
					maxLabel = lw
				}
			}
		}
	}
	baseGap := GAP_X + 1
	if maxLabel+3 > baseGap {
		baseGap = maxLabel + 3
	}

	for r := 1; r <= maxRank; r++ {
		gap := baseGap + busTracks[r-1]
		rankX[r] = rankX[r-1] + rankW[r-1] + gap
	}
	canvasW := rankX[maxRank] + rankW[maxRank]
	bandEnd := make([]int, maxRank+1)
	for r := 0; r <= maxRank; r++ {
		bandEnd[r] = rankX[r] + rankW[r]
	}

	diagramH := 1
	for r, row := range byRank {
		for _, idx := range row {
			w, h := sizes.boxW[idx], sizes.boxH[idx]
			cy := centers[idx]
			y := cy - h/2
			if y < 0 {
				y = 0
			}
			x := rankX[r] + (rankW[r]-w)/2
			placed[idx] = Placed{x, y, w, h, x + w/2, cy, r}
			if y+h > diagramH {
				diagramH = y + h
			}
			if sizes.extraH[idx] > 0 {
				if y+h+2 > diagramH {
					diagramH = y + h + 2
				}
			}
		}
	}

	contentH := diagramH
	for _, e := range graph.Edges {
		if e.From == e.To {
			continue
		}
		if e.Label != "" {
			if ranks[e.To] != ranks[e.From]+1 {
				if diagramH+2 > contentH {
					contentH = diagramH + 2
				}
			}
		}
	}

	edgeLane := make([]int, len(graph.Edges))
	lanes := laneSpans(graph, ranks, placed, false)
	canvasH := contentH
	laneBase := 0
	if len(lanes) > 0 {
		assigned, count := assignTracks(lanes)
		for _, a := range assigned {
			edgeLane[a.idx] = a.slot
		}
		canvasH = contentH + 1 + count
		laneBase = contentH + 1
	}

	return RoutePlan{[2]int{canvasW, canvasH}, bandEnd, edgeBus, laneBase, edgeLane}
}

func Render(src string, styles *MermaidStyles, maxWidth *int) (*MermaidArt, error) {
	if strings.TrimSpace(src) == "" {
		return nil, nil
	}
	
	if graph := ParseGraph(src); graph != nil {
		return layoutFlowchart(graph, styles, maxWidth)
	}
	
	if state := ParseState(src); state != nil {
		return layoutFlowchart(state, styles, maxWidth)
	}
	
	if seq := ParseSequence(src); seq != nil {
		return layoutSequence(seq, styles, maxWidth)
	}

	if graph, infos := ParseClass(src); graph != nil {
		return RenderClass(graph, infos, styles, maxWidth)
	}

	return nil, nil
}

func RenderClass(graph *Graph, infos []ClassInfo, styles *MermaidStyles, maxWidth *int) (*MermaidArt, error) {
	extras := make([]NodeExtra, len(graph.Nodes))
	for i, node := range graph.Nodes {
		var title []string
		if infos[i].Annotation != nil {
			title = append(title, "«"+*infos[i].Annotation+"»")
		}
		title = append(title, node.Label)
		extras[i] = NodeExtra{
			Type:         NodeExtraCompartments,
			Compartments: [][]string{title, infos[i].Members, nil}, // Simple mock: dump all members into second compartment
		}
		// If we wanted to split attrs/methods accurately, we'd do it here based on parens like Rust did
		var attrs, methods []string
		for _, m := range infos[i].Members {
			if strings.Contains(m, "(") {
				methods = append(methods, m)
			} else {
				attrs = append(attrs, m)
			}
		}
		extras[i].Compartments = [][]string{title, attrs, methods}
	}
	
	canvas, err := layoutCanvas(graph, extras, maxWidth)
	if err != nil || canvas == nil {
		return nil, err
	}
	if graph.Dir == DirUp {
		canvas.flipVertical()
	} else if graph.Dir == DirLeft {
		canvas.flipHorizontal()
	}
	styled, plain := canvas.toLines(styles)
	return &MermaidArt{StyledLines: styled, PlainLines: plain}, nil
}

