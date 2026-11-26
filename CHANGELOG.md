# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.4] - 2026-11-26

### Added
- **Windows Named Pipe IPC**: Added `\\.\pipe\...` Named Pipe support via `golang.org/x/sys/windows` overlapped I/O in `internal/engine/transport_windows.go` alongside Unix domain sockets on `!windows` (`internal/engine/transport_unix.go`).

### Changed
- **Streaming Control Coalescing**: Drained queued `ControlMsg` batches and coalesced high-frequency `ControlSet` and `ControlPatch` redraws to the configured `FPS` frame interval (`16 ms` default) in `internal/runner/run.go`.

## [0.2.3] - 2026-11-19

### Added
- **Theme Presets & Root Facade Parity**: Re-exported `TokyoNightTheme`, `CatppuccinTheme`, `DraculaTheme`, and `NordTheme` in `tui` and `lugotui`, alongside `ReservedKeys`, `NodeJSON`, `ProtocolError`, `ErrorCode`, `Demo`, and `Code*` constants in root package `lugotui`.
- **Screen Cache Invalidation**: Added `Runtime.InvalidateScreen()` to force a clean redraw after terminal mode switches (`__alt_screen`) and resize signals.
- **Extended Inline Action Signatures & Shortcut Modifiers**: Supported `func()`, `func(any) error`, `func(string)`, and `func(string) error` in `SetInlineAction` with upfront validation, plus `Alt+<rune>` and `Alt+Shift+<rune>` in `keyMatches`.

### Fixed
- **Core Cloning, IDs & Tree Validation**: Deep-cloned `[]int`, `[]float64`, `float32`, and `Node.Props` in `CloneValue`/`CloneNode` (resetting `LinesCache`), bounded auto-generated IDs in `AutoAssignIDs` to `MaxIDBytes` (128 bytes), required `ID` on interactive/stateful widgets even when `disabled` or `hidden`, and validated builder properties (`backdrop`, `current_dir`, `path`, `close_on_esc`, `multi_select`, `show_icons`, `item_height`, `selected`, `[]float64` sides).
- **Layout, Border & Wide-Cell Rendering**: Unified border/padding insets (`nodeInsets`) and directional `margin` sides across `intrinsic`, `place`, `measuredHeight`, and `contentRect`, improved `row` height estimation, rendered partial borders (`NodeBorderSides`), tracked wide CJK/emoji continuation cells in `cellBuffer`, rendered custom `cursor_char` on focused editors, and formatted non-filterable `list` items cleanly.
- **Transactional Rollback, Pure-Go Reload & Lifecycle**: Completed rollback of `Root`, `Nodes`, `State`, `Revision`, `layoutValid`, screen modes, and `GoActions` in `Action` (including full Lua metatable/key upvalue snapshots), enforced modal focus-trapping and `close_on_esc` in `Commit`/`Key`, handled `nil`/missing keys and errors cleanly in `syncBoundWidgets`/`Toggle`/`ui.set`, supported Pure-Go `Reload("")`, serialized empty `--socket` `events` as `[]`, checked trailing newlines in `frameReader.Read`, fixed `__alt_screen` dispatch without `OnEvent`, held `OSC52Clipboard` lock across writes, and pre-validated named/debounced background tasks before cancelling prior runs.
- **Editors, Collections, Scroll & Primitives**: Added `KeyEnd` and `ScrollExtent` clamping to `scroll`, validated model mutations and notified state observers on editor changes, used `textWidth` in `Table` column widths, fixed `filepicker` `".."` visibility and `Backspace` navigation, guarded `mouseCollection` against drag re-activation and out-of-bounds `list` clicks, clamped `table` horizontal scroll, implemented `tree` `open_char`/`close_char`/`branch_indent`, fixed `viewport` soft-wrap logical line numbers and horizontal scroll clipping, synchronized `timer`/`stopwatch` pause props across ticks with rollback on `on_timeout` error, invalidated `help` layout on `?`, and fixed `README.md`, `docs/sandbox.md`, `docs/widgets.md`, `demo.lua`, and `skills/lugotui/SKILL.md`.

### Security
- **Input & Resource DoS Hardening**: Enforced `core.EstimateBytes` size checks and typed field validation (`validateCollection`), validated `progress.percent_format` placeholders, bounded `compiledRegexCache` to 256 entries, rejected host/non-serializable options in `runner.Sandbox`, and propagated `elf.Open` errors in `isDynamicallyLinkedELF`.

## [0.2.2] - 2026-11-14

### Changed
- **Modular Text & Core Packages**: Extracted Unicode/grapheme text processing into `internal/text` (`TextWidth`, `Graphemes`, `Truncate`, `Lines`, `LimitedLines`, `WrapLines`) and core geometry, themes, borders, value limits, directional sides, and tree utilities into `internal/core`.
- **Hot-Path Allocation Reductions**: Eliminated intermediate buffer allocations and quadratic string concatenation in text wrapping (`WrapLines`), added zero-allocation generic slice handling in `PropSides`, and added early-exit byte-limit checks in `ValidateModel`, reducing `BenchmarkLayout` heap allocations from 154 to 39 allocs/op.

### Fixed
- **Wrapped Span Offsets & Atomic Cursor**: Bound rich-text `spans` rendering to exact source byte ranges (`[StartByte, EndByte)`), positioned the hardware cursor atomically before `screen.Show()`, and skipped `Show()` when both frame cells and cursor state are unchanged.
- **Pure Layout & Scroll Clamping**: Removed tree mutations during `Draw` by computing `ScrollExtent` inside `Layout` (`place`) and separating `applyScroll` clamping; added `label` fallback in text resolution for buttons and widgets.
- **Full Lua Transaction Upvalue Rollback**: Snapshotted and restored closure upvalues (including nested/cyclic tables) alongside `_G` globals when an action or commit fails.
- **Editor Key Capture & Overlay Wheel Isolation**: Routed active `input`, `textarea`, and collection filter typing ahead of global/scope `keymap` and `help` bindings while preserving non-editor shortcuts and widget hooks, and scoped mouse wheel scroll targets to active `z_index` subtrees so sibling overlays block wheel pass-through.
- **GoActions With Middlewares**: Preserved `Options.Actions` (`GoActions`) in `toRunnerOptions` when `WithMiddleware` is also configured.

## [0.2.1] - 2026-11-13

### Changed
- **Minimum Go Version**: Lowered module requirement in `go.mod` from Go `1.26.0` to `1.24.0` (aligning `golang.org/x/sys`, `golang.org/x/term`, and `golang.org/x/text` with `tcell/v2@v2.13.10`) and updated `docs/install.md`.
- **Documentation & CI Clarity**: Streamlined `README.md` with restored demo GIF and aligned `make cover` / CI descriptions with the race-enabled coverage report.

### Fixed
- **Option Map Isolation & Chaining**: Captured an isolated handler snapshot in `WithHandlers` so caller map mutations do not affect `OnEvent` dispatch, and chained `WithOnEvent` with prior event handlers regardless of option order.

### Added
- **Root Facade Test Coverage**: Added unit tests for `lugotui.Map`, `lugotui.NewOSC52Clipboard`, and `lugotui.NewScreenRenderer` in `lugotui_test.go`.

## [0.2.0] - 2026-11-11

### Added
- **Pure-Go View Builder DSL**: Fluent, type-safe constructors for all 22 widgets (`Column`, `Row`, `Box`, `Panel`, `Text`, `Textf`, `Button`, `Input`, `Textarea`, `List`, `Table`, `Tree`, `Viewport`, `Filepicker`, `Spinner`, `Progress`, `Paginator`, `Timer`, `Stopwatch`, `Help`, `Cursor`, `Custom`, `Modal`) enabling 100% Go applications without writing Lua.
- **Zero-Boilerplate Inline Action Closures**: Directly attach logic via `.OnClickDo`, `.OnClickErr`, `.OnChangeText`, `.OnSubmitText`, `.OnSelectItem`, `.KeyDo`, and `.KeyErr` without registering string action names.
- **Hierarchical Key Event Bubbling**: Hotkey shortcuts bubble up the container hierarchy from focused children through parent containers (`Column`, `Row`, `Box`, `Panel`, `Overlay`) to modal/root scope.
- **Typed State & Lifecycle Helpers**: Added `rt.String()`, `rt.Int()`, `rt.Float()`, `rt.Bool()`, `rt.AddInt()`, `rt.ToggleBool()`, `rt.Update()`, `rt.Quit()`, and sentinel `ErrQuit`.
- **Custom Canvas Widget**: Added `Custom(func(c Canvas))` and `.OnPaint()` allowing arbitrary terminal drawing with local coordinates (`SetCell`, `Print`, `Printf`, `Fill`, `LineH`, `LineV`, `IsFocused`).
- **Customizable Border Shapes & Glyphs**: Supported `"rounded"`, `"double"`, `"thick"`, `"ascii"`, `"single"`, and custom 8-rune `BorderGlyphs` alongside `BorderFG`, `BorderBG`, and `TitleFG`.
- **Button Styling & Affixes**: Added `.Plain()`, `.Chevrons()`, `.Parens()`, `.Filled()`, `.Prefix()`, and `.Suffix()`.
- **Collection Cursor & Selection Styling**: Added `.CursorPrefix()`, `.SelectedFG()`, `.SelectedBG()`, and `.SelectedBold()`.
- **Low-Level Key Interception**: Added `.OnRawKey(func(e *tcell.EventKey) bool)` for direct keyboard event consumption.
- **Root Package Facade**: Mirrored the entire API under `github.com/VintlGvard/LugoTUI` (`lugotui.go`).

### Changed
- **Double-Buffered `cellBuffer` Diff Renderer**: Replaced direct tcell screen writing with an in-memory diff matrix, reducing heap allocations from 26,563 down to 18 allocs/op and render latency to 297 µs.
- **Architecture Decomposition**: Split god-files into modular subsystems (`builder.go`, `hooks.go`, `primitives.go`, `primitive_tick.go`, `sandbox_linux.go`, `sandbox_other.go`, `signals_unix.go`, `signals_windows.go`).

### Fixed
- Fixed `cloneNode(nil)` nil-pointer dereference in `Runtime.Replace`.
- Replaced panic on unexpected internal data type in TUI facade with safe nil return.
- Fixed `Modal` type alignment to engine `panel` component.

## [0.1.6] - 2026-11-10

### Changed
- Reduced per-frame heap allocations by 37% (`42,319` to `26,563` allocs/op) and frame render latency from `2.34 ms` to `1.39 ms` by eliminating `uniseg` `g.Runes()` slice allocations, `strings.Repeat` border strings, and redundant container cell fills.
- Cached compiled `validation_pattern` regular expressions (`sync.Map`) and `tcell` normalized key names (`sync.OnceValue`).
- Switched `StartTaskDebounced` to timer-first scheduling (`time.AfterFunc`) before spawning worker goroutines.

### Fixed
- Full transactional rollback of user-defined Lua `_G` globals, deep-cloned `Runtime.Data`, and widget state (`input`, `textarea`, `list`, `table`, `tree`, `timer`, `stopwatch`) when `Action` or `Commit` fails.
- Persistent AI agent IPC socket connections no longer disconnect after 2 seconds of idle time between JSON Lines frames (`frameReader`).
- `Runtime.Patch` and `Runtime.Replace` now accept standard Go `int` and `int64` numeric properties alongside `float64`.
- Headless `FocusNext`/`FocusPrev` before initial `Draw` and staged `ui.focus_next`/`ui.focus_prev`/`ui.focus_direction` inside Lua actions.
- Cleared stale `diagnostic` error banners on subsequent successful user input and preserved hardware cursor visibility when not overlapped by the bottom diagnostic row.
- Fixed word-wrap edge cases with leading/trailing spaces in `LimitedLines`, `spans` newline offset tracking in `paintText`, and control/tab sanitization on `Paste`.
- Validated static ELF linkage before entering `bubblewrap` in `--sandbox` mode and enforced `gofmt` verification in GitHub Actions CI.

### Security
- Guarded Lua `string.rep` against OOM allocation bypass by enforcing the 1 MiB string limit (`maxText`) prior to allocation.

## [0.1.5] - 2026-11-09

### Fixed
- Trailing newline in `tui/layout_test.go` causing `gofmt` failure.
- Replace `print()` with `sys.stdout.write()` in `.github/scripts/release.py` for Q.E.D. zero-debug compliance.

### Added
- Automated `gofmt` verification check to `make test`.

## [0.1.4] - 2026-11-07

### Added
- `on_change` event for collection navigation (`table`, `list`, `tree`, `filepicker`).
- File name in Lua stack traces via `LoadNamed` and `ReloadNamed`.
- Table column alignment and styled cell objects with fast left-align path.
- Built-in debounced tasks (`StartTaskDebounced`) and `host.dispatch` debounce.
- Batch atomic `ui.patch` and `ui.patch_all`.
- Modal focus isolation blocking global keymap.

## [0.1.3] - 2026-10-31

### Added
- 2D spatial navigation and explicit directional nav (`nav_up`, `nav_down`, `nav_left`, `nav_right`, `nav={}`).
- Widget-scoped keymap and `on_key` event interceptor.
- Programmatic focus APIs (`ui.focused`, `ui.focus_direction`, `ui.focus_next`, `ui.focus_prev`).

## [0.1.2] - 2026-10-30

### Fixed
- CLI and release automation improvements.

## [0.1.1] - 2026-10-29

### Fixed
- Minor bug fixes and performance improvements.

## [0.1.0] - 2026-10-28

### Added
- Initial LugoTUI release: Lua-driven TUI engine, widgets, hot reload, and agent IPC.
