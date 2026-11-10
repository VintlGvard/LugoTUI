# Performance & Benchmarks

Detailed performance, memory footprint, and benchmarks for LugoTUI.

Tested on **12th Gen Intel(R) Core(TM) i5-12450H, Linux amd64, Go 1.26**:

* **Idle Behavior:** The main event loop waits for events when `NeedsTick()` is false. Timers, animated widgets, and a focused input or textarea with a blinking cursor activate the animation ticker (16ms interval). File-backed apps additionally poll file metadata every 150ms for hot-reload.
* **Sub-millisecond Frame Time:** Pure Flexbox layout calculation completes in **47 µs** (~21,000 passes/sec). State mutations (`ui.set`/`ui.patch`) execute in **165 µs**. Cell-diff frame rendering with `cellBuffer` takes **297 µs** (~3,360 FPS capability), with **18 heap allocations**. Cached textarea renders take **194 µs with 0 allocations**.
* **Minimal RAM Footprint:** Idle process consumes **~14 MB RSS** (`VmRSS: 14,436 kB`) with active GopherLua VM, reactive bindings, and widget tree. Static binary is **5.3 MB** (`CGO_ENABLED=0`, `-s -w`).

## Latency & Allocations

| Operation | Time (latency) | Memory (B/op) | Allocs (allocs/op) |
| :--- | :--- | :--- | :--- |
| **Visible Nodes Traversal** | `9.9 µs` | `0 B` | `0 allocs` |
| **Flexbox Layout Pass (120x40)** | `47.2 µs` | `768 B` | `11 allocs` |
| **State Mutation (`ui.set` / `ui.patch`)** | `165.1 µs` | `55.8 KB` | `419 allocs` |
| **Frame Render (`cellBuffer` Diff Pass)** | `297.0 µs` | `248 B` | `18 allocs` |
| **Cached Textarea Draw** | `194.8 µs` | `0 B` | `0 allocs` |
| **Large Scene Render (300 nodes)** | `535.1 µs` | `50.2 KB` | `12 allocs` |
| **Cold Start & Lua Tree Build (570 LOC)** | `4.13 ms` | `2.59 MB` | `35,525 allocs` |

## Architectural Optimizations
1. **`cellBuffer` Double-Buffered Diffing:** An in-memory matrix computes character and style diffs before touching terminal cells, slashing allocations from 26,563 down to 18 per frame.
2. **Layout Dirty Cache:** Unchanged frames skip Flexbox recalculation via `scrollDigest` and `layoutValid` state tracking.
3. **Text Line & Measurement Cache:** Text lines and grapheme break indices are cached per node, allowing repeated draws to execute with 0 heap allocations (`0 B/op`, `0 allocs/op`).

## Reproduce Locally

```sh
go test -bench='^Benchmark(InitLoad|LayoutPass|RenderDiffPass|TextareaCachedDraw|LargeScene|StateMutation|VisibleNodes)$' -benchmem ./...
```
