---
name: lugotui
description: Build, inspect, test, and debug Pure Go and Go+Lua TUI applications with LugoTUI (v0.2.3+). Covers the Go DSL (`github.com/VintlGvard/LugoTUI`), Lua UI declaration, Flexbox widgets, grapheme-safe state binding, 2D spatial/directional navigation, widget keymaps, background tasks, live reload, CLI/sandbox flags, and JSON Lines IPC socket control.
---

# LugoTUI Development & Agent Control Skill (v0.2.3)

Build terminal user interfaces in **Pure Go** (`github.com/VintlGvard/LugoTUI`, Go 1.24+) or **Go + Embedded Lua** (`require("ui")`) with deterministic Flexbox layout, grapheme-safe editing, background task orchestration, panic-safe terminal recovery, and JSON Lines IPC.

## When to Activate
- Creating or editing LugoTUI applications in Pure Go (`lugotui` / `tui`) or Lua (`app.lua`).
- Configuring `Program` options, `Middleware`, Go actions (`WithAction`, `WithHandlers`), `OnEvent`, custom `Canvas` widgets, or themes.
- Debugging layout, focus isolation, modal `z_index` mouse blocking, keymap/editor conflicts, Lua rollbacks, or Unicode/emoji text editing.
- Running `lugotui` CLI checks (`--check`, `--snapshot`, `--sandbox`) or automating a live TUI over `--socket` JSON Lines IPC.

---

## 1. Execution Models & Entry Points

### Model A: Pure Go (`github.com/VintlGvard/LugoTUI`)
Import `lugotui "github.com/VintlGvard/LugoTUI"` (or `tui "github.com/VintlGvard/LugoTUI/tui"`):

```go
package main

import (
	"log"

	lugotui "github.com/VintlGvard/LugoTUI"
)

func main() {
	err := lugotui.QuickViewFunc(
		func(rt *lugotui.Runtime) *lugotui.Node {
			name := rt.String("name", "world")
			return lugotui.Panel("Hello",
				lugotui.Input("name", "Your name...").Width(30),
				lugotui.Textf("Hi, %s!", name),
				lugotui.Button("Quit", "quit"),
			).Padding(1).Gap(1)
		},
		map[string]lugotui.ActionHandler{
			"quit": func(rt *lugotui.Runtime, _ any) error {
				return rt.Quit()
			},
		},
		lugotui.WithInitialData(map[string]any{"name": "Lugo"}),
	)
	if err != nil {
		log.Fatal(err)
	}
}
```

Key entry points on `lugotui`:
- `QuickViewFunc(fn ViewFunc, handlers map[string]ActionHandler, opts ...Option) error`
- `QuickView(root *Node, handlers map[string]ActionHandler, opts ...Option) error`
- `QuickRun(luaSource string, handlers map[string]ActionHandler, opts ...Option) error`
- `RunApp(appFile string, opts ...Option) error`
- `RunSource(luaSource string, opts ...Option) error`
- `NewProgram(opts ...Option) *Program` (`p.Run() error`)

### Program Options (`Option`)
- **View / Source**: `WithViewFunc(fn)`, `WithView(root)`, `WithAppFile(path)`, `WithSource(luaSource)`
- **Initial State**: `WithInitialData(map[string]any)`, `WithState(key, val)`
- **Actions, Events & Hooks**:
  - `WithAction(name, handler)` / `WithHandlers(map[string]ActionHandler)` — register Go actions (and auto-wire host dispatch). `WithHandlers` snapshots the caller map so later mutations do not affect the program.
  - `WithOnEvent(func(*Runtime, HostEvent) error)` — chained cleanly with other `WithOnEvent` / `WithHandlers` options regardless of option order; invoked **only after** action state commits.
  - `WithMiddleware(mw ...EventMiddleware)` — composes cleanly alongside `WithAction` / `WithHandlers` (`GoActions` are preserved alongside `Middlewares`).
  - `WithInputFilter(InputFilter)`, `WithStateObserver(obs ...StateObserver)`, `WithOnReady(func(*Runtime))`
- **Terminal & Rendering**: `WithTheme(Theme)`, `WithAltScreen(bool)`, `WithInline(height int)`, `WithMouse(bool)`, `WithPaste(bool)`, `WithFPS(fps int)`, `WithClipboard(Clipboard)`, `WithRenderer(Renderer)`, `WithRendererFunc(...)`, `WithScreen(tcell.Screen)`, `WithReservedKeys(ReservedKeys)`, `WithoutPanicRecovery()`
- **Tooling / Automation**: `WithSocket(path)`, `WithCheck()`, `WithSnapshot()`, `WithSandbox()`

Default theme: `lugotui.DefaultTheme()` (returns `lugotui.Theme`).

### Runtime Stability Invariants (v0.2.3)
- **Panic recovery**: enabled by default; panics inside `Program.Run()` restore the terminal cleanly and return an error wrapping `lugotui.ErrPanicRecovered`.
- **Dynamic idle tick & diff skip**: when `NeedsTick() == false`, the idle ticker stops to avoid spinning CPU; redraw skips `screen.Show()` when neither the cell buffer nor cursor state changed, and updates cursor state before `screen.Show()`.

---

## 2. Declarative Go DSL & Widget Constructors

All constructors below are exported by both `lugotui` and `tui`:

### Containers & Control Flow
- `NewNode(kind string, children ...*Node) *Node`
- `Column(children ...*Node) *Node`, `Row(children ...*Node) *Node`, `Box(children ...*Node) *Node`
- `Overlay(children ...*Node) *Node`, `Spacer() *Node`
- `Panel(title string, children ...*Node) *Node` (sets border + title)
- `Scroll(children ...*Node) *Node`
- `Modal(id, title string, children ...*Node) *Node`
- `If(cond bool, node *Node) *Node`, `IfElse(cond bool, thenNode, elseNode *Node) *Node`
- `Map[T any](items []T, fn func(index int, item T) *Node) []*Node`

### Standard Widgets
- `Text(text string) *Node`, `Textf(format string, args ...any) *Node`
- `Button(label string, onClick ...string) *Node` (`text` defaults to `label` when absent)
- `Input(bindKey string, placeholder ...string) *Node`
- `Textarea(bindKey string, placeholder ...string) *Node`
- `List(items ...string) *Node`
- `Table(columns []TableColumn, rows [][]string) *Node` (`TableColumn{Title string, Width int, Align string}`)
- `Tree(items ...TreeItem) *Node` (`TreeItem{ID string, Text string, Expanded bool, Children []TreeItem}`)
- `Viewport(text string, highlights ...HighlightSpan) *Node` (`HighlightSpan{Start int, End int}`)
- `Filepicker(entries ...FileEntry) *Node` (`FileEntry{Text string, Name string, Path string, IsDir bool, Size any, Permissions string}`)
- `Spinner(preset ...string) *Node`, `Progress(percent float64) *Node`
- `Paginator(page, perPage, total int) *Node`
- `Timer(durationMs int, onTimeout ...string) *Node`, `Stopwatch() *Node`
- `Help(bindings ...HelpBinding) *Node` (`HelpBinding{Keys []string, Description string, Action string, Disabled bool}`)
- `Cursor() *Node`

### Custom Drawing Widget (`Canvas`)
Use `Custom(paintFn func(c Canvas)) *Node`:
```go
lugotui.Custom(func(c lugotui.Canvas) {
    b := c.Bounds()
    c.Fill(b, ' ')
    c.Printf(b.X+1, b.Y, tcell.StyleDefault.Bold(true), "W:%d H:%d Focused:%v", b.W, b.H, c.IsFocused())
}).Focusable(true).Size(40, 8)
```
Exact `Canvas` methods:
- `Bounds() Rect`
- `SetCell(x, y int, r rune, st ...tcell.Style)`
- `Print(x, y int, text string, st ...tcell.Style)`
- `Printf(x, y int, st tcell.Style, format string, args ...any)`
- `Fill(r Rect, ch rune, st ...tcell.Style)`
- `LineH(x, y, w int, ch rune, st ...tcell.Style)`
- `LineV(x, y, h int, ch rune, st ...tcell.Style)`
- `IsFocused() bool`

For direct low-level keyboard handling on a custom node, chain `.OnRawKey(func(e *tcell.EventKey) bool)`.

---

## 3. Chainable `*Node` Methods (Verified API)

- **Identity, Binding & Composition**:
  - `.SetID(id)`, `.Prop(key, val)`, `.PropsMap(map[string]any)`, `.Add(children...)`
  - `.When(cond bool, fn func(*Node))` *(mutates conditionally only when `cond` is true)*
  - `.Bind(key)`, `.Source(key)`
- **Dimensions & Positioning**:
  - `.Width(w)`, `.Height(h)`, `.Size(w, h)`, `.MinWidth(w)`, `.MaxWidth(w)`, `.MinHeight(h)`, `.MaxHeight(h)`
  - `.Grow(g)` / `.Flex(g)`, `.Shrink(s)`, `.Padding(p)`, `.Margin(m)`, `.Gap(g)`
  - `.Absolute(left, top)`, `.Top(v)`, `.Right(v)`, `.Bottom(v)`, `.Left(v)`, `.Inset(top, right, bottom, left)`
- **Text, Borders & Colors**:
  - `.Text(s)`, `.Textf(format, args...)`, `.Title(s)`, `.Placeholder(s)`
  - `.FG(color)`, `.BG(color)`, `.Color(fg, bg)`, `.Bold(bool)`, `.Gradient(startColor, endColor)`
  - `.Align(s)`, `.Justify(s)`, `.Wrap(bool)`, `.WrapMode(mode)`
  - `.Border(v...)`, `.BorderStyle(s)`, `.BorderFG(c)`, `.BorderBG(c)`, `.BorderColor(fg, bg...)`, `.TitleFG(c)`
  - `.BorderGlyphs(bg)` with presets: `BorderSingle`, `BorderRounded`, `BorderDouble`, `BorderThick`, `BorderASCII`
  - `.ButtonStyle(s)`, `.Plain()`, `.Filled()`, `.Chevrons()`, `.Parens()`, `.Prefix(p)`, `.Suffix(s)`
  - `.CursorPrefix(cp)`, `.SelectedFG(c)`, `.SelectedBG(c)`, `.SelectedBold(v...)`
- **Behavior, State & Navigation**:
  - `.Hidden(bool)`, `.Disabled(bool)`, `.Focusable(v...)`
  - `.Modal(bool)`, `.Backdrop(mode)`, `.CloseOnEsc(bool)`, `.ZIndex(z)`
  - `.Echo(mode)`, `.Masked(bool)`, `.CharLimit(n)` / `.MaxLength(n)`, `.ValidationPattern(regex)`
  - `.Filterable(bool)`, `.MultiSelect(bool)`, `.ItemHeight(h)`
  - `.ShowIcons(bool)`, `.ShowHidden(bool)`, `.CurrentDir(dir)`
  - `.Items(items...)`, `.Selected(idx)`, `.Running(bool)`, `.Percent(p)`, `.ShowPercentage(bool)`
  - `.ShowHeader(bool)`, `.ShowLineNumbers(bool)`, `.SoftWrap(bool)`, `.Mode(m)`, `.Preset(p)`, `.Duration(ms)`, `.Interval(ms)`
  - `.Page(page, perPage, total int)`
  - `.NavUp(id)`, `.NavDown(id)`, `.NavLeft(id)`, `.NavRight(id)`, `.Nav(up, down, left, right)`
- **Events & Keymaps**:
  - Action names: `.OnClick(action)`, `.OnChange(action)`, `.OnSubmit(action)`, `.OnSelect(action)`, `.OnNavigate(action)`, `.OnTimeout(action)`, `.OnKey(action)`
  - Typed Go callbacks: `.OnClickDo(func())`, `.OnClickErr(func() error)`, `.OnChangeText(func(string))`, `.OnChangeTextErr(func(string) error)`, `.OnSubmitText(func(string))`, `.OnSubmitTextErr(func(string) error)`, `.OnSelectItem(func(int, any))`, `.OnSelectItemErr(func(int, any) error)`, `.OnTimeoutDo(func())` *(plus dynamic `...Func(any)` variants)*
  - Keymaps: `.Key(k, action)`, `.KeyDo(k, func())`, `.KeyErr(k, func() error)`, `.KeyFunc(k, fn)`, `.Keymap(map[string]string)`

*(Note: `*Node` does **not** have `.Italic()`, `.Underline()`, or `.PageSize()`; use `.Page(page, perPage, total)` for paginators.)*

---

## 4. Input, Layout, Focus & Unicode Rules (v0.2.3)

- **Editor key capture**: Active focused `input`, `textarea`, and active collection filter editing (`/`) consume typing and editing keys *before* global or ancestor `keymap` / `help` bindings. Typed runes like `'q'` or `'?'` no longer fall through to global hotkeys while editing.
- **Grapheme-cluster correctness**: Text measurement (`TextWidth`), wrapping, truncation, `input`/`textarea` cursor movement, selection, and `char_limit` all count **Unicode grapheme clusters** via `uniseg` (so Cyrillic and multi-codepoint emoji stay intact).
- **Modal focus isolation & Z-order mouse blocking**:
  - Open modals trap keyboard focus traversal inside the modal scope.
  - Sibling overlays with higher `z_index` block mouse clicks and scroll wheel pass-through to lower layers (such as underlying `scroll` containers).
- **Read-only Draw & styled wrapping**: `Draw` does not mutate layout tree nodes (e.g., `scroll` computes bounds without mutating node props), and wrapped lines preserve spans cleanly by `[StartByte, EndByte)` byte offsets.

---

## 5. Embedded Lua Apps (`require("ui")`) & Safety Budgets

Lua apps (`--app app.lua`, `RunApp`, `RunSource`, `QuickRun`, `Load`) run inside embedded `gopher-lua` with **no** `os`, `io`, or arbitrary `require` — only `require("ui")`.

- `ui.panel { title = "...", ... }` already implies borders (`border = true` is optional).
- **Transactional rollback**: if a Lua action errors or times out, LugoTUI restores `_G` globals, model `Data`, widget state, **and** Lua closure upvalues (`*lua.Upvalue`), while discarding uncommitted `HostEvent`s so `OnEvent` never observes rolled-back state.
- **Built-in limits** (`internal/core` + `internal/engine`):
  - Execution budgets: `250ms` init timeout, `50ms` action timeout, bounded expansion budget (`luaBudget`) and `string.rep` cap.
  - Structural limits: `MaxNodes = 4096`, `MaxText = 1 MiB` (`1 << 20`), `MaxDepth = 64`, `MaxIDBytes = 128`, `MaxModelKeys = 1024`, `MaxValueDepth = 32`.

---

## 6. Runtime State, Background Tasks & Live Program Control

Inside action handlers / `ViewFunc` (`rt *lugotui.Runtime`):
- **Model access**: `Model`, `String`, `Int`, `Float`, `Bool`, `Strings`, `Map`, `Set`, `Update`, `AddInt`, `ToggleBool`, `Toggle`
- **Focus & UI**: `Revision()`, `Focus()`, `SetFocus(id)`, `FocusDirection(dir)`, `FocusNext()`, `FocusPrev()`, `Quit()`, `Theme()`, `SetTheme(t)`, `Emit(action, payload)`, `On(action, handler)`
- **Dynamic tree & JSON**: `Action(name, event)`, `Patch(id, props)`, `Replace(id, subtree)`, `ApplyJSON(data)`, `ReconcileTree(root)`, `State(id) WidgetState`
- **Background tasks**:
  - `StartTask(action string, work func(context.Context) (map[string]any, error)) (uint64, error)`
  - `StartTaskNamed(name, action string, work func(context.Context) (map[string]any, error)) (uint64, error)`
  - `StartTaskDebounced(name string, delay time.Duration, action string, work func(context.Context) (map[string]any, error)) (uint64, error)`
  - `CancelTask(id uint64)`, `CancelTaskNamed(name string)`
- **Headless testing / rendering**: `Layout(w, h)`, `Draw(screen, diagnostic)`, `Key(ev)`, `Mouse(ev)`, `Paste(text)`, `Tick(now)`, `NeedsTick()`, `Snapshot()`, `AgentReply(data)`

From outside `p *lugotui.Program` (goroutine-safe control channel):
- `p.Send(action, payload)`, `p.Set(key, val)`, `p.Patch(id, props)`, `p.Reload()`, `p.EnterAltScreen()`, `p.ExitAltScreen()`, `p.Quit()`

---

## 7. CLI Flags, Testing & JSON Lines IPC

### CLI (`cmd/lugotui`)
```bash
lugotui --app app.lua                 # Run Lua app (default: embedded demo)
lugotui --app app.lua --check         # Validate & render without terminal
lugotui --app app.lua --snapshot      # Print semantic JSON snapshot (implies --check)
lugotui --app app.lua --socket /tmp/app.sock
lugotui --sandbox --app app.lua --check
lugotui --version
```
`--sandbox` enforces Linux `bubblewrap` + `prlimit` process isolation: requires a static `CGO_ENABLED=0` binary on Linux, regular `--app` file `<= 1 MiB`, no network or host filesystem access, and `--socket` is disabled.

### JSON Lines IPC Protocol (`--socket`, permissions `0600`)
Each request/response is one JSON object per line:
- **Snapshot**: `{"method":"snapshot"}` -> returns snapshot with `Revision`, `Focus`, `Nodes` (including `Bounds`), etc.
- **Apply**: `{"method":"apply","packet":{"revision":N,"operations":[...]}}` -> revision-checked atomic `patch` / `replace` operations.
- **Message**: `{"method":"message","action":"...","payload":{...}}` -> dispatches an action.
- **Events**: `{"method":"events"}` -> drains queued host events.

Exported protocol error codes (`lugotui.ErrorCode(err)`): `CodeInvalidJSON`, `CodeUnknownCommand`, `CodeValidationError`, `CodeRevisionConflict`, `CodeLuaError`, `CodeTaskLimitExceeded`, `CodeInternalError`.
