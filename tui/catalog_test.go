package tui_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestCatalogWorkbench(t *testing.T) {
	r := mouseFixture(t, tui.Demo())
	act := func(name string, event map[string]any) {
		t.Helper()
		if err := r.Action(name, event); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, page := range []string{"Overview", "Text", "Input", "Layout", "Runtime", "Transport", "Help", "Collections", "Motion", "Time", "Editors"} {
		act("select", map[string]any{"value": page})
		for _, size := range [][2]int{{100, 30}, {60, 20}, {1, 1}} {
			screen.SetSize(size[0], size[1])
			r.Draw(screen, "")
			if page == "Overview" && size == [2]int{100, 30} {
				for _, id := range []string{"overview_text", "overview_input", "overview_layout", "overview_runtime"} {
					if r.Node(id).Clip.W == 0 || r.Node(id).Clip.H == 0 {
						t.Fatal("overview quick navigation clipped", id)
					}
				}
			}
			for id, n := range snapshotNodes(t, r) {
				if n.Rect.W < 0 || n.Rect.H < 0 || n.Clip.W < 0 || n.Clip.H < 0 {
					t.Fatalf("negative geometry: %s at %v", id, size)
				}
			}
		}
	}
	for _, target := range []string{"Text", "Input", "Layout", "Runtime", "Transport"} {
		act("select", map[string]any{"value": "Overview"})
		node := r.Node("overview_" + strings.ToLower(target))
		if node == nil {
			t.Fatal("missing quick navigation", target)
		}
		act(str(node, "on_click", ""), nil)
		if r.Model("demo_page") != target {
			t.Fatal("quick navigation", target)
		}
	}
	act("open_layout", nil)
	for _, choice := range []struct{ action, prop, value string }{
		{"justify_start", "justify", "start"}, {"justify_center", "justify", "center"},
		{"justify_end", "justify", "end"}, {"justify_between", "justify", "space-between"}, {"justify_around", "justify", "space-around"},
		{"direction_column", "direction", "column"}, {"direction_row", "direction", "row"},
		{"align_start", "align", "start"}, {"align_center", "align", "center"}, {"align_stretch", "align", "stretch"}, {"align_end", "align", "end"},
	} {
		act(choice.action, nil)
		if str(r.Node("layout_specimen"), choice.prop, "") != choice.value {
			t.Fatal("specimen patch", choice)
		}
		if r.Model("layout_"+choice.prop) != choice.value {
			t.Fatal("specimen model", choice)
		}
	}
	act("open_text", nil)
	act("open_layout", nil)
	if str(r.Node("layout_specimen"), "align", "") != "end" {
		t.Fatal("layout choice lost")
	}
	act("gap_2", nil)
	if r.Node("layout_specimen").Props["gap"] != float64(2) {
		t.Fatal("gap_2 failed")
	}
	act("layout_reset", nil)
	if str(r.Node("layout_specimen"), "direction", "") != "row" ||
		str(r.Node("layout_specimen"), "justify", "") != "space-between" ||
		str(r.Node("layout_specimen"), "align", "") != "center" ||
		r.Node("layout_specimen").Props["gap"] != float64(1) {
		t.Fatal("layout_reset failed")
	}
	act("open_runtime", nil)
	screen.SetSize(100, 30)
	r.Draw(screen, "")
	setFocus(t, r, "notes")
	original := r.Node("runtime_modal")
	act("runtime_open", nil)
	r.ReconcileFocus()
	if reflect.DeepEqual(r.Node("runtime_modal"), original) || r.Focus() != "runtime_name" {
		t.Fatal("replace+focus", r.Focus())
	}
	act("runtime_accept", map[string]any{"value": "  "})
	if r.Node("runtime_modal").Props["hidden"] != false {
		t.Fatal("blank name accepted")
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEscape, 0, 0)); err != nil {
		t.Fatal(err)
	}
	r.ReconcileFocus()
	if r.Focus() != "notes" {
		t.Fatal("modal cancel focus", r.Focus())
	}
	act("runtime_open", nil)
	r.ReconcileFocus()
	r.Draw(screen, "")
	for _, ch := range "café 界" {
		if err := r.Key(tcell.NewEventKey(tcell.KeyRune, ch, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
		t.Fatal(err)
	}
	r.ReconcileFocus()
	if str(r.Node("runtime_result"), "text", "") != "Experiment: café 界" || r.Focus() != "notes" {
		t.Fatal("accept/focus", r.Focus(), r.Model("runtime_result"))
	}
	act("runtime_open", nil)
	r.ReconcileFocus()
	if r.State("runtime_name").Value != "" {
		t.Fatal("new modal retained old name")
	}
	pasteInto(t, r, "runtime_name", "Second")
	act("runtime_accept_current", nil)
	if r.Model("runtime_result") != "Experiment: Second" {
		t.Fatal("accept current")
	}
	act("palette", nil)
	act("command", map[string]any{"value": "Focus scratchpad"})
	r.ReconcileFocus()
	if r.Focus() != "notes" {
		t.Fatal("scratchpad command", r.Focus())
	}
	act("agent.text", map[string]any{"text": "Hello"})
	if str(r.Node("answer"), "text", "") != "Hello" {
		t.Fatal("agent answer")
	}
	next, err := r.Reload(tui.Demo())
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if str(next.Node("runtime_result"), "text", "") != "Experiment: Second" {
		t.Fatal("reload lost experiment")
	}
}

func TestCatalogNativeControls(t *testing.T) {
	r := mouseFixture(t, tui.Demo())
	act := func(name string, event map[string]any) {
		t.Helper()
		if err := r.Action(name, event); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	key := func(id string, code tcell.Key, ch rune) {
		t.Helper()
		r.Draw(screen, "")
		setFocus(t, r, id)
		if err := r.Key(tcell.NewEventKey(code, ch, 0)); err != nil {
			t.Fatal(err)
		}
	}
	for _, page := range []string{"Collections", "Motion", "Time", "Editors"} {
		act("open_"+strings.ToLower(page), nil)
		if r.Model("demo_page") != page {
			t.Fatal("native page navigation", page)
		}
	}
	act("open_collections", nil)
	key("catalog_table", tcell.KeyEnter, 0)
	if !strings.HasPrefix(str(r.Node("status"), "text", ""), "Collection · ") {
		t.Fatal("table selection callback")
	}
	act("file_navigate", map[string]any{"path": "src"})
	if !strings.Contains(str(r.Node("status"), "text", ""), "src") {
		t.Fatal("file navigation callback")
	}
	act("open_motion", nil)
	if !r.NeedsTick() {
		t.Fatal("spinner has no tick demand")
	}
	act("progress_more", nil)
	if got := r.Node("catalog_progress").Props["percent"].(float64); got < .44 || got > .46 {
		t.Fatal("progress target", got)
	}
	act("progress_reset", nil)
	if r.Node("catalog_progress").Props["percent"] != float64(0) {
		t.Fatal("progress reset")
	}
	key("catalog_numeric", tcell.KeyRight, 0)
	if !strings.HasPrefix(str(r.Node("catalog_page_result"), "text", ""), "Page 2 / 5") {
		t.Fatal("paginator callback")
	}
	if r.Node("catalog_dots").Props["page"] != float64(1) {
		t.Fatal("paginator synchronization")
	}
	act("open_time", nil)
	if r.NeedsTick() {
		t.Fatal("paused clocks request ticks")
	}
	act("timer_toggle", nil)
	act("stopwatch_toggle", nil)
	if !r.NeedsTick() {
		t.Fatal("running clocks have no tick demand")
	}
	now := time.Now()
	if err := r.Tick(now); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(now.Add(6 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(str(r.Node("catalog_timer_result"), "text", ""), "Countdown complete") {
		t.Fatal("timeout callback")
	}
	act("stopwatch_toggle", nil)
	act("timer_reset", nil)
	act("stopwatch_reset", nil)
	if r.NeedsTick() {
		t.Fatal("reset clocks request ticks")
	}
	if r.Node("catalog_timer").Props["reset_key"] != float64(1) || r.Node("catalog_stopwatch").Props["reset_key"] != float64(1) {
		t.Fatal("clock reset controls")
	}
	act("open_editors", nil)
	pasteInto(t, r, "catalog_validated", "INVALID!")
	if !strings.Contains(str(r.Node("catalog_editor_result"), "text", ""), "lowercase") {
		t.Fatal("validation callback")
	}
	act("editor_submit", map[string]any{"value": "build"})
	if str(r.Node("catalog_editor_result"), "text", "") != "Submitted: build" {
		t.Fatal("editor submit callback")
	}
	act("open_help", nil)
	if r.Node("catalog_help") != nil {
		t.Fatal("native editor subtree was not replaced")
	}
}

func TestDemoPagesAndActions(t *testing.T) {
	r := mouseFixture(t, tui.Demo())
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	act := func(name string, event map[string]any) {
		t.Helper()
		if err := r.Action(name, event); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := r.SetFocus("notes"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlA, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Paste("saved 界🙂"); err != nil {
		t.Fatal(err)
	}
	notes := r.State("notes")
	var previous *tui.Node
	kinds := map[string]bool{}
	var walk func(*tui.Node)
	walk = func(n *tui.Node) {
		kinds[n.Type] = true
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, page := range []string{"Overview", "Text", "Input", "Layout", "Runtime", "Transport", "Help", "Collections", "Motion", "Time", "Editors"} {
		act("select", map[string]any{"value": page})
		if reflect.DeepEqual(r.Node("active_page"), previous) {
			t.Fatal("page not replaced", page)
		}
		previous = r.Node("active_page")
		if r.Model("demo_page") != page || r.State("notes").Value != "saved 界🙂" || r.State("notes").Cursor != notes.Cursor {
			t.Fatal("navigation lost model/editor", page)
		}
		walk(rootNode(t, r))
		for _, size := range [][2]int{{100, 30}, {30, 10}, {1, 1}} {
			screen.SetSize(size[0], size[1])
			r.Draw(screen, "")
			for id, n := range snapshotNodes(t, r) {
				if n.Rect.W < 0 || n.Rect.H < 0 || n.Clip.W < 0 || n.Clip.H < 0 {
					t.Fatal("negative rect", id, size)
				}
			}
		}
	}
	for _, kind := range []string{"box", "row", "column", "panel", "text", "input", "textarea", "button", "list", "scroll", "overlay"} {
		if !kinds[kind] {
			t.Fatal("missing primitive", kind)
		}
	}
	screen.SetSize(100, 30)
	act("select", map[string]any{"value": "Input"})
	r.Draw(screen, "")
	setFocus(t, r, "demo_input")
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, '界', 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Model("demo_value").(string), "界") || !strings.HasPrefix(str(r.Node("callback_result"), "text", ""), "on_change:") {
		t.Fatal("binding/change callback")
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(str(r.Node("callback_result"), "text", ""), "on_submit:") {
		t.Fatal("submit callback")
	}
	act("demo_source", nil)
	if got := r.Items(r.Node("demo_list")); len(got) != 3 || got[0] != "Model source updated" {
		t.Fatal("source", got)
	}
	setFocus(t, r, "demo_list")
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(str(r.Node("callback_result"), "text", ""), "on_select:") {
		t.Fatal("select callback")
	}
	act("demo_toggle", nil)
	if r.Node("disabled_button").Props["disabled"] != false || r.Node("hidden_sample").Props["hidden"] != false {
		t.Fatal("toggle")
	}
	setFocus(t, r, "disabled_button")
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if str(r.Node("callback_result"), "text", "") != "on_click: enabled button" {
		t.Fatal("click callback")
	}
	act("select", map[string]any{"value": "Overview"})
	act("select", map[string]any{"value": "Input"})
	if r.Node("disabled_button").Props["disabled"] != false || r.Node("hidden_sample").Props["hidden"] != false {
		t.Fatal("page replace lost toggle state")
	}
	act("demo_toggle", nil)
	act("select", map[string]any{"value": "Runtime"})
	act("demo_count", nil)
	act("demo_replace", nil)
	if r.Node("replacement_text") == nil || r.Model("demo_count") != float64(1) {
		t.Fatal("replace/count")
	}
	setFocus(t, r, "notes")
	beforeRoot, beforeRevision, beforeEvents := rootNode(t, r), r.Revision(), r.DrainEvents()
	if err := r.Action("demo_rollback", nil); err == nil || !strings.Contains(err.Error(), "intentional failure") {
		t.Fatal("expected rollback error", err)
	}
	if !reflect.DeepEqual(rootNode(t, r), beforeRoot) {
		t.Fatalf("root mismatch")
	}
	if r.Revision() != beforeRevision {
		t.Fatalf("revision mismatch: %d != %d", r.Revision(), beforeRevision)
	}
	if r.Focus() != "notes" {
		t.Fatalf("focus mismatch: %s != notes", r.Focus())
	}
	if r.Model("demo_count") != float64(1) {
		t.Fatalf("demo_count mismatch: %v != 1", r.Model("demo_count"))
	}
	if len(r.DrainEvents()) != len(beforeEvents) {
		t.Fatalf("events mismatch: %d != %d", len(r.DrainEvents()), len(beforeEvents))
	}
	act("palette", nil)
	r.ReconcileFocus()
	if r.Focus() != "command_filter" {
		t.Fatal("modal focus", r.Focus())
	}
	act("filter_commands", map[string]any{"value": "Page:"})
	if len(r.Items(r.Node("commands"))) != 11 {
		t.Fatal("command filter")
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEscape, 0, 0)); err != nil {
		t.Fatal(err)
	}
	r.ReconcileFocus()
	if r.Focus() != "notes" {
		t.Fatal("modal restore", r.Focus())
	}
	act("select", map[string]any{"value": "Transport"})
	act("demo_dispatch", nil)
	events := r.DrainEvents()
	if events[len(events)-1].Action != "demo.ping" {
		t.Fatal("host dispatch", events)
	}
	act("demo_agent", nil)
	act("agent.text", map[string]any{"text": "stream"})
	if !strings.HasSuffix(str(r.Node("answer"), "text", ""), "stream") {
		t.Fatal("agent action")
	}
	act("send", map[string]any{"value": "hello 界"})
	events = r.DrainEvents()
	if events[len(events)-1].Action != "message.send" || r.Model("draft") != "" {
		t.Fatal("send")
	}
	pasteInto(t, r, "prompt", "current")
	act("send_current", nil)
	act("notes_changed", nil)
	act("editor", nil)
	act("help", nil)
	if r.Model("demo_page") != "Help" {
		t.Fatal("help")
	}
	next, err := r.Reload(tui.Demo())
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if next.Model("demo_page") != "Help" || next.State("notes").Value != "saved 界🙂" {
		t.Fatal("reload lost page/editor")
	}
}
