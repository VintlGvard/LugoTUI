package engine

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
)

type screenSpy struct {
	tcell.Screen
	events    []string
	showCount int
}

func newScreenSpy(t *testing.T, w, h int) *screenSpy {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(w, h)
	t.Cleanup(sim.Fini)
	return &screenSpy{Screen: sim}
}

func (s *screenSpy) Show() {
	s.showCount++
	s.events = append(s.events, "Show")
	s.Screen.Show()
}

func (s *screenSpy) ShowCursor(x, y int) {
	s.events = append(s.events, fmt.Sprintf("ShowCursor(%d,%d)", x, y))
	s.Screen.ShowCursor(x, y)
}

func (s *screenSpy) HideCursor() {
	s.events = append(s.events, "HideCursor")
	s.Screen.HideCursor()
}

func TestPaintTextSpansByteOffsetsOnWrap(t *testing.T) {
	r, err := Load(`
local ui = require("ui")
return {
	view = ui.text {
		id = "rich",
		spans = {
			{ text = "x   \n", fg = "red" },
			{ text = "  \n", fg = "blue" },
			{ text = "y", fg = "green" },
		},
	},
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(2, 4)

	r.Draw(sim, "")

	_, _, st0, _ := sim.GetContent(0, 0)
	fg0, _, _ := st0.Decompose()
	if fg0 != tcell.GetColor("red") {
		t.Fatalf("line 0 expected red, got %v", fg0)
	}

	_, _, st1, _ := sim.GetContent(0, 1)
	fg1, _, _ := st1.Decompose()
	if fg1 != tcell.GetColor("blue") {
		t.Fatalf("line 1 expected blue, got %v (span shifted into trailing spaces of line 0)", fg1)
	}

	_, _, st2, _ := sim.GetContent(0, 2)
	fg2, _, _ := st2.Decompose()
	if fg2 != tcell.GetColor("green") {
		t.Fatalf("line 2 expected green, got %v", fg2)
	}
}

func TestDrawUpdatesCursorAtomicallyBeforeShow(t *testing.T) {
	r, err := Load(`
local ui = require("ui")
return {
	view = ui.input { id = "inp", text = "hi", cursor_mode = "static" },
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	spyVisible := newScreenSpy(t, 20, 4)
	r.Draw(spyVisible, "")
	if len(spyVisible.events) != 2 || spyVisible.events[0] != "ShowCursor(0,0)" || spyVisible.events[1] != "Show" {
		t.Fatalf("expected single atomic ShowCursor before Show, got %v", spyVisible.events)
	}

	spyDiag := newScreenSpy(t, 20, 1)
	r.Draw(spyDiag, "error banner")
	if len(spyDiag.events) != 2 || spyDiag.events[0] != "HideCursor" || spyDiag.events[1] != "Show" {
		t.Fatalf("expected single atomic HideCursor before Show when diagnostic covers cursor row, got %v", spyDiag.events)
	}
}

func TestDrawSkipsShowWhenFrameAndCursorUnchanged(t *testing.T) {
	r, err := Load(`
local ui = require("ui")
return {
	view = ui.input { id = "inp", text = "ab", cursor_mode = "static" },
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	spy := newScreenSpy(t, 20, 4)
	r.Draw(spy, "")
	if spy.showCount != 1 {
		t.Fatalf("expected 1 Show on initial frame, got %d", spy.showCount)
	}

	r.Draw(spy, "")
	if spy.showCount != 1 {
		t.Fatalf("expected Show skipped on identical frame and cursor, got %d", spy.showCount)
	}

	if err := r.Key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	r.Draw(spy, "")
	if spy.showCount != 2 {
		t.Fatalf("expected Show when cursor position changed, got %d", spy.showCount)
	}

	r.Draw(spy, "")
	if spy.showCount != 2 {
		t.Fatalf("expected Show skipped again after cursor settled, got %d", spy.showCount)
	}
}

func TestScrollLayoutPreservesChildHeightsWithoutDrawMutation(t *testing.T) {
	r, err := Load(`
local ui = require("ui")
return {
	view = ui.scroll {
		id = "scroll",
		gap = 1,
		ui.text { id = "c1", text = "one", height = 4 },
		ui.text { id = "c2", text = "two", height = 4 },
	},
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if err := Layout(r.Root, 30, 5); err != nil {
		t.Fatal(err)
	}
	if r.Nodes["scroll"].Rect.H != 5 {
		t.Fatalf("expected scroll viewport height 5, got %d", r.Nodes["scroll"].Rect.H)
	}
	if r.Nodes["c1"].Rect.H != 4 || r.Nodes["c2"].Rect.H != 4 {
		t.Fatalf("expected scroll children to keep natural height 4 in Layout without shrinking, got c1=%d c2=%d",
			r.Nodes["c1"].Rect.H, r.Nodes["c2"].Rect.H)
	}
	if r.Nodes["c2"].Clip.H != 0 {
		t.Fatalf("expected offscreen second child clipped to 0 in Layout, got %+v", r.Nodes["c2"].Clip)
	}
}
