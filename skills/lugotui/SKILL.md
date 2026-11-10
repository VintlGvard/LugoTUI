---
name: lugotui
description: Build, inspect, test, and debug Pure-Go and Go+Lua TUI applications with LugoTUI. Covers Pure-Go reactive & retained View Builder DSL, Lua Live Hot-Reload templates, Flexbox widgets, two-way state binding, 2D spatial & directional navigation, middleware/hooks, Go background tasks, and JSON Lines IPC socket control.
---

# LugoTUI Developer & Agent Skill

LugoTUI is a high-performance Terminal UI framework for Go (`github.com/VintlGvard/LugoTUI` / `github.com/VintlGvard/LugoTUI/tui`) that works **100% in Pure Go** (exceeding Bubble Tea + Lip Gloss + Bubbles out of the box) AND supports **Lua Live Hot-Reload (150ms)** as its killer feature.

## When to Activate

- Building TUI applications in **Pure Go** (`QuickViewFunc`, `QuickView`, `NewProgram`, fluent View Builder DSL)
- Building **Hybrid Go + Lua** TUI applications (`app.lua` + `WithAppFile` / `WithHandlers`) with 150ms live hot-reload
- Implementing keyboard navigation (2D spatial, directional arrows, Vim keys, widget-scoped keymaps)
- Using middleware (`WithMiddleware`), input filters (`WithInputFilter`), state observers (`WithStateObserver`), OSC52 clipboard (`WithClipboard`), or custom cell-diff renderers (`WithRenderer`)
- Running background tasks (`StartTask`, `StartTaskNamed`, `StartTaskDebounced`) and external control (`Program.Send`, `Program.Set`, `Program.Patch`, `Program.Quit`)
- Automating, testing, or inspecting running TUIs via headless `LoadView` / `LoadViewFunc` / `Load` or Unix domain socket IPC

---

## 1. Execution & Architecture Modes

| Mode | Entry Point | Best For |
| --- | --- | --- |
| **1. Pure-Go Reactive (Bubble Tea Killer)** | `lugotui.QuickViewFunc(fn, handlers, opts...)` / `WithViewFunc(fn)` | Pure-Go apps with reactive `func(rt *Runtime) *Node` view functions. Zero Lua VM overhead, automatic Flexbox layout, and automatic preservation of focus, cursor, and scroll state across re-renders. |
| **2. Pure-Go Retained** | `lugotui.QuickView(root, handlers, opts...)` / `WithView(root)` | Pure-Go apps that build the widget tree once and mutate via `rt.Set(k, v)` (two-way `Bind`) or surgical `rt.Patch` / `rt.Replace`. |
| **3. Hybrid Go + Lua (Killer Feature)** | `lugotui.RunApp("app.lua", opts...)` / `QuickRun(src, handlers)` | Instant **150ms Live Hot-Reload** on `.lua` edits, user-customizable plugins/themes, and optional Linux `bubblewrap` sandboxing (`--sandbox`). Widget events (`on_click = "save"`) dispatch directly to Go `WithAction` / `WithHandlers` without Lua boilerplate! |
| **4. Standalone CLI / IPC** | `lugotui --app app.lua [--socket /tmp/app.sock]` | Rapid prototyping, standalone Lua tools, and external AI agent control over JSON Lines Unix socket. |

Both `github.com/VintlGvard/LugoTUI` (root package `lugotui`) and `github.com/VintlGvard/LugoTUI/tui` export the exact same API.

---

## 2. Mode 1: Pure-Go Reactive UI (`WithViewFunc` / `QuickViewFunc`)

Unlike Bubble Tea—where you manually route `tea.Msg` through every child input/table, calculate `lipgloss.Width` math on `tea.WindowSizeMsg`, and concatenate ANSI strings—LugoTUI gives you a declarative Go View Builder with automatic Flexbox layout, 2D focus management, and cell-diff rendering:

```go
package main

import (
	"log"

	lugo "github.com/VintlGvard/LugoTUI"
)

func main() {
	err := lugo.QuickViewFunc(
		func(rt *lugo.Runtime) *lugo.Node {
			return lugo.Column(
				lugo.Panel("Counter Dashboard",
					lugo.Textf("Query: %s | Count: %d", rt.String("query"), rt.Int("count")).Bold(true).FG("cyan"),
					lugo.Input("query").SetID("search").Placeholder("Type to filter..."),
					lugo.If(rt.Bool("details"),
						lugo.Text("Extra diagnostics enabled").FG("green"),
					),
					lugo.Row(
						lugo.Button("+1", "inc").SetID("btn_inc"),
						lugo.Button("Toggle Details", "toggle").SetID("btn_toggle"),
					).Gap(1),
				).Grow(1).Padding(1).Gap(1),
			).Padding(1)
		},
		map[string]lugo.ActionHandler{
			"inc": func(rt *lugo.Runtime, _ any) error {
				return rt.AddInt("count", 1)
			},
			"toggle": func(rt *lugo.Runtime, _ any) error {
				return rt.ToggleBool("details")
			},
		},
		lugo.WithState("count", 0),
		lugo.WithState("details", false),
	)
	if err != nil {
		log.Fatal(err)
	}
}
```

### Why This Beats Bubble Tea
- **Zero Boilerplate**: No `Init()`, `Update(tea.Msg)`, `View() string` ceremony or manual `tea.WindowSizeMsg` math.
- **Preserved Widget State**: Typing into `lugo.Input("query").SetID("search")` automatically updates `rt.String("query")`, re-evaluates `ViewFunc`, and reconciles the new `*Node` tree while keeping keyboard focus, cursor position, and undo history intact.
- **Conditional & List Helpers**:
  - `lugo.If(cond, node)` / `node.When(cond)` — returns `nil` when false (containers automatically ignore `nil` children).
  - `lugo.IfElse(cond, thenNode, elseNode)`
  - `lugo.Map(slice, func(i int, item T) *lugo.Node { ... })`

---

## 3. Mode 2: Pure-Go Retained UI (`WithView` / `QuickView`)

For maximum performance (12 heap allocations per frame on a 300-node tree), build the view once and let two-way `Bind(key)` and `rt.Set(key, val)` update widgets in place:

```go
view := lugo.Column(
	lugo.Text("").Bind("status").Height(1),
	lugo.Input("name").Placeholder("Enter your name...").OnSubmit("save").Height(1),
	lugo.Button("Save", "save").Height(1),
).Padding(1).Gap(1)

err := lugo.QuickView(view, map[string]lugo.ActionHandler{
	"save": func(rt *lugo.Runtime, _ any) error {
		return rt.Set("status", "Saved: "+rt.String("name"))
	},
}, lugo.WithState("status", "Ready"))
```

Note: Explicit `.SetID("...")` is optional in Pure-Go views—`AutoAssignIDs` deterministically assigns stable IDs (`root`, `root.input_name`, `root.button_save`, etc.) when omitted. Provide explicit `.SetID("...")` when targeting a node with `rt.Patch(id, ...)` or when reordering dynamic children in `WithViewFunc`.

---

## 4. Mode 3: Hybrid Go + Lua Live Hot-Reload (`WithAppFile` / `WithSource`)

Move layout into `app.lua` whenever you want **150ms live hot-reload** without recompiling Go, or user-scriptable themes/plugins.

### Direct Go Dispatch (Zero Lua Action Boilerplate)
Widget events (`on_click`, `on_change`, `on_submit`, `on_select`, `keymap`) in `app.lua` automatically invoke Go handlers registered via `WithAction` / `WithHandlers`—you do **not** need to write pass-through Lua `actions = { ... }` functions unless you want local Lua logic!

```lua
-- app.lua
local ui = require('ui')

return {
    api_version = 1,
    spatial_nav = true,
    view = function()
        return ui.column {
            padding = 1, gap = 1,
            ui.panel {
                title = "Dashboard",
                border = true,
                ui.text { bind = "status_msg", height = 1 },
                ui.row {
                    gap = 1, height = 1,
                    ui.input { id = "search_input", bind = "query", placeholder = "Search..." },
                    ui.button { id = "btn_submit", text = "Submit", on_click = "submit_query" },
                },
            },
        }
    end,
}
```

```go
// main.go — same Go handlers work with Pure-Go view OR app.lua!
p := lugo.NewProgram(
	lugo.WithAppFile("app.lua"),
	lugo.WithState("status_msg", "Ready"),
	lugo.WithAction("submit_query", func(rt *lugo.Runtime, _ any) error {
		return rt.Set("status_msg", "Searching: "+rt.String("query"))
	}),
)
```

---

## 5. Complete Pure-Go View Builder DSL Reference

### Constructors
- **Containers**:
  - `Column(children ...*Node) *Node`
  - `Row(children ...*Node) *Node`
  - `Box(children ...*Node) *Node`
  - `Panel(title string, children ...*Node) *Node` (sets `title` and `border: true`)
  - `Scroll(children ...*Node) *Node`
  - `Overlay(children ...*Node) *Node`
  - `Modal(id, title string, children ...*Node) *Node` (sets `border: true`, `modal: true`, `trap_focus: true`)
  - `Spacer() *Node` (`Box().Grow(1)`)
- **Primitives & Text**:
  - `Text(text string) *Node`
  - `Textf(format string, args ...any) *Node`
  - `Button(label string, onClick ...string) *Node`
  - `Spinner(preset ...string) *Node` (presets: `"line"`, `"dot"`, `"mini_dot"`, `"jump"`, `"pulse"`, `"points"`, `"globe"`, `"moon"`, `"monkey"`, `"meter"`, `"hamburger"`, `"ellipsis"`)
  - `Progress(percent float64) *Node` (`0.0` .. `1.0`)
  - `Paginator(page, perPage, total int) *Node`
  - `Timer(durationMs int, onTimeout ...string) *Node`
  - `Stopwatch() *Node`
  - `Help(bindings ...HelpBinding) *Node` (`HelpBinding{Keys: []string{"q"}, Description: "Quit"}`)
  - `Cursor() *Node`
- **Editors**:
  - `Input(bindKey string) *Node` (supports `Ctrl+C`, `Ctrl+X`, `Ctrl+V`, `Ctrl+Z`, `Ctrl+Y`, `Ctrl+A`, `Shift+Arrow` selection)
  - `Textarea(bindKey string) *Node`
- **Collections**:
  - `List(items ...string) *Node` (`.Filterable(true)` enables `/` live filtering)
  - `Table(columns []TableColumn, rows [][]string) *Node` (`TableColumn{Title, Width, Align}`)
  - `Tree(items ...TreeItem) *Node` (`TreeItem{ID, Text, Expanded, Children}`)
  - `Viewport(text string, highlights ...HighlightSpan) *Node` (`HighlightSpan{Start, End}`)
  - `Filepicker(entries ...FileEntry) *Node` (`FileEntry{ID, Text, Path, IsDir, Size, Permissions, Description}`)

### Chainable `*Node` Methods
- **Identity & Binding**: `.SetID(id)`, `.Bind(key)`, `.Prop(key, val)`, `.Add(children...)`, `.When(cond)`
- **Dimensions & Flex**: `.Width(w)`, `.Height(h)`, `.Size(w, h)`, `.MinWidth(w)`, `.MaxWidth(w)`, `.MinHeight(h)`, `.MaxHeight(h)`, `.Grow(g)`, `.Shrink(s)`, `.Flex(g)`, `.Padding(p)`, `.Gap(g)`, `.Align(a)`, `.Justify(j)`, `.ZIndex(z)`, `.Absolute(x, y)`
- **Appearance**: `.Text(s)`, `.Title(s)`, `.Placeholder(s)`, `.Border(bool)`, `.Bordered()`, `.FG(color)`, `.BG(color)`, `.Color(fg, bg)`, `.Bold(bool)`, `.Wrap(bool)`, `.Variant(v)`
- **Behavior & State**: `.Hidden(bool)`, `.Disabled(bool)`, `.Focusable(bool)`, `.Modal(bool)`, `.TrapFocus(bool)`, `.Masked(bool)`, `.MaxLength(n)`, `.Filterable(bool)`, `.ShowHeader(bool)`, `.ShowLineNumbers(bool)`, `.SoftWrap(bool)`, `.PageSize(n)`
- **Events & Keys**: `.OnClick(action)`, `.OnChange(action)`, `.OnSubmit(action)`, `.OnSelect(action)`, `.OnActivate(action)`, `.OnTimeout(action)`, `.Key(chord, action)`, `.Keymap(map[string]string)`

---

## 6. Program Options, Middleware, & External Control

### Functional `Option` Builders
- **Source / View**: `WithView(node)`, `WithViewFunc(fn)`, `WithAppFile(path)`, `WithSource(luaCode)`
- **State & Actions**: `WithState(key, val)`, `WithInitialData(map)`, `WithAction(name, fn)`, `WithHandlers(map)`, `WithOnEvent(fn)`, `WithOnReady(fn)`
- **Extensibility Hooks**:
  - `WithMiddleware(mw ...EventMiddleware)` — intercept, log, or short-circuit events
  - `WithInputFilter(func(tcell.Event) tcell.Event)` — rewrite or drop raw terminal events before processing
  - `WithStateObserver(func(key string, oldVal, newVal any))` — observe state changes
  - `WithRenderer(Renderer)` / `WithRendererFunc(fn)` — custom frame rendering hook
  - `WithClipboard(Clipboard)` — custom clipboard (defaults to `OSC52Clipboard`)
- **Terminal & Runtime Modes**:
  - `WithAltScreen(bool)` — fullscreen alternate buffer (default `true` unless inline)
  - `WithInline(height int)` — inline rendering inside primary terminal buffer without clearing history
  - `WithMouse(bool)`, `WithPaste(bool)`, `WithFPS(fps int)`, `WithTheme(Theme)`
  - `WithSocket(path)`, `WithSandbox()`, `WithCheck()`, `WithSnapshot()`, `WithoutPanicRecovery()`

### Thread-Safe External Control on `*Program`
Call from any goroutine while `p.Run()` is active:
- `p.Send(action string, payload map[string]any) error`
- `p.Set(key string, value any) error`
- `p.Patch(id string, props map[string]any) error`
- `p.Reload() error`
- `p.EnterAltScreen() error` / `p.ExitAltScreen() error`
- `p.Quit()`

### Typed State Helpers on `*Runtime`
- `rt.String(key) string`, `rt.Int(key) int`, `rt.Float(key) float64`, `rt.Bool(key) bool`, `rt.Model(key) any`
- `rt.Set(key, val) error`, `rt.AddInt(key, delta) error`, `rt.ToggleBool(key) error`
- `rt.On(action, handler) *Runtime`, `rt.Patch(id, props) error`, `rt.Replace(id, subtree) error`, `rt.ReconcileTree(root) error`

---

## 7. Lua API Reference (When Using `.lua` Views)

- **State & Tree**: `ui.get(key)`, `ui.set(key, val)`, `ui.patch(id, props)`, `ui.patch_all(table)`, `ui.replace(id, node)`
- **Focus**: `ui.focus(id)`, `ui.focused()`, `ui.focus_next()`, `ui.focus_prev()`, `ui.focus_direction("up"|"down"|"left"|"right")`
- **Clipboard & Screen**: `ui.clipboard_get()`, `ui.clipboard_set(text)`, `ui.enter_alt_screen()`, `ui.exit_alt_screen()`
- **Host Dispatch**: `host.dispatch(action, payload, [opts])` (pass `{ debounce = true }` to coalesce rapid events)

---

## 8. Headless Testing & Verification

Test Pure-Go or Lua interfaces in milliseconds without a real TTY:

```go
func TestCounter(t *testing.T) {
	rt, err := lugo.LoadViewFunc(
		func(r *lugo.Runtime) *lugo.Node {
			return lugo.Column(
				lugo.Textf("Count: %d", r.Int("n")).SetID("lbl"),
				lugo.Button("+1", "inc").SetID("btn"),
			)
		},
		lugo.WithState("n", 1),
		lugo.WithAction("inc", func(r *lugo.Runtime, _ any) error {
			return r.AddInt("n", 1)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	_ = rt.Action("inc", nil)
	if rt.Int("n") != 2 {
		t.Fatalf("expected 2, got %d", rt.Int("n"))
	}
}
```
