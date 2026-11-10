package tui_test

import (
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestCustomCanvasWidgetDrawing(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 10)

	var drawnW, drawnH int
	var focusedInPaint bool

	view := func(st *tui.Runtime) *tui.Node {
		return tui.Column(
			tui.Custom(func(c tui.Canvas) {
				drawnW = c.Width()
				drawnH = c.Height()
				focusedInPaint = c.IsFocused()
				c.Fill(tui.Rect{X: 0, Y: 0, W: c.Width(), H: c.Height()}, '.')
				c.LineH(1, 0, 10, '=')
				c.LineV(0, 0, 5, '|')
				c.Print(2, 2, "CUSTOM")
			}).SetID("canvas_node").Focusable(true).Grow(1),
		)
	}

	rt, err := tui.LoadViewFunc(view)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	rt.Draw(screen, "")
	content := screenContent(screen)

	if drawnW != 30 || drawnH != 10 {
		t.Fatalf("expected canvas size 30x10, got %dx%d", drawnW, drawnH)
	}
	if !strings.Contains(content, "CUSTOM") {
		t.Fatalf("expected canvas content to contain 'CUSTOM', got:\n%s", content)
	}
	if !strings.Contains(content, "|==========") {
		t.Fatalf("expected horizontal line, got:\n%s", content)
	}
	if !focusedInPaint {
		t.Fatalf("expected canvas node with Focusable(true) to be focused")
	}
}

func TestBorderStylesAndColors(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 6)

	// 1. Rounded border
	pRounded := tui.Panel("Round", tui.Text("Hi")).
		Border("rounded").
		BorderFG("cyan").
		TitleFG("yellow")

	rt1, err := tui.LoadView(pRounded)
	if err != nil {
		t.Fatal(err)
	}
	defer rt1.Close()
	rt1.Draw(screen, "")
	content1 := screenContent(screen)
	if !strings.Contains(content1, "╭") || !strings.Contains(content1, "╰") {
		t.Fatalf("expected rounded corners '╭' and '╰', got:\n%s", content1)
	}

	// 2. Double border
	pDouble := tui.Panel("Double", tui.Text("Hi")).Border("double")
	rt2, err := tui.LoadView(pDouble)
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close()
	rt2.Draw(screen, "")
	content2 := screenContent(screen)
	if !strings.Contains(content2, "╔") || !strings.Contains(content2, "╚") {
		t.Fatalf("expected double corners '╔' and '╚', got:\n%s", content2)
	}

	// 3. Thick border
	pThick := tui.Panel("Thick", tui.Text("Hi")).Border("thick")
	rt3, err := tui.LoadView(pThick)
	if err != nil {
		t.Fatal(err)
	}
	defer rt3.Close()
	rt3.Draw(screen, "")
	content3 := screenContent(screen)
	if !strings.Contains(content3, "┏") || !strings.Contains(content3, "┗") {
		t.Fatalf("expected thick corners '┏' and '┗', got:\n%s", content3)
	}

	// 4. ASCII border
	pASCII := tui.Panel("ASCII", tui.Text("Hi")).Border("ascii")
	rt4, err := tui.LoadView(pASCII)
	if err != nil {
		t.Fatal(err)
	}
	defer rt4.Close()
	rt4.Draw(screen, "")
	content4 := screenContent(screen)
	if !strings.Contains(content4, "+-") {
		t.Fatalf("expected ascii border '+-', got:\n%s", content4)
	}

	// 5. Custom glyphs
	customGlyphs := tui.BorderGlyphs{
		Top: '*', Bottom: '*', Left: '!', Right: '!',
		TopLeft: '1', TopRight: '2', BottomLeft: '3', BottomRight: '4',
	}
	pCustom := tui.Box(tui.Text("Hi")).BorderGlyphs(customGlyphs)
	rt5, err := tui.LoadView(pCustom)
	if err != nil {
		t.Fatal(err)
	}
	defer rt5.Close()
	rt5.Draw(screen, "")
	content5 := screenContent(screen)
	if !strings.Contains(content5, "1*") || !strings.Contains(content5, "3*") {
		t.Fatalf("expected custom glyphs '1*' and '3*', got:\n%s", content5)
	}
}

func TestCustomButtonStyles(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 8)

	view := tui.Column(
		tui.Button("Save").Plain().SetID("b1"),
		tui.Button("Run").Chevrons().SetID("b2"),
		tui.Button("Cancel").Parens().SetID("b3"),
		tui.Button("Deploy").Prefix(">> ").Suffix(" <<").SetID("b4"),
	)

	rt, err := tui.LoadView(view)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	rt.Draw(screen, "")
	content := screenContent(screen)

	if !strings.Contains(content, "Save") || strings.Contains(content, "[ Save ]") {
		t.Fatalf("expected plain button without brackets, got:\n%s", content)
	}
	if !strings.Contains(content, "< Run >") {
		t.Fatalf("expected chevron button '< Run >', got:\n%s", content)
	}
	if !strings.Contains(content, "( Cancel )") {
		t.Fatalf("expected parens button '( Cancel )', got:\n%s", content)
	}
	if !strings.Contains(content, ">> Deploy <<") {
		t.Fatalf("expected custom prefix/suffix '>> Deploy <<', got:\n%s", content)
	}
}

func TestCustomCollectionCursorPrefix(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 6)

	list := tui.List("Alpha", "Beta", "Gamma").
		SetID("list1").
		CursorPrefix("❯ ").
		SelectedFG("yellow").
		SelectedBG("blue").
		SelectedBold(true)

	rt, err := tui.LoadView(list)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	rt.Draw(screen, "")
	content := screenContent(screen)

	if !strings.Contains(content, "❯ Alpha") {
		t.Fatalf("expected cursor prefix '❯ Alpha', got:\n%s", content)
	}
}

func TestOnRawKeyInterception(t *testing.T) {
	rawHandled := false
	customWidget := tui.Custom(func(c tui.Canvas) {
		c.Print(0, 0, "Focusable custom widget")
	}).SetID("custom_editor").Focusable(true).OnRawKey(func(e *tcell.EventKey) bool {
		if e.Key() == tcell.KeyRune && e.Rune() == 'x' {
			rawHandled = true
			return true // Handled!
		}
		return false
	})

	rt, err := tui.LoadView(customWidget)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	screen := tcell.NewSimulationScreen("UTF-8")
	_ = screen.Init()
	defer screen.Fini()
	screen.SetSize(40, 5)
	rt.Draw(screen, "")

	if rt.Focus() != "custom_editor" {
		t.Fatalf("expected focus on custom_editor, got %q", rt.Focus())
	}

	// Send key 'x'
	err = rt.Key(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	if err != nil {
		t.Fatal(err)
	}
	if !rawHandled {
		t.Fatalf("expected OnRawKey to intercept and handle rune 'x'")
	}
}
