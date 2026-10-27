package tui_test

import (
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func BenchmarkLargeTextDraw(b *testing.B) {
	r, e := tui.Load(`return {view={type='text',id='text',text='large'}}`)
	if e != nil {
		b.Fatal(e)
	}
	defer r.Close()
	if e = r.Patch("text", map[string]any{"text": strings.Repeat("hello world ", 10000)}); e != nil {
		b.Fatal(e)
	}
	s := tcell.NewSimulationScreen("UTF-8")
	if e = s.Init(); e != nil {
		b.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Draw(s, "")
	}
}

func TestBoundTextLayout(t *testing.T) {
	r, err := tui.Load(`ui.set('body', 'one\ntwo\nthree'); return {view=ui.column{ui.text{id='body',bind='body',shrink=0},ui.text{id='after',text='end'}},actions={change=function(e)ui.set('body',e.text)end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(10, 20)
	for _, tc := range []struct {
		text   string
		height int
	}{{"one\ntwo\nthree", 3}, {"short", 1}, {"hello world again", 3}, {"", 1}} {
		if err := r.Action("change", map[string]any{"text": tc.text}); err != nil {
			t.Fatal(err)
		}
		r.Draw(s, "")
		if r.Node("body").Rect.H != tc.height || r.Node("after").Rect.Y != tc.height {
			t.Fatalf("%q: body=%+v after=%+v", tc.text, r.Node("body").Rect, r.Node("after").Rect)
		}
		if str(r.Node("body"), "text", "missing") != "missing" {
			t.Fatal("layout changed source properties")
		}
	}
}

func TestThemeDefaultsOverridesAndReload(t *testing.T) {
	const source = `return {view=ui.column{ui.input{id='input',text='x',height=1},ui.text{id='text',text='y',height=1},ui.text{id='rich',height=1,spans={{text='z',fg='yellow',bg='blue'}}}}}`
	r, err := tui.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(20, 5)
	check := func(y int, fg, bg string) {
		t.Helper()
		_, _, style, _ := s.GetContent(0, y)
		f, b, _ := style.Decompose()
		if f != tcell.GetColor(fg) || b != tcell.GetColor(bg) {
			t.Fatalf("row %d fg=%v bg=%v expected %s/%s", y, f, b, fg, bg)
		}
	}
	r.Draw(s, "")
	check(0, "#88c0d0", "#171c26")
	check(1, "#d8dee9", "#171c26")
	r.SetTheme(tui.Theme{Foreground: "green", Background: "black", FocusForeground: "red"})
	r.Draw(s, "")
	check(0, "red", "black")
	check(1, "green", "black")
	check(2, "yellow", "blue")
	if err := r.Patch("input", map[string]any{"fg": "white", "bg": "purple"}); err != nil {
		t.Fatal(err)
	}
	r.Draw(s, "")
	check(0, "white", "purple")
	next, err := r.Reload(source)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if next.Theme() != r.Theme() {
		t.Fatal("reload lost theme")
	}
	next.Draw(s, "")
	check(0, "red", "black")
	next.SetTheme(tui.Theme{Background: "navy"})
	next.Draw(s, "")
	check(0, tui.DefaultTheme().FocusForeground, "navy")
	check(1, tui.DefaultTheme().Foreground, "navy")
}
