package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestNativeWidgetIntegration(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)

	r := mouseFixture(t, tui.Demo())

	if err := r.Action("open_motion", nil); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")
	if r.Node("catalog_cursor") == nil {
		t.Fatal("missing catalog_cursor")
	}
	if str(r.Node("catalog_cursor"), "mode", "") != "blink" {
		t.Fatal("cursor mode mismatch")
	}

	prog := r.Node("catalog_progress")
	if prog == nil || prog.Props["percent_format"] == nil {
		t.Fatal("progress component missing percent_format")
	}

	if err := r.Action("open_runtime", nil); err != nil {
		t.Fatal(err)
	}
	presets := []string{"line", "dot", "mini_dot", "jump", "pulse", "points", "globe", "moon", "monkey", "meter", "hamburger", "ellipsis"}
	targetID := "replace_slot"
	for _, p := range presets {
		node := &tui.Node{
			Type:  "spinner",
			ID:    "test_sp_" + p,
			Props: map[string]any{"preset": p},
		}
		if err := r.Replace(targetID, node); err != nil {
			t.Fatalf("preset %s failed: %v", p, err)
		}
		targetID = node.ID
	}

	if err := r.Action("open_motion", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFocus("catalog_numeric"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'l', 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(str(r.Node("catalog_page_result"), "text", ""), "Page 2 / 5") {
		t.Fatal("paginator vim 'l' navigation failed")
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'h', 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(str(r.Node("catalog_page_result"), "text", ""), "Page 1 / 5") {
		t.Fatal("paginator vim 'h' navigation failed")
	}

	if err := r.Action("open_time", nil); err != nil {
		t.Fatal(err)
	}
	if r.Node("catalog_timer") == nil || r.Node("catalog_stopwatch") == nil {
		t.Fatal("missing timer or stopwatch")
	}

	if err := r.Action("open_editors", nil); err != nil {
		t.Fatal(err)
	}
	if r.Node("catalog_help") == nil {
		t.Fatal("missing help node")
	}

	if err := r.Action("open_collections", nil); err != nil {
		t.Fatal(err)
	}
	files := r.Node("catalog_files")
	if files == nil || !files.Props["show_size"].(bool) || !files.Props["show_permissions"].(bool) {
		t.Fatal("filepicker missing show_size or show_permissions")
	}
	if err := r.SetFocus("catalog_table"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'j', 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFocus("catalog_tree"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'j', 0)); err != nil {
		t.Fatal(err)
	}

	if err := r.Action("open_text", nil); err != nil {
		t.Fatal(err)
	}
	vp := r.Node("catalog_viewport")
	if vp == nil || !vp.Props["show_line_numbers"].(bool) {
		t.Fatal("viewport missing show_line_numbers")
	}
	if err := r.SetFocus("catalog_viewport"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'j', 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'k', 0)); err != nil {
		t.Fatal(err)
	}

	if err := r.Action("open_editors", nil); err != nil {
		t.Fatal(err)
	}
	input := r.Node("catalog_suggest")
	if input == nil || str(input, "cursor_mode", "") != "blink" {
		t.Fatal("input missing cursor_mode")
	}
	ta := r.Node("catalog_editor_textarea")
	if ta == nil || !ta.Props["line_numbers"].(bool) || str(ta, "cursor_mode", "") != "blink" {
		t.Fatal("textarea missing line_numbers or cursor_mode")
	}
}

func TestClocksInteractiveControls(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)

	r, err := tui.Load(tui.Demo())
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Action("select", map[string]any{"value": "Time"}); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")

	swBtn := r.Node("stopwatch_toggle")
	if swBtn == nil {
		t.Fatal("stopwatch_toggle not found")
	}
	evDown := tcell.NewEventMouse(swBtn.Rect.X+1, swBtn.Rect.Y, tcell.Button1, 0)
	if err := r.Mouse(evDown); err != nil {
		t.Fatal(err)
	}
	evUp := tcell.NewEventMouse(swBtn.Rect.X+1, swBtn.Rect.Y, tcell.ButtonNone, 0)
	if err := r.Mouse(evUp); err != nil {
		t.Fatal(err)
	}
	if !r.NeedsTick() {
		t.Fatal("stopwatch_toggle click failed to start stopwatch")
	}

	now := time.Now()
	if err := r.Tick(now); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(now.Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")
	text := screenContent(screen)
	if !strings.Contains(text, "00:00:00.5") {
		t.Fatalf("expected subsecond 00:00:00.5, got screen:\n%s", text)
	}

	tmBtn := r.Node("timer_toggle")
	if tmBtn == nil {
		t.Fatal("timer_toggle not found")
	}
	if err := r.Mouse(tcell.NewEventMouse(tmBtn.Rect.X+1, tmBtn.Rect.Y, tcell.Button1, 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Mouse(tcell.NewEventMouse(tmBtn.Rect.X+1, tmBtn.Rect.Y, tcell.ButtonNone, 0)); err != nil {
		t.Fatal(err)
	}

	if err := r.Tick(now.Add(1000 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(now.Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")
	text = screenContent(screen)
	if !strings.Contains(text, "Countdown complete") {
		t.Fatalf("expected countdown complete, got:\n%s", text)
	}

	if err := r.Action("timer_toggle", nil); err != nil {
		t.Fatal(err)
	}
	if !r.NeedsTick() {
		t.Fatal("restarting timed-out timer failed")
	}

	if err := r.SetFocus("catalog_timer"); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	if !r.NeedsTick() {
		t.Fatal("enter key failed to resume clock")
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
}
