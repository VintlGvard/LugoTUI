# Linux Process Sandbox

LugoTUI offers optional Linux process isolation for Lua applications. It uses kernel namespaces, read-only bind mounts and resource limits, not a VM or a guarantee against kernel vulnerabilities.

---

## 1. Sandbox Architecture & Prerequisites

The sandbox isolates the **entire UI process** at the operating system level rather than relying solely on language-level interpreter restrictions.

### System Prerequisites
- **Linux kernel** with user namespace support enabled (`CONFIG_USER_NS=y`).
- **`bubblewrap`** (`bwrap`) executable available on `$PATH`.
- **`prlimit`** executable (from `util-linux`) available on `$PATH`.
- **Static Binary:** The application must be compiled statically (`CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' ...` or `make build`).

### CLI Usage

```sh
# Run interactive UI in an isolated Linux sandbox
./lugotui --sandbox

# Validate an untrusted Lua script inside the sandbox headlessly
./lugotui --sandbox --app untrusted.lua --check
```

---

## 2. Isolation Guarantees

When `--sandbox` is activated, LugoTUI executes a new isolated instance using Linux namespaces and `prlimit` (not cgroups):

- **Filesystem Isolation:** The root filesystem is replaced with an empty `tmpfs`. The host filesystem and user home directories (`~`) are completely inaccessible. Only two files are mounted read-only into the sandbox:
  1. The compiled `lugotui` executable itself.
  2. The target `.lua` application file (if specified via `--app`).
- **Network Isolation:** Runs in an unshared, isolated network namespace (`--unshare-net`) with loopback unconfigured. No host network interfaces are exposed.
- **IPC & Process Isolation:** Runs in isolated PID and IPC namespaces (`--unshare-pid`, `--unshare-ipc`). The sandboxed process cannot inspect or signal any host processes.
- **Environment Sanitization:** Host environment variables are stripped completely. Only minimal terminal variables (`TERM` and `LANG`) are propagated.
- **Disabled Transports:** Agent Unix sockets (`--socket`) are explicitly disallowed in sandbox mode.
- **Fail-Closed Security:** If namespace creation, mounting, or resource limits fail, the process aborts immediately. It **never** falls back to unconfined execution.

---

## 3. Resource Limits (via `prlimit`)

The sandboxed process is subjected to strict kernel resource caps:

| Resource Limit | Value | Description |
| --- | --- | --- |
| Address Space (`RLIMIT_AS`) | 1 GiB | Virtual address space limit per process, inherited by children. |
| CPU Time (`RLIMIT_CPU`) | 120 seconds | CPU time limit per process before SIGXCPU; inherited by children. |
| Open File Descriptors (`RLIMIT_NOFILE`) | 64 | Maximum number of open file descriptors. |

*Note:* Address space caps apply to virtual allocations and do not represent target resident set sizes (RSS).

---

## 4. Operational Considerations & Limitations

- **Terminal I/O:** Standard input and output (`stdin`/`stdout`) remain attached to the user's terminal emulator for interactive TUI rendering. This is process isolation, not a defense against vulnerabilities in terminal emulator escape sequence parsers.
- **Programmatic Options:** Custom Go callbacks (`OnEvent`), custom reserved keys, and programmatic theme mutations are rejected in subprocess sandbox mode.
- **Live Hot-Reload in Sandbox:** Editing an external Lua file bound read-only via `--app` may require restarting the sandbox if the editor replaces inodes during atomic saves.
- **In-Process vs. Subprocess:** `tui.Load(...)` in Go is trusted in-process execution. Restricted Lua libraries and execution deadlines alone do **not** constitute a security boundary. `--sandbox` requests OS process isolation subject to the requirements above.

### Re-execution and embedded Go hosts

`--sandbox` starts `prlimit`, then `bwrap`, then **the current executable** obtained through `os.Executable()` and resolved through symlinks. Inside the sandbox that executable is `/app`; the optional Lua file is `/ui.lua`. The child receives only `--app /ui.lua` when applicable, `--check` and/or `--snapshot`. It does not receive `--sandbox`, so isolation does not recurse. With no `--app`, the child uses the embedded demo.

The stock `lugotui` CLI implements this child command-line contract. A custom executable calling `tui.RunOptions` with `Sandbox: true` is supported only if its entry point also accepts those flags and invokes the runner **before** unrelated host initialization. Arbitrary Go hosts are not automatically supported: flags, services, credentials, callbacks and application state are not serialized or recreated. A host that always enables `Sandbox` can recurse; one that ignores child flags can start the wrong program. Prefer invoking the stock static CLI as a separate process for untrusted Lua.

The child root contains no dynamic linker, shared libraries, terminal database or host configuration. Use a static build and a terminal type supported by the compiled-in terminal descriptions. Containers, security policies, or disabled unprivileged user namespaces can prevent bubblewrap from starting even when both tools are installed. Such failures return a nonzero CLI exit status; they never trigger an unisolated fallback.

### Verification scope

Headless stock CLI checks were verified on Linux (CachyOS, kernel 7.2.11) with `bwrap` and `prlimit` installed, using a `CGO_ENABLED=0` binary and `--sandbox --check`. This verifies process startup and embedded Lua validation, not interactive rendering or a manual terminal compatibility matrix. Automated runner tests cover task cancellation, socket cleanup and `Screen.Fini` on normal exit and socket startup errors. A Linux automated pseudo-terminal check (`TERM=xterm-256color`, normal startup and Ctrl+Q exit) also confirmed identical terminal attributes before and after exit. This is not a manual terminal test; interactive terminal behavior on other OSes remains unverified.
