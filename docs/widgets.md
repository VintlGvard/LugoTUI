# Widgets, Pure-Go DSL & Lua Contract

This document provides a complete reference for the LugoTUI declarative widget system, the Pure-Go View Builder DSL, the Lua application contract, lifecycle rules, Flexbox layout properties, and interaction behaviors.

---

## 1. Pure-Go View Builder DSL

LugoTUI provides a type-safe, fluent Go API (`github.com/VintlGvard/LugoTUI` or `github.com/VintlGvard/LugoTUI/tui`) for building declarative widget trees.

### Constructors
- **Containers**:
  - `Column(children ...*Node) *Node` — Flex container with vertical column direction.
  - `Row(children ...*Node) *Node` — Flex container with horizontal row direction.
  - `Box(children ...*Node) *Node` — Generic flex container.
  - `Panel(title string, children ...*Node) *Node` — Styled container with title, border, and padding.
  - `Scroll(children ...*Node) *Node` — Scrollable container.
  - `Overlay(children ...*Node) *Node` — Floating container placed above standard layout flow.
  - `Modal(id, title string, children ...*Node) *Node` — Modal dialog (`modal: true`, `border: true`) that traps focus and auto-closes on `Escape`.
  - `Spacer() *Node` — Flex space consumer (`Box().Grow(1)`).
- **Primitives & Indicators**:
  - `Text(text string) *Node` — Static or bound text display.
  - `Textf(format string, args ...any) *Node` — Formatted text display.
  - `Button(label string, onClick ...string) *Node` — Clickable button.
  - `Spinner(preset ...string) *Node` — Animated spinner (12 presets).
  - `Progress(percent float64) *Node` — Progress bar (`0.0` .. `1.0`).
  - `Paginator(page, perPage, total int) *Node` — Pagination control.
  - `Timer(durationMs int, onTimeout ...string) *Node` — Countdown timer.
  - `Stopwatch() *Node` — Elapsed time counter.
  - `Help(bindings ...HelpBinding) *Node` — Interactive keybinding bar.
  - `Cursor() *Node` — Custom blinking or steady terminal cursor.
- **Editors**:
  - `Input(bindKey string, placeholder ...string) *Node` — Single-line text input with selection and undo.
  - `Textarea(bindKey string, placeholder ...string) *Node` — Multi-line text editor with line numbers and grapheme navigation.
- **Collections**:
  - `List(items ...string) *Node` — List view with optional live fuzzy filtering (`.Filterable(true)`).
  - `Table(columns []TableColumn, rows [][]string) *Node` — Data table with columns and cell alignment.
  - `Tree(items ...TreeItem) *Node` — Hierarchical tree widget with collapsible branches.
  - `Viewport(text string, highlights ...HighlightSpan) *Node` — Scrollable text view with Vim navigation.
  - `Filepicker(entries ...FileEntry) *Node` — Host-supplied filesystem picker.
- **Conditional & List Utilities**:
  - `If(cond bool, node *Node) *Node` — Renders `node` if true, nil otherwise (containers automatically ignore nil children).
  - `IfElse(cond bool, thenNode, elseNode *Node) *Node`
  - `Map[T any](items []T, fn func(i int, item T) *Node) []*Node`

### Fluent Chainable `*Node` Methods
- **Identity & Binding**: `.SetID(id)`, `.Bind(key)`, `.Prop(key, val)`, `.Add(children...)`, `.When(cond)`
- **Dimensions & Flex**: `.Width(w)`, `.Height(h)`, `.Size(w, h)`, `.MinWidth(w)`, `.MaxWidth(w)`, `.MinHeight(h)`, `.MaxHeight(h)`, `.Grow(g)`, `.Shrink(s)`, `.Flex(g)`, `.Padding(p)`, `.Margin(m)`, `.Gap(g)`, `.Align(a)`, `.Justify(j)`, `.ZIndex(z)`, `.Absolute(left, top)`
- **Appearance**: `.Text(s)`, `.Title(s)`, `.Placeholder(s)`, `.Border(bool)`, `.Bordered()`, `.FG(color)`, `.BG(color)`, `.Color(fg, bg)`, `.Bold(bool)`, `.Italic(bool)`, `.Underline(bool)`, `.Wrap(bool)`
- **Behavior & State**: `.Hidden(bool)`, `.Disabled(bool)`, `.Modal(bool)`, `.Masked(bool)`, `.MaxLength(n)`, `.Filterable(bool)`, `.ShowHeader(bool)`, `.ShowLineNumbers(bool)`, `.SoftWrap(bool)`, `.PageSize(n)`
- **Zero-Boilerplate Inline Closures**:
  - `.OnClickDo(func())` / `.OnClickErr(func() error)` — inline button click handler
  - `.OnChangeText(func(text string))` — live text input change handler
  - `.OnSubmitText(func(text string))` — input submit (Enter) handler
  - `.OnSelectItem(func(index int, value any))` — collection activation handler
  - `.KeyDo(chord, func())` / `.KeyErr(chord, func() error)` — shortcut closure (bubbles up container hierarchy)

---

## 2. Lua Application Contract

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
- `ui.focused()`: Return the ID of the currently focused node, or `nil`.
- `ui.focus_direction(dir)`: Move focus in direction `'up'`, `'down'`, `'left'`, or `'right'` using explicit and spatial navigation. Returns target ID or `nil`.
- `ui.focus_next()`: Advance focus forward in document order (equivalent to Tab). Returns target ID or `nil`.
- `ui.focus_prev()`: Retreat focus backward in document order (equivalent to Shift+Tab). Returns target ID or `nil`.
- `host.dispatch(action, payload)`: Queue a host event to Go; no external side effects during candidate initialization.

### Action Transactions & Reload Semantics

- **Transactional consistency:** Action transactions cover UI modifications, model state, queued host events, and focus only. These commit together atomically or roll back cleanly on failure.
- **State migration across reload:** Lua globals and closure upvalues are not rolled back and are not migrated on reload. Keep persistent application state in the explicit model (`ui.get` / `ui.set`).
- **Input preservation:** Editor keystrokes remain valid user input even when an `on_change` callback fails. Stable `(id, type)` pairs preserve editor state (cursor, scroll, text) across reloads.
- **Data binding:** Bind editor strings with `bind`; bind list arrays with `source`.
- **Event signatures:** Callbacks receive events exposing `id`, `value`, and one-based `index` where applicable.

---

## 3. Flexbox Layout System

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

## 4. Native Widgets Catalog

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
- **`ui.table`:** Data table with `columns = {{ title = 'Name', width = 20, align = 'left'|'center'|'right' }}` and `rows = {{ 'Example' }}` or cell objects `{{ { text = '42', align = 'right', fg = 'green', bg = 'black', bold = true } }}`. Supports horizontal scrolling, Vim keys (`j` / `k` / `g` / `G`), `on_select` (row activation), and `on_change` (row selection change).
- **`ui.tree`:** Hierarchical tree widget with nested `{ id, text, children, expanded }` items, Vim/arrow key navigation, `on_select`, and `on_change`.
- **`ui.list`:** List view with optional fuzzy filtering (`filterable = true`, activate with `/`), item descriptions, Vim keys, pagination (`page_size`, `height`), `on_select`, and `on_change`.
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

## 5. Interaction, Focus & Navigation Standards

- **ID stability:** Stateful widgets require stable `id` properties across reloads.
- **Indexing convention:** Collections use **1-based** `index` in selection callbacks; paginator `page` is **0-based**.
- **Collection callbacks:**
  - `on_select`: Triggered when an item or row is activated (e.g. `Enter` key or direct mouse click). Payload: `{ id = "<node_id>", index = <1-based index>, value = <item/row value> }`.
  - `on_change`: Triggered when active item/row selection changes via keyboard arrows, Vim keys (`j`/`k`/`g`/`G`), or mouse click/wheel. Payload: `{ id = "<node_id>", index = <1-based index>, value = <item/row value> }`.
  - In `ui.list` with `filterable = true`, filter query changes trigger `on_change` with `{ id = "<node_id>", query = "<query>", value = "<query>" }` (distinguished by the presence of `query` rather than `index`).
- **Time advancement:** Embedders must call `Tick(time.Time)` from the runtime owner loop while `NeedsTick()` is true. The managed runner handles ticks automatically; `Draw` alone does not advance time.
- **Directional Navigation:**
  - `nav_up`, `nav_down`, `nav_left`, `nav_right`: Explicit target node `id` for arrow key navigation (or `'none'` to disable navigation in that direction).
  - `nav = { up = '...', down = '...', left = '...', right = '...' }`: Structured table alternative.
- **2D Spatial Navigation:**
  - Buttons automatically navigate to adjacent widgets using 2D edge distance projection on all arrow keys.
  - `spatial_nav = true`: Opt-in on any node or globally in the root spec table (`return { spatial_nav = true, ... }`).
  - On `ui.input`: `spatial_nav = true` allows `Up` and `Down` arrow keys to jump to vertically adjacent widgets while preserving cursor movement on `Left` and `Right`.
  - On `ui.table` and `ui.list`: `spatial_nav = true` allows `Left` and `Right` arrow keys to jump to adjacent panels while preserving row selection on `Up` and `Down`.
- **Node-level Keymaps & Interception:**
  - `keymap = { ['Space'] = 'action', ['i'] = 'install' }`: Widget-specific keyboard shortcuts active while the widget has focus.
  - `on_key = 'action'`: Intercepts keystrokes on the focused node, passing `{ id, key, rune, ctrl, alt, shift, focus }` in the event payload.
- **Hierarchical Key Event Bubbling:**
  - When a keystroke is not consumed by the focused widget, it bubbles upward through ancestor containers (`Node` -> `Parent` -> `Grandparent` -> `Root/Modal`).
  - Keymaps attached to parent containers (`Column(...).KeyDo("ctrl+s", fn)` or `ui.column{ keymap = { ... } }`) catch unhandled hotkeys from their children.
- **Default Keybindings:**
  - `Tab` / `Shift+Tab`: Move focus forward / backward.
  - `Ctrl+P`: Command palette.
  - `F2`: Scratchpad toggle.
  - `F5`: Manual reload trigger.
  - `Ctrl+Q` / `Ctrl+C`: Quit application.
  - Mouse: Click to focus/select, wheel scroll, drag selection (requires terminal mouse reporting).
