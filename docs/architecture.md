# Architecture, Embedding & Engine Limits

This document details the internal design of LugoTUI, directory layout conventions, Go integration models (Pure-Go Reactive/Retained vs. Managed Runner vs. Low-level Embedding), concurrency and goroutine ownership rules, and engine capacity limits.

---

## 1. Project Structure & Dependency Graph

```text
lugotui.go                   root library facade (package lugotui)
cmd/lugotui/main.go          standalone CLI executable entry point
tui/
  tui.go                   public runtime wrapper, types, and Options
  program.go               Program runner, functional options, and external control
  builder.go               Pure-Go View Builder DSL constructors
  *_test.go                public API, runner options and integration test suite
internal/engine/
  runtime.go               lifecycle, state restoration, transactions and actions
  tree.go                  nodes, properties, validation, auto-assign IDs and model limits
  builder.go               Pure-Go builder AST constructors and chainable node methods
  hooks.go                 Renderer, Clipboard, OSC52Clipboard, StateObserver interfaces
  lua.go                   Lua setup, bindings and Go/Lua conversion
  layout.go                intrinsic sizing, flex allocation and placement
  render.go                cellBuffer diff-painting, styles, rich text and Unicode wrapping
  input.go                 keyboard, mouse, focus, key event bubbling and editor state
  editors.go               input/textarea validation, regex cache and grapheme offsets
  primitives.go            primitive state, properties, and validation
  primitive_tick.go        timer, stopwatch, and primitive tick/paint/input logic
  collections.go           collection items, state, and validation
  collection_render.go     table, tree, list, viewport, filepicker painting
  collection_input.go      collection keyboard and mouse interaction
  transport.go             Unix socket, JSON operations and snapshots
  tasks.go                 worker lifecycle, cancellation and timer-first debouncing
  *_test.go                engine unit tests, widgets, layout, limits and transport
internal/assets/
  demo.lua                 bundled catalog Lua source
  assets.go                embed.FS and string exports for embedded resources
  assets_test.go           embedded assets verification
internal/runner/
  run.go                   CLI adapter, Options, event loop and terminal lifecycle
  sandbox.go               platform-independent sandbox interface
  sandbox_linux.go         Linux namespace sandbox runner (bubblewrap + prlimit)
  sandbox_other.go         non-Linux sandbox fallback
  signals_unix.go          POSIX signal handling (SIGWINCH, SIGHUP, SIGTERM, SIGINT)
  signals_windows.go       Windows signal handling
  run_test.go              runner unit tests
```

### Dependency Rules
Users import `github.com/VintlGvard/LugoTUI` (root facade) or `github.com/VintlGvard/LugoTUI/tui`. Dependencies are strictly one-way:
- Public facade (`lugotui` / `tui`) depends on `internal/runner` and `internal/engine`.
- `internal/runner` depends on `internal/engine`.
- `internal/engine` embeds assets via `internal/assets`.
- `internal/engine` does **not** parse CLI flags, open application files, subscribe to OS signals, or own the terminal loop.
- Engine modules remain in one package because they share the same mutable node tree and single-goroutine ownership.
- The public runtime wraps the private engine; getters return detached data copies, and mutations use validated methods.

### Extension Guide
- **Add a component:** Constructor in `internal/engine/builder.go` and `internal/engine/lua.go`, property validation in `internal/engine/tree.go`, painting in `internal/engine/render.go`, interaction in `internal/engine/input.go`.
- **Add a Lua operation:** Binding in `internal/engine/lua.go`, atomic application in `internal/engine/runtime.go`.
- **Transport changes:** Belong in `internal/engine/transport.go`, never in the terminal loop.
- **Verification:** Run `make test check`. Benchmark checks: `go test ./tui -run '^$' -bench . -benchmem`. Integration checks: `go test ./tui`. External Lua applications can live anywhere selected with `--app`.

---

## 2. Mental Model & Go Entry Points

The runtime retains a validated node tree, an explicit model, and editor state. Layout calculations use dirty-tracking (`scrollDigest`, `layoutValid`), and terminal output uses a double-buffered cell-diff matrix (`cellBuffer`).

Choose your Go integration path:

- **Pure-Go Reactive (`lugotui.QuickViewFunc` / `tui.WithViewFunc`):** Write a `func(rt *Runtime) *Node` view function. Zero Lua VM overhead. LugoTUI automatically reconciles new trees, preserving widget focus, cursor position, selection, and scroll offsets.
- **Pure-Go Retained (`lugotui.QuickView` / `tui.WithView`):** Build the tree once; mutate via two-way `Bind` or surgical `rt.Set` / `rt.Patch`.
- **Hybrid Go + Lua Live Hot-Reload (`lugotui.RunApp` / `tui.WithAppFile`):** Full 150ms live hot-reload for UI layout and themes without restarting Go. Widget events dispatch directly to Go handlers without Lua boilerplate.
- **Program Runner (`lugotui.NewProgram`):** Full control with functional options (`WithMiddleware`, `WithInputFilter`, `WithStateObserver`, `WithRenderer`, `WithClipboard`) and thread-safe control from other goroutines (`Send`, `Set`, `Patch`, `Reload`, `Quit`).
- **Low-level Embedding (`tui.Load` / `tui.LoadView`):** Creates only a headless runtime. Your own event loop owns the screen and calls `Key`, `Mouse`, `Paste`, `Draw`, `DrainEvents`, and `HandleTask`.

---

## 3. Managed Runner Integration

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/VintlGvard/LugoTUI/tui"
)

func main() {
    err := tui.RunOptions(tui.Options{
        AppFile: "app.lua", // Use the companion Lua file below.
        Theme: tui.Theme{Background: "black", FocusForeground: "cyan"},
        ReservedKeys: &tui.ReservedKeys{
            Quit:   []string{"Ctrl+Q"},
            Reload: []string{"F6"},
        },
        OnEvent: func(r *tui.Runtime, event tui.HostEvent) error {
            if event.Action != "message.send" {
                return nil
            }
            payload, ok := event.Payload.(map[string]any)
            if !ok {
                return fmt.Errorf("message.send: payload must be object")
            }
            text, ok := payload["text"].(string)
            if !ok {
                return fmt.Errorf("message.send: text must be string")
            }
            _, err := r.StartTask("agent.text", func(ctx context.Context) (map[string]any, error) {
                if err := ctx.Err(); err != nil {
                    return nil, err
                }
                return map[string]any{"text": "Echo: " + text}, nil
            })
            return err
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

Save this companion `app.lua` next to the Go program and run `go run .` from that directory:

```lua
local ui = require('ui')
return {
  api_version = 1,
  view = ui.column {
    ui.input {id = 'message', bind = 'message', height = 1},
    ui.button {id = 'send', text = 'Send', on_click = 'send', height = 1},
    ui.text {id = 'reply', text = 'Waiting', height = 1}
  },
  actions = {
    send = function() host.dispatch('message.send', {text = ui.get('message') or ''}) end,
    ['agent.text'] = function(e) ui.patch('reply', {text = e.text}) end
  }
}
```

For a complete worker flow with user-visible errors and cancellation, see [examples/worker](../examples/worker/).

### Managed Runner Details
- **`OnEvent` Execution:** Runs synchronously on the runtime owner goroutine after input, socket mutations, and task completion. Keep it short; use `StartTask` for blocking work.
- **Task Results:** The runner receives task results and invokes `HandleTask`. Success calls the named Lua action with the returned payload. A worker error (including a recovered panic) does not call that action; it appears in the diagnostic line, as do `OnEvent` errors. To show an application-specific error in a widget, return a successful payload describing that error and handle it in Lua.
- **Event Consumption:** When provided, `OnEvent` consumes host events exclusively (including when an agent socket is enabled; socket `events` is not a second subscriber). Without a callback, socket consumers retain the queue; without either, events are discarded.
- **Headless Checks:** `Check` and `Snapshot` options validate and render a single frame without opening a terminal window; they do not run the interactive event/worker loop.
- **Runtime Lifecycle:** The runner owns `Close`. Do not close a runtime passed to `OnEvent`, consume its task channel, or keep the runtime for later use. Successful reload closes the old runtime, cancels its tasks, and supplies a new runtime to subsequent callbacks; failure keeps the old runtime active. Shutdown cancels tasks without joining workers.
- **Reserved Keys:** `ReservedKeys: nil` preserves `Ctrl+Q` / `Ctrl+C` to quit and `F5` to reload. A non-nil struct replaces both lists; omitted or empty lists disable that reserved action and let keys pass through to Lua. Names follow `tcell.EventKey.Name()`. OS interrupt/termination signals still terminate the process even when keyboard shortcuts are disabled.
- **Theming:** `Theme{}` retains built-in defaults; empty fields inherit defaults. Set `Options.Theme` for the stock runner or call `Runtime.SetTheme(theme)` when embedding. Explicit node `fg`/`bg` and rich-span colors override theme colors. Reload preserves the runtime theme. `DefaultTheme()` returns built-in values.

---

## 4. Low-Level Embedding

This headless example owns the screen directly and handles one worker result:

```go
package main

import (
    "context"
    "log"

    "github.com/VintlGvard/LugoTUI/tui"
    "github.com/gdamore/tcell/v2"
)

func run() error {
    r, err := tui.Load(`return {
        view = ui.text{id='result', text='Waiting'},
        actions = {done=function(e) ui.patch('result', {text=e.text}) end}
    }`)
    if err != nil {
        return err
    }
    defer r.Close()

    screen := tcell.NewSimulationScreen("UTF-8")
    if err := screen.Init(); err != nil {
        return err
    }
    defer screen.Fini()
    screen.SetSize(40, 5)

    r.Draw(screen, "")
    _, err = r.StartTask("done", func(ctx context.Context) (map[string]any, error) {
        if err := ctx.Err(); err != nil {
            return nil, err
        }
        return map[string]any{"text": "Ready"}, nil
    })
    if err != nil {
        return err
    }

    if err := r.HandleTask(<-r.TaskResults()); err != nil {
        return err
    }
    r.Draw(screen, "")
    return nil
}

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```

- **Go-side tree updates:** `Runtime.Patch(id, props)` and `Runtime.Replace(id, *tui.Node)` validate and commit Go-side tree updates directly without JSON overhead.
- **Manual Reload:** `Reload(source)` returns a candidate runtime. On success, close the old runtime and adopt the candidate. Failure leaves the old runtime fully usable. `Close` is idempotent; other calls require an active runtime.

---

## 5. Ownership, Goroutines & Background Tasks

- **Single-Goroutine Ownership:** Every runtime method belongs to one owner goroutine, including `StartTask`, `CancelTask`, `TaskResults`, and `Close`. `OnEvent` executes on that goroutine.
- **Thread Isolation:** No `Runtime` method is goroutine-safe, including read-only getters. Never pass the `Runtime` or raw Lua values to background workers. Capture required input data by value or detached copy and return plain Go payloads. The worker's `context.Context` is safe to observe from the worker; cancellation is cooperative.
- **Detached Value Copies:** Read `Revision()` and `Focus()`; `Node(id) *Node`, `Model(key) any`, and `State(id) WidgetState` return detached deep copies. Modifying those copies has no effect on the runtime. Use `Patch`, `Replace`, `SetTheme`, and `SetFocus` for updates.
- **Worker Management:** `StartTask(action, work)` validates an existing Lua action and starts a Go worker with a cancellable context (maximum 32 pending tasks, including completions not yet handled). The owner reads `TaskResults()` and calls `HandleTask(result)` to invoke the Lua action. `CancelTask(id)` cancels context and discards pending results; it does not wait for the worker. `HandleTask` ignores cancelled, stale and foreign results. Workers from an old runtime are not transferred during reload.
- **Task Results Channel:** `TaskResults()` is `nil` before the first task is launched. Its channel is not closed by `Close`; do not `range` over it waiting for shutdown. `Close` signals context cancellation and returns without waiting to join workers.
- **Supported Payload Types:** Workers must return supported Go types:
  - `nil`, `string`, `bool`
  - `int`, `int64` (within ±2^53 integer range)
  - finite `float64`
  - `[]string`, `[]any`, `map[string]any`
  - Workers must not mutate payloads after returning. Worker panics are converted to error diagnostics.

---

## 6. Engine Limits & Performance Specifications

| Metric / Parameter | Hard Limit / Specification |
| --- | --- |
| Max Tree Nodes | 4,096 nodes |
| Max Tree Depth | 64 levels |
| Max Lua Script Size | 1 MiB + 1 byte (startup & reload) |
| Max Model Storage | 1 MiB serialized / 1,024 keys |
| Max Pending Host Events | 256 events |
| Max Undo Stack | 64 entries (~1 MiB total) |
| Lua-to-Go Expanded Values | 262,144 values (including keys; aliases count on each visit) |
| Aggregate String Budget | 1 MiB per conversion pass |
| Lua Init Timeout | 250 ms |
| Lua Action Timeout | 50 ms |
| Socket Request Size | 1 MiB (excluding line delimiter) |
| Max Concurrent Tasks | 32 pending tasks |

### Rendering & Layout Performance
- **Diff Painting:** The renderer paints only the visible scene; `tcell` emits changed terminal cells via double-buffered diffing.
- **Text Efficiency:** Text painting stops immediately after reaching the viewport boundary.
- **Measurement Cache:** Layout caches intrinsic measurements per layout pass. Suggestions skip text scanning when unavailable.
- **Textarea Indexing:** Textarea reuses an internal grapheme/position index, rebuilding only on edits or container width changes, painting only visible rows.
- **Idle Scheduling:** No perpetual FPS loop or continuous compositor thread. The 16ms animation ticker runs only while `NeedsTick()` is true (including a blinking editor cursor). File-backed apps poll metadata every 150ms for reload. Idle CPU depends on the UI and environment; no universal 0.00% claim is made.

### Automated Benchmarks & Hot-Path Latency

Reproduce full test and benchmark suite locally:
```bash
go test -bench=. -benchmem ./...
```

#### Raw Benchmark Output
```text
goos: linux
goarch: amd64
pkg: github.com/VintlGvard/LugoTUI/internal/engine
cpu: 12th Gen Intel(R) Core(TM) i5-12450H
BenchmarkVisibleNodes-12    	  117946	      9976 ns/op	       0 B/op	       0 allocs/op
BenchmarkLargeScene-12      	    2359	    535089 ns/op	   50240 B/op	      12 allocs/op
BenchmarkLayout-12          	   25378	     47100 ns/op	     768 B/op	      11 allocs/op

pkg: github.com/VintlGvard/LugoTUI/tui
cpu: 12th Gen Intel(R) Core(TM) i5-12450H
BenchmarkInitLoad-12              	     292	   4137183 ns/op	 2591032 B/op	   35525 allocs/op
BenchmarkLayoutPass-12            	   25824	     47256 ns/op	     768 B/op	      11 allocs/op
BenchmarkRenderDiffPass-12        	    4129	    297008 ns/op	     248 B/op	      18 allocs/op
BenchmarkStateMutation-12         	    6831	    165116 ns/op	   55768 B/op	     419 allocs/op
BenchmarkStateMutationPatch-12    	     444	   2649788 ns/op	  331930 B/op	   42590 allocs/op
BenchmarkLargeTextDraw-12         	    1208	    968547 ns/op	  237698 B/op	   25611 allocs/op
BenchmarkTextareaCachedDraw-12    	    6106	    194894 ns/op	       0 B/op	       0 allocs/op
```

#### Technical Rationale for Performance
1. **In-Memory Retained Tree:** Direct pointer traversal (`*Node`) eliminates IPC and JSON serialization overhead during frame updates. Tree mutations clone only modified subtrees transactionally.
2. **Zero Reflection in Hot Paths:** Traversal, layout calculations, and rendering use typed switch dispatch and fast property accessors without Go runtime `reflect.Value` costs.
3. **Double-Buffered Cell Diffing (`cellBuffer` + `tcell`):** Writes characters to an in-memory double-buffer diff matrix. Only mutated cells are sent to `tcell` and emitted over stdout as terminal escape sequences.
4. **Layout Dirty Cache:** Unchanged frames skip expensive Flexbox calculations via `scrollDigest` and `layoutValid` state tracking.
5. **Buffer & Measurement Reuse:**
   - Multi-line text editors reuse cached grapheme and line-break offsets (`EditorIndex`), recalculating only on content mutations or container resizes.
   - Text rendering caches split lines and skips off-screen lines immediately after reaching viewport boundaries (`0 B/op`, `0 allocs/op` on cached draws).
   - Visible node collection allocates zero heap memory (`0 B/op`, `0 allocs/op`).

### Architectural Boundaries & Ceilings
- LugoTUI implements a Flexbox subset, not full CSS compatibility.
- Parity with general-purpose GUI or heavy code editor frameworks is not claimed.
