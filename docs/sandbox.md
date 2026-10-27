# Linux Process Sandbox

LugoTUI features an optional, hardware-enforced process sandbox for Linux environments, designed to run untrusted Lua applications with strict isolation.

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

When `--sandbox` is activated, LugoTUI executes a new isolated instance using Linux namespaces and cgroups:

- **Filesystem Isolation:** The root filesystem is replaced with an empty `tmpfs`. The host filesystem and user home directories (`~`) are completely inaccessible. Only two files are mounted read-only into the sandbox:
  1. The compiled `lugotui` executable itself.
  2. The target `.lua` application file (if specified via `--app`).
- **Network Isolation:** Runs in an unshared, isolated network namespace (`--unshare-net`) with loopback unconfigured. Network access is physically impossible.
- **IPC & Process Isolation:** Runs in isolated PID and IPC namespaces (`--unshare-pid`, `--unshare-ipc`). The sandboxed process cannot inspect or signal any host processes.
- **Environment Sanitization:** Host environment variables are stripped completely. Only minimal terminal variables (`TERM` and `LANG`) are propagated.
- **Disabled Transports:** Agent Unix sockets (`--socket`) are explicitly disallowed in sandbox mode.
- **Fail-Closed Security:** If namespace creation, mounting, or resource limits fail, the process aborts immediately. It **never** falls back to unconfined execution.

---

## 3. Resource Limits (via `prlimit`)

The sandboxed process is subjected to strict kernel resource caps:

| Resource Limit | Value | Description |
| --- | --- | --- |
| Address Space (`RLIMIT_AS`) | 1 GiB | Total virtual memory cap for the process tree. |
| CPU Time (`RLIMIT_CPU`) | 120 seconds | Total accumulated CPU time allowed before SIGXCPU. |
| Open File Descriptors (`RLIMIT_NOFILE`) | 64 | Maximum number of open file descriptors. |

*Note:* Address space caps apply to virtual allocations and do not represent target resident set sizes (RSS).

---

## 4. Operational Considerations & Limitations

- **Terminal I/O:** Standard input and output (`stdin`/`stdout`) remain attached to the user's terminal emulator for interactive TUI rendering. This is process isolation, not a defense against vulnerabilities in terminal emulator escape sequence parsers.
- **Programmatic Options:** Custom Go callbacks (`OnEvent`), custom reserved keys, and programmatic theme mutations are rejected in subprocess sandbox mode.
- **Live Hot-Reload in Sandbox:** Editing an external Lua file bound read-only via `--app` may require restarting the sandbox if the editor replaces inodes during atomic saves.
- **In-Process vs. Subprocess:** `tui.Load(...)` in Go is trusted in-process execution. Restricted Lua libraries and execution deadlines alone do **not** constitute a security boundary. Only `--sandbox` provides process isolation.
