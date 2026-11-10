# Performance & Benchmarks

Detailed performance, memory footprint, and benchmarks for LugoTUI.

Tested on **12th Gen Intel(R) Core(TM) i5-12450H, Linux amd64, Go 1.26**:

* **Idle Behavior:** The main event loop waits for events when `NeedsTick()` is false. Timers, animated widgets, and a focused input or textarea with a blinking cursor activate the animation ticker (16ms interval). File-backed apps additionally poll file metadata every 150ms for hot-reload.
* **Sub-millisecond Frame Time:** Pure Flexbox layout calculation completes in **45 µs** (~22,000 passes/sec). State mutations (`ui.set`/`ui.patch`) execute in **162 µs**. Full frame render to `tcell` screen buffer takes **1.39 ms** (~720 FPS).
* **Minimal RAM Footprint:** Idle process consumes **~14 MB RSS** (`VmRSS: 14,436 kB`) with active GopherLua VM, reactive bindings, and widget tree. Static binary is **5.3 MB** (`CGO_ENABLED=0`, `-s -w`).

## Latency & Allocations

| Operation | Time (latency) | Memory (B/op) | Allocs (allocs/op) |
| :--- | :--- | :--- | :--- |
| **Visible Nodes Traversal** | `10.1 µs` | `0 B` | `0 allocs` |
| **Flexbox Layout Pass (120x40)** | `45.3 µs` | `768 B` | `11 allocs` |
| **State Mutation (`ui.set` / `ui.patch`)** | `162.9 µs` | `55.8 KB` | `419 allocs` |
| **Frame Render (`tcell` Screen Buffer)** | `1.39 ms` | `198.6 KB` | `26,563 allocs` |
| **State Mutation + Frame Render** | `1.63 ms` | `339.0 KB` | `27,897 allocs` |
| **Cold Start & Lua Tree Build (570 LOC)** | `3.80 ms` | `2.59 MB` | `35,525 allocs` |

## Reproduce Locally

```sh
go test -bench='^Benchmark(InitLoad|LayoutPass|RenderDiffPass|StateMutation|VisibleNodes)$' -benchmem ./...
```
