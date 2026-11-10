package tui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestPureGoRetainedViewAndActions(t *testing.T) {
	view := tui.Column(
		tui.Text("Counter").Bind("label"),
		tui.Input("name").SetID("name_in").Placeholder("Enter name").OnSubmit("submit_name"),
		tui.Row(
			tui.Button("+1", "inc").SetID("inc_btn"),
			tui.Button("Fail", "fail_act").SetID("fail_btn"),
		).Gap(1),
	).Padding(1).Gap(1)

	rt, err := tui.LoadView(
		view,
		tui.WithState("count", 10),
		tui.WithState("label", "Count: 10"),
		tui.WithAction("inc", func(r *tui.Runtime, _ any) error {
			if err := r.AddInt("count", 5); err != nil {
				return err
			}
			return r.Set("label", "Count: 15")
		}),
		tui.WithAction("submit_name", func(r *tui.Runtime, payload any) error {
			m, _ := payload.(map[string]any)
			val, _ := m["value"].(string)
			return r.Set("submitted", val)
		}),
		tui.WithAction("fail_act", func(r *tui.Runtime, _ any) error {
			_ = r.Set("count", 999)
			return errors.New("boom")
		}),
	)
	if err != nil {
		t.Fatalf("LoadView failed: %v", err)
	}
	defer rt.Close()

	if rt.Focus() != "name_in" {
		t.Fatalf("expected initial focus on name_in, got %q", rt.Focus())
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyRune, 'G', tcell.ModNone)); err != nil {
		t.Fatalf("Key 'G' failed: %v", err)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone)); err != nil {
		t.Fatalf("Key 'o' failed: %v", err)
	}
	if got := rt.String("name"); got != "Go" {
		t.Fatalf("expected bound model 'name' = %q, got %q", "Go", got)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatalf("Enter on input failed: %v", err)
	}
	if got := rt.String("submitted"); got != "Go" {
		t.Fatalf("expected submitted = %q, got %q", "Go", got)
	}

	if next := rt.FocusNext(); next != "inc_btn" {
		t.Fatalf("expected FocusNext to reach inc_btn, got %q", next)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatalf("Enter on inc_btn failed: %v", err)
	}
	if got := rt.Int("count"); got != 15 {
		t.Fatalf("expected count = 15, got %d", got)
	}
	if got := rt.String("label"); got != "Count: 15" {
		t.Fatalf("expected label = 'Count: 15', got %q", got)
	}

	// Verify transactional rollback when Go action returns error.
	if err := rt.Action("fail_act", nil); err == nil {
		t.Fatal("expected fail_act to return error")
	}
	if got := rt.Int("count"); got != 15 {
		t.Fatalf("expected count rolled back to 15 after failed action, got %d", got)
	}
}

func TestPureGoReactiveViewFuncPreservesFocusAndState(t *testing.T) {
	items := []string{"Alpha", "Beta"}
	rt, err := tui.LoadViewFunc(
		func(r *tui.Runtime) *tui.Node {
			return tui.Column(
				tui.Textf("Query: %s | Count: %d", r.String("query"), r.Int("count")).SetID("header").Bold(true),
				tui.Input("query").SetID("search"),
				tui.If(r.Bool("show_details"),
					tui.Panel("Details",
						tui.Text("Detailed metrics active").SetID("details_txt").FG("green"),
					).SetID("details_panel"),
				),
				tui.Row(
					tui.Button("Increment", "inc").SetID("btn_inc"),
					tui.Button("Toggle Details", "toggle").SetID("btn_toggle"),
				).Gap(1),
			).Add(
				tui.Map(items, func(i int, item string) *tui.Node {
					return tui.Textf("%d. %s", i+1, item)
				})...,
			).Padding(1)
		},
		tui.WithInitialData(map[string]any{
			"query":        "",
			"count":        1,
			"show_details": false,
		}),
		tui.WithHandlers(map[string]tui.ActionHandler{
			"inc": func(r *tui.Runtime, _ any) error {
				return r.AddInt("count", 1)
			},
			"toggle": func(r *tui.Runtime, _ any) error {
				return r.ToggleBool("show_details")
			},
		}),
	)
	if err != nil {
		t.Fatalf("LoadViewFunc failed: %v", err)
	}
	defer rt.Close()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	rt.Draw(sim, "")

	// Type into search input; reactive ViewFunc should automatically update header text
	// while keeping focus and cursor inside "search".
	for _, ch := range "hello" {
		if err := rt.Key(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone)); err != nil {
			t.Fatalf("Key(%q): %v", ch, err)
		}
	}
	if rt.Focus() != "search" {
		t.Fatalf("expected focus preserved on search, got %q", rt.Focus())
	}
	if got := rt.State("search").Cursor; got != 5 {
		t.Fatalf("expected cursor at 5, got %d", got)
	}
	header := rt.Node("header")
	if header == nil || header.Props["text"] != "Query: hello | Count: 1" {
		t.Fatalf("expected reactive header updated, got %#v", header)
	}
	if rt.Node("details_panel") != nil {
		t.Fatal("expected details_panel hidden initially")
	}

	// Trigger toggle action; details_panel should appear in the reconciled tree.
	if err := rt.Action("toggle", nil); err != nil {
		t.Fatalf("Action(toggle): %v", err)
	}
	if rt.Node("details_panel") == nil || rt.Node("details_txt") == nil {
		t.Fatal("expected details_panel and details_txt after toggle")
	}
}

func TestAllBuilderWidgetsValidateAndRender(t *testing.T) {
	root := tui.Column(
		tui.Panel("Dashboard",
			tui.Row(
				tui.Spinner("dot"),
				tui.Text("Building...").FG("cyan").Bold(true),
				tui.Spacer(),
				tui.Progress(0.65).Width(20),
			).Gap(1).Height(1),
			tui.Table(
				[]tui.TableColumn{
					{Title: "Service", Width: 12, Align: "left"},
					{Title: "Status", Width: 10, Align: "center"},
				},
				[][]string{
					{"api", "UP"},
					{"worker", "UP"},
				},
			).Height(4),
			tui.Tree(
				tui.TreeItem{
					ID:       "src",
					Text:     "src",
					Expanded: true,
					Children: []tui.TreeItem{
						{ID: "main_go", Text: "main.go"},
					},
				},
			).Height(3),
			tui.List("Item A", "Item B").Filterable(true).Height(3),
			tui.Viewport("line 1\nline 2", tui.HighlightSpan{Start: 1, End: 4}).ShowLineNumbers(true).Height(2),
			tui.Filepicker(
				tui.FileEntry{Text: "README.md", Path: "/README.md", Size: 128},
			).Height(2),
			tui.Row(
				tui.Paginator(0, 10, 35),
				tui.Timer(5000),
				tui.Stopwatch(),
				tui.Cursor(),
			).Gap(2).Height(1),
			tui.Help(
				tui.HelpBinding{Keys: []string{"q", "ctrl+c"}, Description: "Quit"},
			).Height(1),
		).Grow(1),
	)

	rt, err := tui.LoadView(root)
	if err != nil {
		t.Fatalf("LoadView for all widgets failed: %v", err)
	}
	defer rt.Close()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	defer sim.Fini()
	sim.SetSize(100, 30)
	rt.Draw(sim, "")

	snap, err := rt.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !strings.Contains(string(snap), `"root.panel_0.table_1"`) {
		t.Fatalf("expected Snapshot to contain root.panel_0.table_1, got %s", string(snap))
	}
}

func TestHybridLuaWithoutBoilerplateActions(t *testing.T) {
	// Notice: no `actions = { ... }` table in Lua at all!
	// `on_click = "ping"` dispatches directly to Go WithHandlers.
	const luaTemplate = `
return {
  view = {
    id = "root",
    type = "column",
    children = {
      { id = "lbl", type = "text", bind = "msg" },
      { id = "btn", type = "button", text = "Ping", on_click = "ping" },
    },
  },
}
`
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	sim.SetSize(60, 20)

	ready := make(chan struct{})
	p := tui.NewProgram(
		tui.WithSource(luaTemplate),
		tui.WithScreen(sim),
		tui.WithState("msg", "waiting"),
		tui.WithAction("ping", func(rt *tui.Runtime, _ any) error {
			return rt.Set("msg", "pong-from-go")
		}),
		tui.WithOnReady(func(rt *tui.Runtime) {
			close(ready)
		}),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for program ready")
	}

	if err := p.Send("ping", nil); err != nil {
		t.Fatalf("p.Send(ping): %v", err)
	}
	p.Quit()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("p.Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for program exit")
	}
}
