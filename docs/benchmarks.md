# Performance & Benchmarks

Detailed performance, memory footprint, and benchmarks for LugoTUI.

Tested on **12th Gen Intel(R) Core(TM) i5-12450H, Linux amd64, Go 1.26**:

* **Idle Behavior:** The main event loop waits for events when `NeedsTick()` is false. Timers, animated widgets, and a focused input or textarea with a blinking cursor activate the animation ticker (16ms interval). File-backed apps additionally poll file metadata every 150ms for hot-reload.
* **Sub-millisecond Frame Time:** Pure Flexbox layout calculation completes in **45 µs** (~22,000 passes/sec). State mutations (`ui.set`/`ui.patch`) execute in **162 µs**. Full mutation + cell-diff render pass takes **2.6 ms** (~375 FPS).
* **Minimal RAM Footprint:** Idle process consumes **~14 MB RSS** (`VmRSS: 14,436 kB`) with active GopherLua VM, reactive bindings, and widget tree. Static binary is **5.3 MB** (`CGO_ENABLED=0`, `-s -w`).

## Latency & Allocations

| Operation | Time (latency) | Memory (B/op) | Allocs (allocs/op) |
| :--- | :--- | :--- | :--- |
| **Visible Nodes Traversal** | `10.1 µs` | `0 B` | `0 allocs` |
| **Flexbox Layout Pass (120x40)** | `45.3 µs` | `768 B` | `11 allocs` |
| **State Mutation (`ui.set` / `ui.patch`)** | `162.9 µs` | `55.8 KB` | `419 allocs` |
| **Frame Render & Cell Diffing** | `2.34 ms` | `276.0 KB` | `42,319 allocs` |
| **State Mutation + Full Diff Render** | `2.66 ms` | `331.9 KB` | `42,590 allocs` |
| **Cold Start & Lua Tree Build (570 LOC)** | `3.80 ms` | `2.59 MB` | `35,525 allocs` |

## Reproduce Locally

```sh
go test -bench='^Benchmark(InitLoad|LayoutPass|RenderDiffPass|StateMutation|VisibleNodes)$' -benchmem ./...
```
