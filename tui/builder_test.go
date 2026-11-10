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

func TestInlineActionClosuresAndBuilderProps(t *testing.T) {
	var selectedIdx int
	var selectedVal any
	var submittedText string

	rt, err := tui.LoadViewFunc(func(r *tui.Runtime) *tui.Node {
		return tui.Overlay(
			tui.Column(
				tui.Textf("Count=%d Open=%v", r.Int("count", 0), r.Bool("open", false)).SetID("status").Wrap(false),
				tui.Input("draft", "Type here").
					SetID("inp").
					Masked(true).
					MaxLength(8).
					OnChangeText(func(val string) {
						_ = r.Set("live", val)
					}).
					OnSubmitText(func(val string) {
						submittedText = val
					}),
				tui.List("first", "second").
					SetID("lst").
					OnSelectItem(func(index int, value any) {
						selectedIdx = index
						selectedVal = value
					}),
				tui.Button("+1").SetID("btn").OnClickDo(func() {
					_ = r.AddInt("count", 1)
				}),
				tui.Help(
					tui.HelpBinding{Keys: []string{"q"}, Description: "QuitApp"},
				).SetID("hlp").Height(1),
			).KeyDo("t", func() {
				r.Toggle("open")
			}),
			tui.If(r.Bool("open"),
				tui.Modal("dlg", "Confirm",
					tui.Text("Modal body").Wrap(true),
				).Absolute(2, 1).Size(20, 6),
			),
		)
	})
	if err != nil {
		t.Fatalf("LoadViewFunc with inline closures failed: %v", err)
	}
	defer rt.Close()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	rt.Draw(sim, "")

	// Check Help description rendered onto screen ("q QuitApp")
	helpFound := false
	for y := 0; y < 24; y++ {
		var row strings.Builder
		for x := 0; x < 80; x++ {
			r, _, _, _ := sim.GetContent(x, y)
			row.WriteRune(r)
		}
		if strings.Contains(row.String(), "q QuitApp") {
			helpFound = true
			break
		}
	}
	if !helpFound {
		t.Fatal("expected Help widget to render 'q QuitApp'")
	}

	// Type into masked input with MaxLength(8)
	for _, ch := range "1234567890" {
		if err := rt.Key(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone)); err != nil {
			t.Fatalf("Key(%q): %v", ch, err)
		}
	}
	if got := rt.String("draft"); got != "12345678" {
		t.Fatalf("expected MaxLength(8) to cap draft at '12345678', got %q", got)
	}
	if got := rt.String("live"); got != "12345678" {
		t.Fatalf("expected OnChangeText to set live='12345678', got %q", got)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatalf("Enter on input: %v", err)
	}
	if submittedText != "12345678" {
		t.Fatalf("expected OnSubmitText '12345678', got %q", submittedText)
	}

	// Select item in list
	if err := rt.SetFocus("lst"); err != nil {
		t.Fatalf("SetFocus(lst): %v", err)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); err != nil {
		t.Fatalf("KeyDown on list: %v", err)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatalf("Enter on list: %v", err)
	}
	if selectedIdx != 2 || selectedVal != "second" {
		t.Fatalf("expected OnSelectItem (2, 'second'), got (%d, %v)", selectedIdx, selectedVal)
	}

	// Click button via Enter
	if err := rt.SetFocus("btn"); err != nil {
		t.Fatalf("SetFocus(btn): %v", err)
	}
	if err := rt.Key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); err != nil {
		t.Fatalf("Enter on btn: %v", err)
	}
	if got := rt.Int("count"); got != 1 {
		t.Fatalf("expected count=1 after OnClickDo, got %d", got)
	}

	// Trigger KeyDo("t") to open modal with Absolute(2, 1)
	if err := rt.Key(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone)); err != nil {
		t.Fatalf("Key 't': %v", err)
	}
	if !rt.Bool("open") || rt.Node("dlg") == nil {
		t.Fatal("expected modal 'dlg' visible after KeyDo('t')")
	}

	// Test dynamic Update, Strings, Map, Float
	if err := rt.Update(map[string]any{
		"ratio": 3.5,
		"tags":  []string{"go", "lua"},
		"meta":  map[string]any{"env": "prod"},
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := rt.Float("ratio"); got != 3.5 {
		t.Fatalf("expected Float('ratio')=3.5, got %v", got)
	}
	if got := rt.Strings("tags"); len(got) != 2 || got[0] != "go" || got[1] != "lua" {
		t.Fatalf("expected Strings('tags')=[go lua], got %v", got)
	}
	if got := rt.Map("meta"); got["env"] != "prod" {
		t.Fatalf("expected Map('meta')[env]=prod, got %v", got)
	}
}

func TestErrQuitExitsProgramCleanly(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim.Init: %v", err)
	}
	sim.SetSize(60, 20)

	ready := make(chan struct{})
	var finalRuntime *tui.Runtime
	p := tui.NewProgram(
		tui.WithScreen(sim),
		tui.WithViewFunc(func(rt *tui.Runtime) *tui.Node {
			return tui.Column(
				tui.Button("Save & Quit").SetID("quit_btn").OnClickErr(func() error {
					_ = rt.Set("saved", true)
					return rt.Quit()
				}),
			)
		}),
		tui.WithOnReady(func(rt *tui.Runtime) {
			finalRuntime = rt
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

	sim.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error on ErrQuit, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ErrQuit program exit")
	}

	if finalRuntime == nil || !finalRuntime.Bool("saved") {
		t.Fatal("expected state mutation before rt.Quit() to be preserved")
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
