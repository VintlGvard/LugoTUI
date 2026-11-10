# LugoTUI

[![Go Reference](https://pkg.go.dev/badge/github.com/VintlGvard/LugoTUI.svg)](https://pkg.go.dev/github.com/VintlGvard/LugoTUI)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-blue.svg)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/VintlGvard/LugoTUI)](https://github.com/VintlGvard/LugoTUI/releases/latest)
[![CI](https://github.com/VintlGvard/LugoTUI/actions/workflows/ci.yml/badge.svg)](https://github.com/VintlGvard/LugoTUI/actions/workflows/ci.yml)

**Declarative, High-Performance TUI Engine for Go with Pure-Go DSL, instant Lua Live Hot-Reload, and built-in AI Agent IPC.**

Build modern terminal interfaces in **100% Pure Go** (`QuickViewFunc`, fluent View Builder DSL) or separate layouts into `.lua` with file-backed live hot-reload polling every 150ms. Zero CGO, single static binary.

![LugoTUI Demo](docs/assets/demo.gif)

---

## Why LugoTUI?

* 🚀 **Pure Go or Lua:** Write 100% type-safe Go with inline closures (`OnClickDo`, `OnChangeText`, `KeyDo`), or use Lua for instant visual iteration without recompiling.
* 🔥 **150ms Live Hot-Reload:** Edit `.lua` layouts on the fly; state, cursor position, and keyboard focus are preserved across reloads.
* ⚡ **Sub-Millisecond Cell-Diffing:** Double-buffered `cellBuffer` renderer and layout dirty-tracking render 300 nodes in **~297 µs with 18 heap allocations**.
* 📐 **Flexbox & 2D Spatial Nav:** Native Flexbox (`row`, `column`, `grow`, `shrink`, `gap`, `padding`), 2D arrow-key navigation, and modal focus traps.
* 🎨 **Canvas & Custom Borders:** Draw arbitrary widgets with `Custom(func(c Canvas))`, rounded/double/thick borders, and custom button styles.
* 🧩 **Rich Native Widgets:** Out-of-the-box `table`, `tree`, filterable `list`, `viewport`, `input`, `textarea`, `modal`, `filepicker`, and 12 `spinner` presets with mouse and Vim support.
* 🤖 **AI Agent-Ready (IPC Socket):** Inspect snapshots, trigger actions, and stream events externally via a JSON Lines Unix domain socket.
* 🛡️ **Transactional & Sandboxed:** Failed actions roll back cleanly without screen corruption. Optional Linux `bubblewrap` sandbox isolates untrusted scripts.

---

## Quickstart

### 1. 100% Pure Go (`QuickViewFunc`)

```sh
go get github.com/VintlGvard/LugoTUI@latest
```

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
					lugo.Button("Refresh").Chevrons().OnClickDo(func() { rt.AddInt("polls", 1) }),
					lugo.Button("Quit").Plain().OnClickErr(rt.Quit),
				).Gap(1),
			).Border("rounded").BorderFG("cyan").Grow(1).Padding(1).Gap(1),
		).Padding(1).KeyDo("ctrl+q", func() { rt.Quit() })
	}, nil, lugo.WithState("polls", 1))
	if err != nil {
		log.Fatal(err)
	}
}
```

### 2. Hybrid Go + Lua Live Hot-Reload

Define UI in `app.lua` and edit it live without stopping your Go process:

```lua
local ui = require('ui')

return {
  api_version = 1,
  view = function()
    return ui.column {
      padding = 1, gap = 1,
      ui.text { bind = "status", fg = "green", height = 1 },
      ui.input { id = "name_in", bind = "name", placeholder = "Type your name...", on_submit = "greet", height = 1 },
      ui.button { id = "btn", text = "Greet", on_click = "greet", height = 1 }
    }
  end,
}
```

Run from `main.go` with Go action handlers:

```go
package main

import (
	"log"

	lugo "github.com/VintlGvard/LugoTUI"
)

func main() {
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
}
```

### 3. Standalone CLI

Run Lua applications without writing Go:

```sh
go install github.com/VintlGvard/LugoTUI/cmd/lugotui@latest

lugotui --app app.lua          # Run UI
lugotui --app app.lua --check  # Headless validation
```

Or download pre-built binaries from [Releases](https://github.com/VintlGvard/LugoTUI/releases/latest). See [CLI Installation Guide](docs/install.md).

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
