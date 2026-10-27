# Widgets & Lua Contract

This document provides a complete reference for the LugoTUI declarative widget system, the Lua application contract, lifecycle rules, Flexbox layout properties, and interaction behaviors.

---

## 1. Lua Application Contract

A LugoTUI application in Lua returns a root table with the following structure:

```lua
return {
  api_version = 1,
  view = tree_or_factory_function,
  actions = { ... },
  keymap = { ... }
}
```

The embedded module is loaded via `local ui = require('ui')`.

### View Factory vs. Reactive Rendering

**`view` is an initial-tree factory, not a reactive render function.**
- If `view` is a function, it runs **once** when `Load` or `Reload` reaches view construction (including attempts that later fail validation).
- A table is also accepted directly as `view`.
- `ui.set` does **not** re-invoke `view`; changing model data alone does not rebuild node properties or child hierarchies.

### Explicit Update Paths

To update the interface, use one of three explicit paths:

1. **`bind` / `source`:**
   - `bind`: displays model strings and synchronizes editor values bidirectionally.
   - `source`: reads model arrays for list widgets (`ui.list`).
2. **`ui.patch(id, props)`:** stage property updates on an existing node.
3. **`ui.replace(id, tree)`:** stage full subtree replacement when the structure changes.

### Complete Example with Component Composition

Lua functions can be used for component composition without any custom widget registration. Components return standard built-in node tables:

```lua
local ui = require('ui')

local function labeled_input(id, label, key)
  return ui.column {
    ui.text { text = label, height = 1 },
    ui.input { id = id, bind = key, height = 1 }
  }
end

return {
  api_version = 1,
  view = function()
    return ui.column {
      labeled_input('name_input', 'Name', 'name'),
      ui.text { id = 'preview', bind = 'name', height = 1 },
      ui.text { id = 'status', text = 'Idle', height = 1 },
      ui.box { id = 'details', height = 1 },
      ui.button { id = 'save', text = 'Save', on_click = 'save', height = 1 }
    }
  end,
  actions = {
    save = function()
      local name = ui.get('name') or ''
      ui.set('name', string.upper(name))
      ui.patch('status', { text = 'Saved' })
      ui.replace('details', ui.text { id = 'details', text = name, height = 1 })
    end
  }
}
```

### Core Lua Operations

- `ui.patch(id, props)`: Stage property updates for the node with identifier `id`.
- `ui.replace(id, tree)`: Stage subtree replacement for the node with identifier `id`.
- `ui.get(key)`: Read a value from the explicit model.
- `ui.set(key, value)`: Write a value to the explicit model.
- `ui.focus(id)`: Request keyboard focus for node `id`.
- `host.dispatch(action, payload)`: Queue a host event to Go; no external side effects during candidate initialization.

### Action Transactions & Reload Semantics

- **Transactional consistency:** Action transactions cover UI modifications, model state, queued host events, and focus only. These commit together atomically or roll back cleanly on failure.
- **State migration across reload:** Lua globals and closure upvalues are not rolled back and are not migrated on reload. Keep persistent application state in the explicit model (`ui.get` / `ui.set`).
- **Input preservation:** Editor keystrokes remain valid user input even when an `on_change` callback fails. Stable `(id, type)` pairs preserve editor state (cursor, scroll, text) across reloads.
- **Data binding:** Bind editor strings with `bind`; bind list arrays with `source`.
- **Event signatures:** Callbacks receive events exposing `id`, `value`, and one-based `index` where applicable.

---

## 2. Flexbox Layout System

LugoTUI implements a subset of Flexbox optimized for terminal cell grids:

- **Dimensions:** `width` and `height` accept cell counts (integers), percentage strings (e.g. `'50%'`), or `'auto'`.
- **Flex distribution:** `grow`, `shrink`, `basis` (numeric), `min_width`, `max_width`, `min_height`, `max_height`.
- **Alignment:**
  - `direction`: `'row'` or `'column'`.
  - `justify`: `'start'`, `'center'`, `'end'`, `'space-between'`, `'space-around'`.
  - `align`: `'start'`, `'center'`, `'end'`, `'stretch'`.
  - `align_self`: override container alignment for individual items (`'start'`, `'center'`, `'end'`, `'stretch'`).
- **Spacing:** Uniform `padding`, `margin`, `gap` (in terminal cells).
- **Positioning:** `position = 'absolute'`, with offsets (`top`, `bottom`, `left`, `right`) and `z_index` stacking.
- **Sizing model:** Border-box sizing. Flex-wrap is not supported.
- **Text wrapping:** `wrap = 'none'` disables word wrapping (soft-wrap by default).
- **Rich text styling:** `spans = {{ text = '...', fg = 'red', bold = true }}` supplies rich text spans.
- **Modals:** `modal = true` traps focus within the container; pressing `Escape` hides the modal.

---

## 3. Native Widgets Catalog

Widgets use LugoTUI nodes, Lua actions, and retained state. No secondary event loop or external UI framework is involved.

### Primitives & Containers
- `ui.box`: Generic flex container with optional borders (`border = 'single' | 'double' | 'rounded' | 'none'`), `title`, and styling.
- `ui.row`: Flex container with `direction = 'row'`.
- `ui.column`: Flex container with `direction = 'column'`.
- `ui.panel`: Box styled with title and container background.
- `ui.text`: Text display supporting `bind`, `spans`, alignment, and `wrap = 'none'`.
- `ui.button`: Clickable button with `text`, `on_click` action name, and focus handling.
- `ui.scroll`: Scrollable container.
- `ui.overlay`: Floating container placed above standard layout flow.

### Collections
- **`ui.table`:** Data table with `columns = {{ title = 'Name', width = 20 }}` and `rows = {{ 'Example' }}`. Supports horizontal scrolling and Vim keys (`j` / `k` / `g` / `G`).
- **`ui.tree`:** Hierarchical tree widget with nested `{ id, text, children, expanded }` items and Vim/arrow key navigation.
- **`ui.list`:** List view with optional fuzzy filtering (`filterable = true`, activate with `/`), item descriptions, Vim keys, and pagination (`page_size`, `height`).
- **`ui.viewport`:** Scrollable text view with Vim bindings (`j` / `k`, `d` / `u`, `g` / `G`, `h` / `l`). Supports `soft_wrap` and `show_line_numbers`.

### Motion & Indicators
- **`ui.cursor`:** Configurable blinking, static, or hidden cursor modes with customizable blink rates.
- **`ui.spinner`:** Animated spinner supporting 12 built-in presets (`line`, `dot`, `mini_dot`, `jump`, `pulse`, `points`, `globe`, `moon`, `monkey`, `meter`, `hamburger`, `ellipsis`) or custom `frames`, with `interval` in milliseconds.
- **`ui.progress`:** Progress bar with `percent` between `0` and `1`, optional animation, `percent_format`, and a two-color `gradient = { 'from_color', 'to_color' }` array.
- **`ui.paginator`:** Pagination control with zero-based `page`, `per_page`, `total`, display modes (`mode = 'numeric'` or `'dots'`), and Vim keys (`h` / `l`).

### Time Widgets
- **`ui.timer`:** Countdown timer counting down from `duration` in milliseconds, invoking `on_timeout` once.
- **`ui.stopwatch`:** Elapsed time counter counting up.
- *Common timer features:* Both support `running`, numeric `reset_key`, Space to pause/resume, and `r` to reset.

### Editors & Inputs
- **`ui.input` / `ui.textinput`:** Single-line text editor supporting `prompt`, `placeholder`, grapheme-based `char_limit`, suggestions, `cursor_mode`, and `validation_pattern`. Validation failures report an `error` string in callbacks without silently rejecting edits. `echo = 'password'` masks text; `echo = 'none'` hides text.
- **`ui.textarea`:** Multi-line text editor with `line_numbers`, full grapheme navigation, and multiline editing.

### Keybindings & Help
- **`ui.help`:** Interactive help overlay displaying configured key bindings: `keys = { ui.key { keys = { 'x' }, help = 'Example', action = 'example' } }`. Supports expanded help (`full = true`, toggled with `?`). Disabled bindings do not execute.

### Filesystem Controls
- **`ui.filepicker`:** Renders host-supplied filesystem entries passed via `items` or `source`. Displays human-readable file sizes (`show_size`) and permission bits (`show_permissions`). Directory navigation triggers `on_navigate`; Lua code receives no direct filesystem access.

---

## 4. Interaction, Focus & Navigation Standards

- **ID stability:** Stateful widgets require stable `id` properties across reloads.
- **Indexing convention:** Collections use **1-based** `index` in selection callbacks; paginator `page` is **0-based**.
- **Time advancement:** Embedders must call `Tick(time.Time)` from the runtime owner loop while `NeedsTick()` is true. The managed runner handles ticks automatically; `Draw` alone does not advance time.
- **Default Keybindings:**
  - `Tab` / `Shift+Tab`: Move focus forward / backward.
  - `Ctrl+P`: Command palette.
  - `F2`: Scratchpad toggle.
  - `F5`: Manual reload trigger.
  - `Ctrl+Q` / `Ctrl+C`: Quit application.
  - Mouse: Click to focus/select, wheel scroll, drag selection (requires terminal mouse reporting).
