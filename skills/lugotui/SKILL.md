---
name: lugotui
description: Build, inspect, test, and debug Go+Lua TUI applications with LugoTUI. Covers Lua UI declaration, Flexbox widgets, state binding, Go background tasks, hot reload, CLI commands, and JSON Lines IPC socket control.
---

# LugoTUI Agent Skill

LugoTUI is a terminal UI library and runner for Go and Lua. It combines declarative Lua component trees, Flexbox layout, two-way model binding, hot reload, Go background worker tasks, and a JSON Lines IPC socket.

## When to Activate

- Writing or refactoring LugoTUI Lua applications (`app.lua`)
- Integrating LugoTUI into Go programs (`github.com/VintlGvard/LugoTUI/tui`)
- Implementing background tasks, event handling, or state synchronization
- Automating or inspecting running TUIs via Unix domain socket (IPC)
- Debugging layout issues, hot-reload state resets, or concurrency bugs

---

## 1. Execution Modes

Choose the right mode based on the user's needs:

| Mode | Entry Point | Best For |
| --- | --- | --- |
| **Go Library** | `tui.RunOptions(opts)` | Apps requiring Go business logic, database/HTTP access, or custom background workers. |
| **Standalone CLI** | `lugotui --app app.lua` | Pure Lua UIs, rapid prototyping, scripts, or apps controlled externally via IPC. |
| **IPC Controller** | Unix Domain Socket | Headless automation, testing, or external agent control of an active interface. |

---

## 2. Lua Application Structure (`app.lua`)

A LugoTUI application returns a table specifying `api_version`, `view`, `actions`, and optional `keymap`:

```lua
local ui = require('ui')

return {
    api_version = 1,

    -- Factory function: called once on load or hot reload (F5)
    view = function()
        return ui.column {
            grow = 1,
            padding = 1,
            gap = 1,
            ui.panel {
                title = "Dashboard",
                border = "rounded",
                ui.text { bind = "status_msg" },
                ui.row {
                    gap = 1,
                    ui.input { id = "user_input", bind = "query", placeholder = "Enter query..." },
                    ui.button { text = "Submit", on_click = "submit_query" },
                },
            },
        }
    end,

    -- Event handlers called when user interacts with UI or host dispatches
    actions = {
        submit_query = function(event)
            local query = ui.get("query")
            if not query or query == "" then
                ui.set("status_msg", "Please enter a non-empty query.")
                return
            end
            ui.set("status_msg", "Processing: " .. query)
            -- Dispatch to Go host (if embedded in Go)
            host.dispatch("start_search", { query = query })
        end,

        -- Called when Go host background task finishes
        search_complete = function(event)
            if event.payload and event.payload.result then
                ui.set("status_msg", "Found: " .. event.payload.result)
            elseif event.payload and event.payload.error then
                ui.set("status_msg", "Error: " .. event.payload.error)
            end
        end,
    end,

    -- Global key shortcuts
    keymap = {
        ["ctrl+s"] = "submit_query",
    },
}
```

---

## 3. Widget Reference & Flexbox Layout

### Layout Containers
- `ui.column { ... }` — Vertical Flexbox container (`flex-direction: column`).
- `ui.row { ... }` — Horizontal Flexbox container (`flex-direction: row`).
- `ui.box { ... }` — Generic container.
- `ui.panel { title = "Title", border = "rounded", ... }` — Container with a border and title.
  Borders: `"single"`, `"double"`, `"rounded"`, `"none"`.
- `ui.scroll { ... }` — Scrollable container.
- `ui.overlay { base = node, overlay = node }` — Stacks overlay on top of base.

### Common Layout Properties
- `grow = 1`, `shrink = 0`, `basis = 10`
- `width = 20` | `"50%"` | `"auto"`, `height = 10` | `"auto"`
- `padding = 1`, `margin = 1`, `gap = 1`
- `justify = "start"` | `"center"` | `"end"` | `"space-between"` | `"space-around"`
- `align = "start"` | `"center"` | `"end"` | `"stretch"`
- `align_self = "start"` | `"center"` | `"end"` | `"stretch"`

### Interactive & Input Widgets
- `ui.input { id = "id", bind = "key", placeholder = "...", on_change = "action" }` — Single-line input.
- `ui.textarea { id = "id", bind = "key", placeholder = "..." }` — Multi-line editor.
- `ui.button { text = "Save", on_click = "action_name" }` — Clickable button.
- Always provide a stable, unique `id` for inputs/textareas to preserve cursor position across hot reload.

### Display & Data Widgets
- `ui.text { text = "Literal", bind = "model_key", spans = { { text = "Red", fg = "red" } } }`
- `ui.list { id = "list", items = { "A", "B" }, bind = "selected_index", on_select = "action" }`
- `ui.table { columns = { "ID", "Name" }, rows = { { "1", "Alice" }, { "2", "Bob" } } }`
- `ui.viewport { text = "Large text stream..." }` — High-performance scrollable viewport.
- `ui.spinner { preset = "dots", interval = 80 }` — Animated loader.
- `ui.progress { percent = 0.5 }` — Progress bar (0.0 to 1.0).

---

## 4. State & Model Management

LugoTUI maintains an explicit model layer separate from widget instances:

1. **`ui.get(key)` / `ui.set(key, value)`**: Read and write model values.
2. **`bind = "model_key"`**: Two-way binding for `input`/`textarea`, one-way for `text`.
3. **`ui.patch(id, props)`**: Update properties of an existing node by ID without rebuilding the tree.
4. **`ui.replace(id, new_node)`**: Replace a node and its children.
5. **`ui.focus(id)`**: Move keyboard focus to the target node.
6. **Hot Reload (F5)**: Lua globals and local variables are reset on reload. Model state (`ui.set`) and editor states with stable `id`s are preserved. Always store persistent state in the model.

---

## 5. Go Host Integration (`main.go`)

When embedding LugoTUI in a Go application:

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
)

func main() {
	var taskID uint64

	opts := tui.Options{
		AppFile: "app.lua",
		OnEvent: func(rt *tui.Runtime, event tui.HostEvent) {
			switch event.Action {
			case "start_search":
				query, _ := event.Payload["query"].(string)

				// Cancel previous task if running
				if taskID != 0 {
					rt.CancelTask(taskID)
				}

				var err error
				// StartTask registers a background worker.
				// On completion, LugoTUI invokes the Lua action "search_complete".
				taskID, err = rt.StartTask("search_complete", func(ctx context.Context) (map[string]any, error) {
					// Simulate background work
					select {
					case <-time.After(500 * time.Millisecond):
						return map[string]any{"result": "Matched: " + query}, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				})
				if err != nil {
					fmt.Printf("failed to start task: %v\n", err)
				}
			}
		},
	}

	if err := tui.RunOptions(opts); err != nil {
		panic(err)
	}
}
```

### Concurrency & Thread-Safety Rules
- **Rule 1 (Owner Goroutine)**: `*tui.Runtime` methods (`StartTask`, `CancelTask`, `Model`, `Close`) are NOT thread-safe. They must ONLY be called from `OnEvent` or the runtime owner goroutine.
- **Rule 2 (No Runtime Pointers in Workers)**: Background worker functions run on separate goroutines. NEVER pass `*tui.Runtime` into a worker or touch the Lua state from a worker.
- **Rule 3 (Data Transfer)**: Pass inputs by copying values before starting the task. Return outputs via the `map[string]any` payload. LugoTUI safely dispatches the payload to the Lua action on the main UI goroutine.
- **Rule 4 (Handle Cancellation)**: Workers must check `<-ctx.Done()` to clean up resources when canceled or when the app exits.

---

## 6. Testing & Headless Verification

Verify LugoTUI applications headlessly without opening an interactive terminal:

```sh
# 1. Syntax, parse, layout, and render check (exit code 0 on success)
lugotui --app app.lua --check

# 2. Inspect layout tree and properties as JSON
lugotui --app app.lua --snapshot

# 3. In Go tests: load runtime headlessly
```

```go
func TestAppLayout(t *testing.T) {
	rt, err := tui.Load(tui.RuntimeOptions{AppFile: "app.lua"})
	if err != nil {
		t.Fatalf("failed to load app: %v", err)
	}
	defer rt.Close()

	if err := rt.Layout(80, 24); err != nil {
		t.Fatalf("layout failed: %v", err)
	}
}
```

---

## 7. Socket IPC Protocol (JSON Lines)

Enable socket IPC by passing `--socket`:
```sh
lugotui --app app.lua --socket /tmp/lugotui.sock
```

Communicate via JSON Lines over the Unix domain socket:

### Snapshot (`snapshot`)
Request the current UI tree and state:
```json
{"cmd":"snapshot","id":1}
```
Response:
```json
{"id":1,"ok":true,"result":{"revision":0,"focus":"user_input","nodes":[{"id":"user_input","type":"input","value":""}]}}
```

### Apply Action (`apply`)
Trigger Lua actions or update inputs:
```json
{
  "cmd": "apply",
  "id": 2,
  "revision": 0,
  "actions": [
    {"type": "input", "id": "user_input", "value": "test query"},
    {"type": "call", "action": "submit_query"}
  ]
}
```

### Subscribe to Events (`events`)
Stream UI and host events in real time:
```json
{"cmd":"events","id":3}
```

---

## 8. Common Pitfalls & Troubleshooting

| Symptom | Cause | Solution |
| --- | --- | --- |
| `invalid property ...` during `--check` | Wrong type passed in Lua table (e.g. string instead of number for `height`). | Check widget reference; numeric dimensions take integers, percentages use `"50%"`. |
| Input cursor resets on hot reload | Widget lacks stable `id`. | Add `id = "unique_input_name"` to `ui.input` or `ui.textarea`. |
| Data resets on hot reload (F5) | State stored in Lua local variables instead of model. | Use `ui.set("key", val)` and `ui.get("key")` for persistent state. |
| Panic: concurrent map read/write | Worker goroutine accessed `*tui.Runtime` or shared variable. | Keep worker pure; pass data in via copy, return data via `map[string]any`. |
| `bind: invalid argument` on macOS socket | Unix socket path exceeds OS limit (~104 chars). | Place socket in `/tmp/lugotui.sock` or current working directory. |
