# LugoTUI

**High-Performance Declarative TUI Engine for Go — Pure Go by Default, Lua Live Hot-Reload as a Superpower**

[![CI](https://github.com/VintlGvard/LugoTUI/actions/workflows/ci.yml/badge.svg)](https://github.com/VintlGvard/LugoTUI/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/VintlGvard/LugoTUI)](https://goreportcard.com/report/github.com/VintlGvard/LugoTUI)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](LICENSE)

LugoTUI is an embeddable Terminal UI library (`github.com/VintlGvard/LugoTUI` / `github.com/VintlGvard/LugoTUI/tui`) and standalone runner that gives you **100% Pure-Go declarative & reactive UIs** out of the box—plus **150ms Live Hot-Reload via Lua** whenever you want instant visual iteration or user-scriptable plugins.

* **Pure Go or Hybrid Go + Lua**: Build your entire TUI in type-safe Go (`QuickViewFunc`, `QuickView`, `Column`, `Row`, `Panel`, `Table`, `Input`), or move the view into `.lua` for instant hot-reload without changing a single line of your Go handlers.
* **Zero Boilerplate vs. Bubble Tea**: No `Init() / Update(tea.Msg) / View() string` ceremony, no manual `tea.WindowSizeMsg` math, and no manual child message routing. Flexbox layout, 2D spatial focus navigation, two-way `Bind(key)` state sync, and widgets (`table`, `tree`, `list`, `input`, `textarea`, `viewport`, `filepicker`, `modal`) work automatically.
* **Sub-Millisecond Cell-Diff Renderer**: Backbuffer cell-diffing (`cellBuffer`) + dirty-tracked Flexbox layout + cached grapheme/line measurements render a 300-node dashboard in **~460 µs with 12 heap allocations**.
* **AI-Agent & Headless Ready**: Inspect and control any running app over a Unix domain socket (`--socket`) or test headless in microseconds via `LoadView` / `LoadViewFunc`.

---

## Install

### As a Go Library
```sh
go get github.com/VintlGvard/LugoTUI@latest
```

### Standalone CLI Runner
```sh
go install github.com/VintlGvard/LugoTUI/cmd/lugotui@latest
```

---

## Quick Start

### 1. 100% Pure Go (Reactive `QuickViewFunc` — Zero Lua Required)

Write a reactive view function in pure Go. Use zero-boilerplate inline closures (`OnClickDo`, `OnChangeText`, `KeyDo`) or registered actions. When state changes, LugoTUI automatically reconciles the widget tree while preserving keyboard focus, cursor position, selection, and scroll offsets:

```go
package main

import (
	"log"

	lugo "github.com/VintlGvard/LugoTUI"
)

func main() {
	err := lugo.QuickViewFunc(func(rt *lugo.Runtime) *lugo.Node {
		return lugo.Column(
			lugo.Panel("Service Monitor",
				lugo.Textf("Filter: %s | Polls: %d", rt.String("query"), rt.Int("polls")).Bold(true).FG("cyan"),
				lugo.Input("query").Placeholder("Filter services..."),
				lugo.Row(
					lugo.Button("Refresh").OnClickDo(func() { rt.AddInt("polls", 1) }),
					lugo.Button("Quit").OnClickErr(rt.Quit),
				).Gap(1),
			).Grow(1).Padding(1).Gap(1),
		).Padding(1).KeyDo("ctrl+q", func() { rt.Quit() })
	}, nil, lugo.WithState("polls", 1))
	if err != nil {
		log.Fatal(err)
	}
}
```

### 2. 100% Pure Go (Retained `QuickView` + Two-Way `Bind`)

Build the widget tree once and mutate bound state directly (`rt.Set("status", ...)`):

```go
view := lugo.Column(
	lugo.Text("").Bind("status").FG("green").Height(1),
	lugo.Input("name").Placeholder("Enter your name...").OnSubmit("greet").Height(1),
	lugo.Button("Greet", "greet").Height(1),
).Padding(1).Gap(1)

err := lugo.QuickView(view, map[string]lugo.ActionHandler{
	"greet": func(rt *lugo.Runtime, _ any) error {
		return rt.Set("status", "Hello, "+rt.String("name")+"!")
	},
}, lugo.WithState("status", "Ready"))
```

### 3. Hybrid Go + Lua Live Hot-Reload (LugoTUI's Killer Feature)

Move your layout into `app.lua` to edit spacing, colors, and widgets while the app is running (**150ms live hot-reload** preserving state and focus). Widget events (`on_click = "greet"`, `on_submit = "greet"`) dispatch directly to your Go handlers without needing Lua `actions` boilerplate:

```lua
-- app.lua
local ui = require('ui')

return {
    api_version = 1,
    spatial_nav = true,
    view = function()
        return ui.column {
            padding = 1, gap = 1,
            ui.text { bind = "status", fg = "green", height = 1 },
            ui.input { id = "name_in", bind = "name", placeholder = "Enter your name...", on_submit = "greet", height = 1 },
            ui.button { id = "btn_greet", text = "Greet", on_click = "greet", height = 1 },
        }
    end,
}
```

```go
// main.go — identical Go handlers!
p := lugo.NewProgram(
	lugo.WithAppFile("app.lua"),
	lugo.WithState("status", "Ready"),
	lugo.WithAction("greet", func(rt *lugo.Runtime, _ any) error {
		return rt.Set("status", "Hello, "+rt.String("name")+"!")
	}),
)
if err := p.Run(); err != nil {
	log.Fatal(err)
}
```

---

## LugoTUI vs. Bubble Tea

| Capability | Bubble Tea + Lip Gloss + Bubbles | LugoTUI |
| --- | --- | --- |
| **Pure-Go Declarative UI** | Manual string concatenation in `View() string` | Fluent `*lugo.Node` Builder (`Column`, `Row`, `Panel`, `Table`, `Tree`, `Input`) |
| **Event Closures & Dispatch** | Manual type-switches on `tea.Msg` in `Update()` | Direct **Inline Closures** (`.OnClickDo`, `.OnChangeText`, `.KeyDo`) or registered actions |
| **Key Event Handling** | Manual message forwarding through nested models | Automatic **Key Event Bubbling** up container hierarchy (`Node` -> `Parent` -> `Root`) |
| **Layout Engine** | Manual `tea.WindowSizeMsg` width/height arithmetic | Built-in **Flexbox Layout** (`Grow`, `Shrink`, `Padding`, `Gap`, `Align`, `Justify`) |
| **Focus & Navigation** | Manual focus index tracking & child `Update` routing | Automatic **Tab/Shift+Tab**, **2D Spatial Navigation**, & **Modal Focus Trap** |
| **Two-Way State Binding** | Manual state copying to/from child models | Automatic `.Bind("key")` + `rt.String("key")` / `rt.Set("key", val)` |
| **Live Hot-Reload** | None (requires full Go recompile & restart) | **150ms Lua Live Hot-Reload** preserving state & focus |
| **300-Node Frame Cost** | Full string re-allocation + ANSI parsing | **~297 µs / 18 heap allocs** (`cellBuffer` diff renderer + layout dirty cache) |
| **AI Agent / IPC Control** | None built-in | Built-in Unix domain socket JSON Lines IPC (`snapshot`, `apply`, `events`) |

---

## Documentation

* 🧩 **[Widgets & Lua Reference](docs/widgets.md)** — Layout rules, widget properties, and keybindings.
* 🤝 **[Architecture & Concurrency](docs/architecture.md)** — Goroutine ownership, task workers, and engine internals.
* 📊 **[Performance & Benchmarks](docs/benchmarks.md)** — Latency numbers, memory allocation stats, and reproduction steps.
* 🤖 **[Agent IPC Socket Protocol](docs/socket-protocol.md)** — JSON Lines API (`snapshot`, `apply`, `events`).
* 🔒 **[Linux Sandbox](docs/sandbox.md)** — Bubblewrap containerization for untrusted UI scripts.
* 💻 **[CLI Installation](docs/install.md)** — Binary archives, checksums, and environment configuration.
* 🧠 **[Developer & Agent Skill](skills/lugotui/SKILL.md)** — Complete Pure-Go and Go+Lua guide for developers and AI coding agents.

---

## Development

```sh
make run      # Run the demo app
make test     # Run test suite with race detector
make cover    # Run tests with coverage gate (>=80%)
make check    # Headless parse/layout/render validation
```

---

## License

Distributed under the **[Mozilla Public License 2.0 (MPL-2.0)](LICENSE)**.
