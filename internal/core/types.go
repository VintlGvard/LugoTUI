package core

import (
	"fmt"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/text"
	"github.com/gdamore/tcell/v2"
)

type Rect struct {
	X, Y, W, H int
}

func (r Rect) Intersect(other Rect) Rect {
	x, y := max(r.X, other.X), max(r.Y, other.Y)
	return Rect{
		X: x,
		Y: y,
		W: max(0, min(r.X+r.W, other.X+other.W)-x),
		H: max(0, min(r.Y+r.H, other.Y+other.H)-y),
	}
}

type Sides struct {
	Top, Right, Bottom, Left int
}

func (s Sides) Horizontal() int { return s.Left + s.Right }

func (s Sides) Vertical() int { return s.Top + s.Bottom }

func (s Sides) Add(other Sides) Sides {
	return Sides{
		Top:    s.Top + other.Top,
		Right:  s.Right + other.Right,
		Bottom: s.Bottom + other.Bottom,
		Left:   s.Left + other.Left,
	}
}

type BorderSides struct {
	Top, Right, Bottom, Left bool
}

func (b BorderSides) Any() bool { return b.Top || b.Right || b.Bottom || b.Left }

func (b BorderSides) All() bool { return b.Top && b.Right && b.Bottom && b.Left }

func (b BorderSides) Insets() Sides {
	var s Sides
	if b.Top {
		s.Top = 1
	}
	if b.Right {
		s.Right = 1
	}
	if b.Bottom {
		s.Bottom = 1
	}
	if b.Left {
		s.Left = 1
	}
	return s
}

type Canvas interface {
	Width() int
	Height() int
	Bounds() Rect
	SetCell(x, y int, r rune, st ...tcell.Style)
	Print(x, y int, text string, st ...tcell.Style)
	Printf(x, y int, st tcell.Style, format string, args ...any)
	Fill(r Rect, ch rune, st ...tcell.Style)
	LineH(x, y, w int, ch rune, st ...tcell.Style)
	LineV(x, y, h int, ch rune, st ...tcell.Style)
	IsFocused() bool
}

type Node struct {
	Type, ID            string
	Props               map[string]any
	Children            []*Node
	Rect, Clip          Rect
	LayoutText          string
	LayoutBound         bool
	LastPropText        string
	IntrinsicCache      [2]float64
	IntrinsicValid      bool
	MeasuredWidth       int
	MeasuredHeightCache float64
	MeasuredValid       bool
	ScrollExtent        int
	LinesCacheText      string
	LinesCacheWidth     int
	LinesCacheLimit     int
	LinesCacheWrap      bool
	LinesCacheValid     bool
	LinesCache          []text.WrappedLine
	InlineActions       map[string]any
	CustomPaint         func(c Canvas)
	RawKey              func(e *tcell.EventKey) bool
}

type NodeError struct {
	Node     *Node
	Property string
	Err      error
}

func (e *NodeError) Error() string {
	context := "node <nil>"
	if e.Node != nil {
		context = fmt.Sprintf("node %q (type %q)", e.Node.ID, e.Node.Type)
	}
	if e.Property != "" {
		context += fmt.Sprintf(" property %q", e.Property)
	}
	return context + ": " + e.Err.Error()
}

func (e *NodeError) Unwrap() error { return e.Err }

type Operation struct {
	ID      string
	Props   map[string]any
	Tree    *Node
	Replace bool
}

type CollectionState struct {
	Query    string
	Editing  bool
	X        int
	Expanded map[string]bool
}

func CloneCollectionState(s *CollectionState) *CollectionState {
	if s == nil {
		return nil
	}
	c := *s
	c.Expanded = make(map[string]bool, len(s.Expanded))
	for k, v := range s.Expanded {
		c.Expanded[k] = v
	}
	return &c
}

type PrimitiveState struct {
	Initialized                    bool
	Last                           time.Time
	Elapsed                        time.Duration
	Duration, ResetKey             float64
	Running, RunningProp, TimedOut bool
	Frame                          int
	FrameElapsed                   time.Duration
	Percent, From, Target          float64
	ProgressElapsed                time.Duration
	Page                           int
	PageProp                       float64
	Full, FullProp                 bool
	CursorBlinked                  bool
	CursorElapsed                  time.Duration
}

func ClonePrimitiveState(s *PrimitiveState) *PrimitiveState {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

type WidgetState struct {
	Value                            string
	Cursor, Anchor, Scroll, Selected int
	Undo, Redo                       []string
	Collection                       *CollectionState
	Primitive                        *PrimitiveState
	IndexedValue                     string
	IndexedWidth                     int
	IndexedMultiline                 bool
	Indexed                          bool
	Clusters                         []string
	Positions                        [][2]int
}

func EditorPositions(g []string, width int, multiline bool) [][2]int {
	p := make([][2]int, len(g)+1)
	x, y := 0, 0
	for i, s := range g {
		if multiline && s != "\n" && x+text.TextWidth(s) > width && x > 0 {
			x = 0
			y++
		}
		p[i] = [2]int{x, y}
		if s == "\n" && multiline {
			x = 0
			y++
		} else {
			x += text.TextWidth(s)
		}
	}
	p[len(g)] = [2]int{x, y}
	return p
}

func (s *WidgetState) EditorIndex(width int, multiline bool) ([]string, [][2]int) {
	if !s.Indexed || s.IndexedValue != s.Value {
		s.Clusters = text.Graphemes(s.Value)
		s.Positions = nil
		s.IndexedValue = s.Value
	}
	if s.Positions == nil || s.IndexedWidth != width || s.IndexedMultiline != multiline {
		s.Positions = EditorPositions(s.Clusters, width, multiline)
		s.IndexedWidth = width
		s.IndexedMultiline = multiline
	}
	s.Indexed = true
	return s.Clusters, s.Positions
}

type BorderGlyphs struct {
	Top         rune
	Bottom      rune
	Left        rune
	Right       rune
	TopLeft     rune
	TopRight    rune
	BottomLeft  rune
	BottomRight rune
}

var (
	SingleBorder  = BorderGlyphs{Top: '─', Bottom: '─', Left: '│', Right: '│', TopLeft: '┌', TopRight: '┐', BottomLeft: '└', BottomRight: '┘'}
	RoundedBorder = BorderGlyphs{Top: '─', Bottom: '─', Left: '│', Right: '│', TopLeft: '╭', TopRight: '╮', BottomLeft: '╰', BottomRight: '╯'}
	DoubleBorder  = BorderGlyphs{Top: '═', Bottom: '═', Left: '║', Right: '║', TopLeft: '╔', TopRight: '╗', BottomLeft: '╚', BottomRight: '╝'}
	ThickBorder   = BorderGlyphs{Top: '━', Bottom: '━', Left: '┃', Right: '┃', TopLeft: '┏', TopRight: '┓', BottomLeft: '┗', BottomRight: '┛'}
	ASCIIBorder   = BorderGlyphs{Top: '-', Bottom: '-', Left: '|', Right: '|', TopLeft: '+', TopRight: '+', BottomLeft: '+', BottomRight: '+'}
)

type Theme struct {
	Foreground      string
	Background      string
	FocusForeground string
}

type ThemePreset = Theme

func DefaultTheme() Theme {
	return Theme{Foreground: "#d8dee9", Background: "#171c26", FocusForeground: "#88c0d0"}
}

func TokyoNightTheme() Theme {
	return Theme{Foreground: "#c0caf5", Background: "#1a1b26", FocusForeground: "#7aa2f7"}
}

func CatppuccinTheme() Theme {
	return Theme{Foreground: "#cdd6f4", Background: "#1e1e2e", FocusForeground: "#89b4fa"}
}

func DraculaTheme() Theme {
	return Theme{Foreground: "#f8f8f2", Background: "#282a36", FocusForeground: "#bd93f9"}
}

func NordTheme() Theme {
	return Theme{Foreground: "#d8dee9", Background: "#2e3440", FocusForeground: "#88c0d0"}
}
