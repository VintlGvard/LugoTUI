# LugoTUI

[![Go Reference](https://pkg.go.dev/badge/github.com/VintlGvard/LugoTUI/tui.svg)](https://pkg.go.dev/github.com/VintlGvard/LugoTUI/tui)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-blue.svg)](LICENSE)
![Release: v0.1.0-beta](https://img.shields.io/badge/Release-v0.1.0--beta-orange.svg)

**Declarative, Lua-driven Terminal UI Engine for Go with instant Live Hot-Reload and built-in AI Agent IPC.**

Stop recompiling your Go binary every time you tweak a margin or color. **LugoTUI** separates heavy backend logic (Go) from reactive UI layouts (embedded Lua) — giving you a Flutter-like declarative syntax, Flexbox layout, and 150ms live reload in a single **100% pure Go (`CGO_ENABLED=0`)** static binary.

![LugoTUI Demo](docs/assets/demo.gif)

---

## Why LugoTUI?

* 🔥 **Instant Live Hot-Reload:** Edit `.lua` files and watch the terminal update in `<150ms` while preserving focus, inputs, and state.
* 📦 **Zero CGO, Pure Go:** Powered by `tcell/v2`, `uniseg`, and `GopherLua`. Cross-compiles to a single static binary on Linux, macOS, and Windows.
* 📐 **Flexbox Layout Engine:** Built-in `row`, `column`, `grow`, `shrink`, `gap`, `padding`, `z_index`, and absolute positioning.
* 🧩 **Rich Native Widgets:** Out-of-the-box `table`, `tree`, filterable `list`, `viewport`, `input`, `textarea`, `modal`, `filepicker`, gradients, and 12 `spinner` presets with Vim navigation (`j/k/g/G`) and full mouse support.
* 🤖 **AI Agent-Ready (IPC Socket):** Inspect UI snapshots, trigger actions, and stream events externally via a JSON Lines Unix domain socket.
* 🛡️ **Transactional & Sandboxed:** Lua errors roll back cleanly without crashing the screen. Optional Linux `bubblewrap` + `prlimit` sandbox isolates untrusted UI scripts.
* 🪶 **Event-Driven Rendering:** The animation ticker stops when `NeedsTick()` is false. A blinking input cursor also needs ticks; file-backed apps poll for changes every 150ms.

---

## Performance & Memory Footprint

Tested on **12th Gen Intel(R) Core(TM) i5-12450H, Linux amd64, Go 1.26**:

* **Idle Behavior:** The main event loop waits for events when `NeedsTick()` is false. Timers, animated widgets, and a focused input or textarea with a blinking cursor activate the animation ticker (16ms interval). File-backed apps additionally poll file metadata every 150ms for hot-reload. Idle CPU depends on the active UI and environment; no universal 0.00% guarantee is implied.
* **Sub-millisecond Frame Time:** Pure Flexbox layout calculation completes in **45 µs** (~22,000 passes/sec). State mutations (`ui.set`/`ui.patch`) execute in **162 µs**. Full mutation + cell-diff render pass takes **2.6 ms** (~375 FPS).
* **Minimal RAM Footprint:** Idle process consumes **~14 MB RSS** (`VmRSS: 14,436 kB`) with active GopherLua VM, reactive bindings, and widget tree. Static binary is **5.3 MB** (`CGO_ENABLED=0`, `-s -w`).

| Operation | Time (latency) | Memory (B/op) | Allocs (allocs/op) |
| :--- | :--- | :--- | :--- |
| **Visible Nodes Traversal** | `10.1 µs` | `0 B` | `0 allocs` |
| **Flexbox Layout Pass (120x40)** | `45.3 µs` | `768 B` | `11 allocs` |
| **State Mutation (`ui.set` / `ui.patch`)** | `162.9 µs` | `55.8 KB` | `419 allocs` |
| **Frame Render & Cell Diffing** | `2.34 ms` | `276.0 KB` | `42,319 allocs` |
| **State Mutation + Full Diff Render** | `2.66 ms` | `331.9 KB` | `42,590 allocs` |
| **Cold Start & Lua Tree Build (570 LOC)** | `3.80 ms` | `2.59 MB` | `35,525 allocs` |

> *Reproduce locally: `go test -bench='^Benchmark(InitLoad|LayoutPass|RenderDiffPass|StateMutation|VisibleNodes)$' -benchmem ./...`*

---

## 🚀 Quick Start (30 Seconds)

### 1. Install
Requires **Go 1.26+**.
```sh
go get github.com/VintlGvard/LugoTUI
```

### 2. Try the built-in interactive demo
```sh
go run ./cmd/lugotui
```

### 3. Build your first screen

Create **`app.lua`** (your declarative UI & state):
```lua
local ui = require('ui')

return {
  api_version = 1,
  view = function()
    return ui.column {
      padding = 1, gap = 1,
      ui.text { text = "Hello from LugoTUI!", height = 1 },
      ui.input { id = "name_in", bind = "name", placeholder = "Type your name...", height = 1 },
      ui.text { id = "greeting", text = "Waiting for input...", height = 1 },
      ui.button { id = "btn", text = "Uppercase & Save", on_click = "save", height = 1 }
    }
  end,
  actions = {
    save = function()
      local name = ui.get("name") or "Stranger"
      ui.set("name", string.upper(name))
      ui.patch("greeting", { text = "Saved: " .. string.upper(name) })
    end
  }
}
```

Create **`main.go`** (your Go host):
```go
package main

import (
	"log"

	"github.com/VintlGvard/LugoTUI/tui"
)

func main() {
	err := tui.RunOptions(tui.Options{
		AppFile: "app.lua",
		Theme:   tui.Theme{Background: "black", FocusForeground: "cyan"},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Run `go run main.go` from the directory containing both files. The **Uppercase & Save** button runs the Lua `save` action; no Go callback is needed. For host events and background workers, see [Managed Runner Integration](docs/architecture.md#3-managed-runner-integration).

---

## 🧠 Mental Model: How Go & Lua Talk

LugoTUI uses a strict **Functional Core, Imperative Shell** architecture:

1. **Lua owns the View:** `.lua` scripts declare the widget tree (`view`) and handle fast, synchronous UI mutations (`ui.set`, `ui.patch`, `ui.replace`) inside a single-threaded transactional runtime.
2. **Go owns the I/O & Concurrency:** Your Go application handles network calls, databases, and heavy computation via `OnEvent` and `r.StartTask(...)`, pushing results back to the UI safely without data races.
3. **External Agents (Optional):** AI agents or CLI tools can connect via `--socket ./agent.sock` using JSON Lines (`snapshot`, `apply`, `message`, `events`) to read or drive the TUI remotely.

---

## 📚 Documentation & Deep Dives

Detailed specifications and reference guides live in [`docs/`](docs/):

* 🧩 **[Widgets & Lua Contract](docs/widgets.md)** — Flexbox properties, `table`, `tree`, `viewport`, inputs, animations, and default Vim/mouse keybindings.
* 🤝 **[Architecture, Embedding & Limits](docs/architecture.md)** — Project layout, Managed `Runner` vs. low-level `SimulationScreen` embedding, goroutine ownership, and engine limits.
* 🤖 **[Agent IPC Socket Protocol](docs/socket-protocol.md)** — JSON Lines schema, methods (`snapshot`, `apply`, `message`, `events`), permissions, and error codes.
* 🔒 **[Linux Process Sandbox](docs/sandbox.md)** — Securing untrusted Lua scripts with `bubblewrap` and `prlimit`.

---

## 🛠️ Development & Cross-Platform Commands

```sh
make run      # Run the demo app
make test     # Run full test suite (with race detector)
make check    # Headless single-tick parse/layout/render validation (--check)
./lugotui --socket ./agent.sock
```

<details>
<summary><b>Windows PowerShell Commands (No Make)</b></summary>

```powershell
$env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o lugotui.exe ./cmd/lugotui
.\lugotui.exe --check
go vet ./...
# Race detection needs CGO and a supported C compiler on PATH.
$env:CGO_ENABLED='1'
go test -race ./...
```

On macOS/Linux, native commands without GNU Make are:
```sh
go build ./cmd/lugotui
go vet ./...
go test -race ./...
go run ./cmd/lugotui --check
```
</details>

---

## ⚖️ API Stability & License

* **Status (`v0.x Beta`):** Minor releases may introduce breaking changes to the Lua contract or Go facade while the API stabilizes toward `v1.0`.
* **License:** Distributed under the **[Mozilla Public License 2.0 (MPL-2.0)](LICENSE)**.
