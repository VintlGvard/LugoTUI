package engine

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/internal/core"
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

func TestLayoutAndRenderBorderInsetsPartialSidesAndWideOverlap(t *testing.T) {
	r, err := Load(`local ui = require('ui')
return {
  view = ui.box {
    id = 'root',
    ui.box {
      id = 'rounded_box',
      position = 'absolute',
      left = 0, top = 0, width = 8, height = 4,
      border = 'rounded',
      padding_left = 2,
      ui.text { id = 'inner', text = '界x' }
    },
    ui.box {
      id = 'bottom_only',
      position = 'absolute',
      left = 0, top = 4, width = 5, height = 2,
      border = true,
      border_top = false, border_left = false, border_right = false, border_bottom = true
    },
    ui.overlay {
      id = 'cover',
      position = 'absolute',
      left = 4, top = 1, width = 3, height = 1, z_index = 5,
      ui.text { id = 'ov_txt', text = 'Z' }
    }
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	sim := newScreenSpy(t, 12, 8)
	r.Draw(sim, "")

	inner := r.Nodes["inner"]
	if inner == nil || inner.Rect.X != 3 || inner.Rect.Y != 1 {
		t.Fatalf("expected inner at (3,1) respecting rounded border + padding_left=2, got %+v", inner.Rect)
	}
	chTopLeft, _, _, _ := sim.GetContent(0, 0)
	if chTopLeft != '╭' {
		t.Fatalf("expected rounded top-left '╭', got %q", chTopLeft)
	}
	chOverlay, _, _, _ := sim.GetContent(4, 1)
	chClearedLeft, _, _, _ := sim.GetContent(3, 1)
	if chOverlay != 'Z' || chClearedLeft != ' ' {
		t.Fatalf("expected partial wide overlap at (4,1) to show 'Z' and clear (3,1) to ' ', got (%q, %q)", chClearedLeft, chOverlay)
	}
	bTop, _, _, _ := sim.GetContent(1, 4)
	bBot, _, _, _ := sim.GetContent(1, 5)
	if bTop != ' ' || bBot != '─' {
		t.Fatalf("expected bottom-only border to draw ' ' at top and '─' at bottom, got (%q, %q)", bTop, bBot)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRuntimeSafetyLuaSetInlineActionsAndTransport(t *testing.T) {
	r, err := Load(`local ui = require('ui')
return {
  view = ui.column {
    id = 'root',
    keymap = { ['Alt+x'] = 'alt_act' },
    ui.input { id = 'inp', bind = 'k' },
    ui.overlay { id = 'dlg', modal = true, hidden = true, close_on_esc = false, ui.button { id = 'b', text = 'ok' } }
  },
  actions = {
    alt_act = function() host.dispatch('alt_fired', {}) end,
    clear = function() ui.set('k', nil) end,
    bad_empty = function() ui.set('', 1) end,
    fail_alt = function() ui.enter_alt_screen(); error('boom') end
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt)); err != nil {
		t.Fatal(err)
	}
	if ev := r.DrainEvents(); len(ev) != 1 || ev[0].Action != "alt_fired" {
		t.Fatalf("expected Alt+x keymap to fire alt_fired, got %+v", ev)
	}
	if err := r.Paste("abc"); err != nil {
		t.Fatal(err)
	}
	if err := r.Action("clear", nil); err != nil {
		t.Fatal(err)
	}
	if _, exists := r.Data["k"]; exists || r.State["inp"].Value != "" {
		t.Fatalf("expected ui.set('k', nil) to delete key and clear bound input, got data=%v val=%q", r.Data["k"], r.State["inp"].Value)
	}
	if err := r.Action("bad_empty", nil); err == nil {
		t.Fatal("expected ui.set('', 1) to fail")
	}
	if err := r.Action("fail_alt", nil); err == nil || !r.AltScreen {
		t.Fatal("expected failed action error")
	}
	if err := r.Patch("dlg", map[string]any{"hidden": false}); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEscape, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if r.Nodes["dlg"].Props["hidden"] != false {
		t.Fatal("modal with close_on_esc=false should not hide on Escape")
	}
	if reply := r.AgentReply([]byte(`{"method":"events"}`)); !bytes.Contains(reply, []byte(`"result":[]`)) {
		t.Fatalf("expected empty events array in JSON, got %s", reply)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	fr := &frameReader{conn: c1}
	go func() {
		_, _ = c2.Write([]byte("{\"a\":1}\n{\"b\":"))
	}()
	buf := make([]byte, 64)
	if _, err := fr.Read(buf); err != nil {
		t.Fatal(err)
	}
	if !fr.inFrame {
		t.Fatal("expected inFrame=true when buffer ends with incomplete second frame")
	}

	clip := NewOSC52Clipboard(failWriter{})
	if err := clip.Write("hi"); err == nil {
		t.Fatal("expected OSC52Clipboard.Write to propagate writer error")
	}

	if _, err := LoadNode(&Node{
		Type: "button",
		ID:   "bad_inline",
		InlineActions: map[string]any{
			"on_click": func(int) {},
		},
	}, nil, nil, false); err == nil {
		t.Fatal("expected unsupported inline action signature to return error")
	}

	r.Close()
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error on Key, got %v", err)
	}
	if err := r.Mouse(tcell.NewEventMouse(0, 0, tcell.Button1, 0)); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error on Mouse, got %v", err)
	}
	if err := r.Paste("x"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error on Paste, got %v", err)
	}

	_ = core.Sides{}
}
