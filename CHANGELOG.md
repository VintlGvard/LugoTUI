# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
