# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Validated static ELF linkage before entering `bubblewrap` in `--sandbox` mode and updated GitHub Actions workflow versions (`actions/checkout@v4`, `actions/setup-go@v5`, `govulncheck@latest`).

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
