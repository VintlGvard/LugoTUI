---
name: lugotui
description: Build, inspect, test, and debug Go+Lua TUI applications with LugoTUI. Covers Lua UI declaration, Flexbox widgets, state binding, 2D spatial & directional navigation, widget keymaps, Go background tasks, hot reload, CLI commands, and JSON Lines IPC socket control.
---

# LugoTUI Agent Skill

LugoTUI is a terminal UI library and runner for Go and Lua. It combines declarative Lua component trees, Flexbox layout, two-way model binding, 2D spatial navigation, hot reload (150ms), Go background worker tasks, and a JSON Lines IPC socket.

## When to Activate

- Writing or refactoring LugoTUI Lua applications (`app.lua`)
- Implementing keyboard navigation (2D spatial, directional arrows, Vim keys, local keymaps)
- Integrating LugoTUI into Go programs (`github.com/VintlGvard/LugoTUI/tui`)
- Implementing background tasks, event handling, or model synchronization
- Automating, testing, or inspecting running TUIs via headless CLI or Unix domain socket (IPC)
- Debugging layout issues, schema errors, hot-reload state resets, or concurrency bugs

---

## 1. Execution Modes

| Mode | Entry Point | Best For |
| --- | --- | --- |
| **Go Library** | `tui.RunOptions(opts)` | Apps requiring Go business logic, database/HTTP access, custom background workers, or system APIs. |
| **Standalone CLI** | `lugotui --app app.lua` | Pure Lua UIs, rapid prototyping, scripts, or apps controlled externally via IPC. |
| **IPC Controller** | Unix Domain Socket | Headless automation, testing, or external agent control of an active interface. |

---

## 2. Lua Application Structure (`app.lua`)

A LugoTUI application returns a table specifying `api_version`, `view`, `actions`, and optional `keymap` / `spatial_nav`:

```lua
local ui = require('ui')

return {
    api_version = 1,
    spatial_nav = true, -- Enable 2D spatial arrow navigation globally

    -- Factory function: called on initial load and on hot reload (F5)
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
                    ui.input {
                        id = "search_input",
                        bind = "query",
                        placeholder = "Search packages...",
                        nav_down = "pkg_table",
                    },
                    ui.button {
                        id = "btn_submit",
                        text = "Submit",
                        on_click = "submit_query",
                        nav_down = "pkg_table",
                    },
                },
                ui.table {
                    id = "pkg_table",
                    columns = {
                        { title = "Package", width = 25 },
                        { title = "Version", width = 12 },
                        { title = "Status", width = 15 },
                    },
                    rows = {
                        { "neovim", "0.10.0", "installed" },
                        { "ripgrep", "14.1.0", "available" },
                    },
                    bind = "selected_pkg",
                    on_select = "on_pkg_select",
                    nav_up = "search_input",
                    -- Node-scoped keymap (does not steal keys from search_input):
                    keymap = {
                        ["Space"] = "toggle_pkg",
                        ["i"]     = "install_pkg",
                    },
                },
            },
        }
    end,

    -- Event handlers called when user interacts with UI or host dispatches
    actions = {
        submit_query = function(event)
            local query = ui.get("query")
            if not query or query == "" then
                ui.set("status_msg", "Please enter a query.")
                return
            end
            ui.set("status_msg", "Searching: " .. query)
            host.dispatch("start_search", { query = query })
        end,

        on_pkg_select = function(event)
            -- event.index is 1-based row index
            ui.set("status_msg", "Selected row: " .. tostring(event.index))
        end,

        toggle_pkg = function(event)
            ui.set("status_msg", "Toggled package at " .. tostring(event.id))
        end,

        install_pkg = function(event)
            ui.set("status_msg", "Installing selected package...")
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

    -- Global key shortcuts (active across entire app)
    keymap = {
        ["ctrl+s"] = "submit_query",
    },
}
```

---

## 3. Core Lua Operations & Focus APIs

### Model & Tree Operations
- `ui.get(key)`: Read value from explicit model.
- `ui.set(key, value)`: Write value to explicit model (preserves across hot reload).
- `ui.patch(id, props)`: Update specific properties of an existing node without full rebuild.
- `ui.patch({ [id1] = props1, [id2] = props2 })` / `ui.patch_all(table)`: Batch update multiple nodes in a single atomic commit.
- `ui.replace(id, new_node)`: Replace a node and its children.
- `host.dispatch(action, payload, [opts])`: Dispatch event to Go host (`OnEvent`). Pass `{ debounce = true }` in `opts` or payload to deduplicate and replace pending events by action.

### Focus & Navigation APIs
- `ui.focus(id)`: Request keyboard focus for node `id`.
- `ui.focused()`: Return `id` of currently focused node, or `nil`.
- `ui.focus_direction(dir)`: Move focus in direction `'up'`, `'down'`, `'left'`, or `'right'`. Uses explicit target or 2D edge distance projection. Returns target node `id` or `nil`.
- `ui.focus_next()`: Advance focus forward in document order (Tab equivalent). Returns target `id` or `nil`.
- `ui.focus_prev()`: Retreat focus backward in document order (Shift+Tab equivalent). Returns target `id` or `nil`.

---

## 4. 2D Spatial & Directional Navigation

LugoTUI supports 3-tier navigation:

### 1. Explicit Directional Navigation
Hardwire navigation routes between complex panels:
- `nav_up = "target_id"`, `nav_down = "target_id"`, `nav_left = "target_id"`, `nav_right = "target_id"`
- Or table syntax: `nav = { up = "id", down = "id", left = "id", right = "id" }`
- To disable a direction: `nav_up = "none"` or `nav_up = "false"`

### 2. Automatic & Spatial 2D Fallback
When explicit target is absent, the engine calculates the closest focusable node using 2D edge projection:
- **Buttons (`ui.button`)**: Automatically navigate in all 4 directions on arrow keys.
- **Opt-in (`spatial_nav = true`)**: Set globally in root spec (`return { spatial_nav = true, ... }`) or per-node (`ui.input { spatial_nav = true, ... }`).
  - **Inputs (`ui.input`)**: `Up` / `Down` arrows jump to adjacent widgets; `Left` / `Right` arrows remain cursor navigation.
  - **Collections (`ui.table`, `ui.list`, `ui.scroll`, `ui.viewport`, `ui.tree`)**: `Left` / `Right` arrows jump to adjacent panels; `Up` / `Down` arrows remain item/row navigation.

---

## 5. Widget Keymaps & Keystroke Interception

Avoid global key conflicts by scoping keybindings to focused nodes:

### Node-Scoped `keymap`
```lua
ui.table {
    id = "my_table",
    keymap = {
        ["Space"]  = "toggle_item",
        ["i"]      = "install_item",
        ["d"]      = "delete_item",
        ["ctrl+f"] = "filter_items",
    },
}
```
Only fires when `my_table` has focus. Does not interfere with typing in an `input` widget.

### Raw Keystroke Interception (`on_key`)
Intercept all keystrokes on a focused node:
```lua
ui.box {
    id = "game_canvas",
    on_key = "handle_game_input",
}
```
Event payload passed to action:
```lua
actions = {
    handle_game_input = function(event)
        -- event.id    : string ("game_canvas")
        -- event.key   : string ("Up", "Enter", "ctrl+a", etc.)
        -- event.rune  : string (character rune)
        -- event.ctrl  : boolean
        -- event.alt   : boolean
        -- event.shift : boolean
        -- event.focus : string
    end,
}
```

---

## 6. Widget Catalog & Schemas

### Layout Containers
- **`ui.column { ... }`**: Vertical container (`direction = "column"`).
- **`ui.row { ... }`**: Horizontal container (`direction = "row"`).
- **`ui.box { ... }`**: Generic flex container.
- **`ui.panel { title = "Title", border = "rounded", ... }`**: Styled container with border and title.
  Borders: `"single"`, `"double"`, `"rounded"`, `"none"`.
- **`ui.scroll { ... }`**: Scrollable container for overflowing children.
- **`ui.overlay { base = node, overlay = node }`**: Stacks floating overlay over base layout.

### Flexbox Properties
- `grow = 1`, `shrink = 0`, `basis = 10`
- `width = 20` | `"50%"` | `"auto"`, `height = 10` | `"auto"`
- `min_width`, `max_width`, `min_height`, `max_height`
- `padding = 1`, `margin = 1`, `gap = 1`
- `justify = "start"` | `"center"` | `"end"` | `"space-between"` | `"space-around"`
- `align = "start"` | `"center"` | `"end"` | `"stretch"`
- `align_self = "start"` | `"center"` | `"end"` | `"stretch"`
- `position = "absolute"`, `top = 0`, `bottom = 0`, `left = 0`, `right = 0`, `z_index = 10`

### Interactive Widgets
- **`ui.input`**: Single-line text input.
  `id` (required for state persistence), `bind` (model key), `placeholder`, `prompt`, `char_limit`, `echo = "password"|"none"`, `on_change`, `on_submit`, `nav_*`, `spatial_nav`, `keymap`, `on_key`.
- **`ui.textarea`**: Multi-line text editor.
  `id`, `bind`, `placeholder`, `line_numbers = true`, `nav_*`, `keymap`, `on_key`.
- **`ui.button`**: Clickable button.
  `id`, `text`, `on_click`, `nav_*`, `keymap`, `on_key`. (Intrinsic width accounts for brackets `[ text ]`, flex shrink defaults to 0).

### Data & Collection Widgets
- **`ui.table`**: Data table with keyboard selection.
  *CRITICAL SCHEMA RULE:* `columns` MUST be a list of objects with `title` and `width`:
  ```lua
  ui.table {
      id = "items_table",
      columns = {
          { title = "ID", width = 6, align = "left" },
          { title = "Name", width = 20, align = "left" },
          { title = "Price", width = 10, align = "right" },
      },
      rows = {
          { "1", "Widget A", "$10" },
          { "2", { text = "Widget B", fg = "green", bold = true, align = "right" }, "$20" },
      },
      bind = "selected_row",
      on_select = "on_row_select",
      on_change = "on_row_change", -- { id, index, value }
  }
  ```
- **`ui.list`**: List with optional fuzzy filtering.
  `id`, `items = { "Alpha", "Beta" }`, `bind = "selected_idx"`, `on_select = "action"`, `on_change = "action"`, `filterable = true` (press `/` to filter; query updates trigger `on_change` with `{ id, query, value }`, row selection triggers `{ id, index, value }`).
- **`ui.tree`**: Hierarchical tree view.
  `id`, `items = { { id = "root", text = "Root", expanded = true, children = { ... } } }`, `on_select`, `on_change`.
- **`ui.viewport`**: Scrollable read-only text view with Vim keys (`j`/`k`, `d`/`u`, `g`/`G`).
  `id`, `text = "..."`, `soft_wrap = true`, `show_line_numbers = true`.

### Indicators & Motion
- **`ui.text`**: Text display.
  `text`, `bind`, `wrap = "none"`, `spans = { { text = "Bold Red", fg = "red", bold = true } }`.
- **`ui.spinner`**: Animated spinner.
  `preset = "dots"` (options: `dots`, `line`, `dot`, `jump`, `pulse`, `globe`, `moon`, `ellipsis`), `interval = 80`.
- **`ui.progress`**: Progress bar.
  `percent = 0.75` (0.0 to 1.0), `gradient = { "#0088ff", "#00ff88" }`.
- **`ui.paginator`**: Pagination control.
  `page = 0` (0-based), `per_page = 10`, `total = 50`, `mode = "numeric"|"dots"`.

### Modals & Dialogs
- Add `modal = true` to any panel/box. Focus is trapped inside; pressing `Escape` closes the modal.

---

## 7. State Management & Hot Reload Rules

LugoTUI enforces a clean separation between transient UI closures and persistent model state:

1. **Explicit Model (`ui.get` / `ui.set`)**:
   Always store business state (selected IDs, inputs, checkboxes, fetched lists) in the model. Model state survives hot reloads (F5).
2. **Never Store State in Lua Module-Level Globals**:
   Lua files are re-evaluated completely on reload. Module variables reset to default values.
3. **Always Provide Stable `id`s**:
   `ui.input`, `ui.textarea`, `ui.table`, `ui.list` track scroll offsets, cursor positions, and selections via `id`. Without `id`, state resets on every render.
4. **Action Transactions**:
   Actions execute atomically. Model updates, focus shifts, and queued host dispatches commit together. Errors roll back changes cleanly.

---

## 8. Go Host Integration & Concurrency

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

				// Automatically cancel previous pending task by name and delay 200ms:
				_, err := rt.StartTaskDebounced("search", 200*time.Millisecond, "search_complete", func(ctx context.Context) (map[string]any, error) {
					return map[string]any{"result": "Matched: " + query}, nil
				})
				if err != nil {
					fmt.Printf("start task error: %v\n", err)
				}
			}
		},
	}

	if err := tui.RunOptions(opts); err != nil {
		panic(err)
	}
}
```

### Go Host Rules
- **Rule 1 (UI Thread Only)**: `*tui.Runtime` methods (`StartTask`, `CancelTask`, `Model`, `SetFocus`, `FocusDirection`, `FocusNext`, `FocusPrev`, `Close`) MUST only be called from `OnEvent` or the runtime owner goroutine.
- **Rule 2 (No Runtime Pointers in Workers)**: Background worker functions run on separate goroutines. NEVER pass `*tui.Runtime` into a worker or touch the Lua state from a worker.
- **Rule 3 (Data Transfer via Payload)**: Pass data into tasks by value. Return results via `map[string]any`. LugoTUI safely dispatches results back to the Lua action on the main UI loop.
- **Rule 4 (Handle Cancellation)**: Workers must check `<-ctx.Done()` to clean up resources when canceled or when the app exits.

---

## 9. Verification & Headless Testing

Agents should always verify changes headlessly before presenting:

```sh
# 1. Validate syntax, schema, layout, and render (exit code 0 = pass)
lugotui --app app.lua --check

# 2. Inspect layout tree, clip rectangles, and state as JSON
lugotui --app app.lua --snapshot

# 3. Run Go test suite with race detector
go test -race ./...
```

### Headless Go Unit Test Example
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
	if rt.Focus() == "" {
		t.Errorf("expected initial focus")
	}
}
```

---

## 10. Socket IPC Protocol (JSON Lines)

Enable socket IPC:
```sh
lugotui --app app.lua --socket /tmp/lugotui.sock
```

Communicate over the Unix domain socket:
- **Snapshot**: `{"cmd":"snapshot","id":1}`
- **Apply Action**:
  ```json
  {
    "cmd": "apply",
    "id": 2,
    "revision": 0,
    "actions": [
      {"type": "input", "id": "search_input", "value": "term"},
      {"type": "call", "action": "submit_query"}
    ]
  }
  ```
- **Subscribe to Events**: `{"cmd":"events","id":3}`

---

## 11. Common Pitfalls & Diagnostics

| Problem | Cause | Solution |
| --- | --- | --- |
| `columns must be array of objects` | `ui.table` defined with string array `columns = { "A", "B" }`. | Use `{ { title = "A", width = 10 }, { title = "B", width = 15 } }`. |
| Arrow keys trapped in table / input | Widget consumes arrows for internal navigation. | Use explicit `nav_up`/`nav_down` or enable `spatial_nav = true`. |
| Hotkey triggers while typing in input | Global `keymap` caught letter or `Space`. | Move shortcut to widget-specific `keymap = { ... }` on the table or button. |
| Input cursor/scroll resets on update | Widget lacks `id` or has changing `id`. | Assign a fixed, unique `id = "..."` string. |
| Button text truncated with brackets `[ ... ]` | Explicit `width` too small or container flex constraints. | Buttons include 4 cells for brackets `[  ]`. Use `shrink = 0` (default) or increase `width`. |
| Data disappears on F5 hot reload | State stored in local Lua variables. | Store in model via `ui.set("key", val)` and read with `ui.get("key")`. |
| Panic: concurrent map read/write | Worker goroutine touched `*tui.Runtime` or shared map. | Keep workers isolated; return data through the worker's `map[string]any` payload. |
