# Go + Lua background worker

Requires the Go version in the root `go.mod`. From the repository root:

```sh
go run ./examples/worker
```

Type text, then select **Run** with Tab and Enter (or click it). Status becomes
**Running**, then **Done** after one second; the result is uppercase text.
Type exactly `fail` for **Error** with `controlled failure`. Select **Cancel**
while Running for **Cancelled**. Starting again cancels the previous task.
Quit with Ctrl+Q or Ctrl+C; the managed runner closes the runtime and cancels
outstanding work. Workers observe `context.Context`; `Close` does not join them.
Edit the Lua file for hot reload, or press F5. Reload cancels old-runtime tasks.

```sh
go run ./examples/worker --check
go run ./examples/worker --snapshot
go test -race ./examples/worker
```

Headless commands validate/render but do not run the interactive task loop.
The test exercises Lua events, Go handlers, worker completion, controlled failure,
cancellation, shutdown cancellation, snapshots, and IPC response validation.

The default `AppFile` is `examples/worker/app.lua`, relative to the current
working directory. When copying the Go/Lua files into your own application,
set `--app app.lua` or change that default; initialize your module and pin
`github.com/VintlGvard/LugoTUI/tui@v0.1.1` as in the main README.

`OnEvent` executes synchronously on the runtime owner. It copies and validates
input before `StartTask`; workers never access the runtime. A normal returned
Go error becomes a runner diagnostic and does not invoke `finished`. Expected
application errors therefore travel as successful payloads with an `error`
field, which Lua renders. The example performs no network or filesystem work.

This example and the portable skill deliberately carry self-contained versions
of the same small flow: the skill must remain usable after copying its directory
without a repository checkout. Both copies have runnable tests.
