# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
