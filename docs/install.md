# CLI installation

Use [v0.1.1 release assets](https://github.com/VintlGvard/LugoTUI/releases/tag/v0.1.1). Choose `linux`, `macos` or `windows`, then `amd64` (x86-64) or `arm64` (ARM64). Unix archives are `lugotui_v0.1.1_<os>_<arch>.tar.gz`; Windows archives are `lugotui_v0.1.1_windows_<arch>.zip`. Also download `SHA256SUMS.txt`.

## Verify before running

Compare your archive's hash with its exact filename entry in `SHA256SUMS.txt`:

```sh
# Linux
sha256sum lugotui_v0.1.1_linux_amd64.tar.gz
# macOS
shasum -a 256 lugotui_v0.1.1_macos_arm64.tar.gz
```

```powershell
Get-FileHash .\lugotui_v0.1.1_windows_amd64.zip -Algorithm SHA256
```

A matching hash detects corruption; a checksum downloaded from the same release is not an independent signature or proof of publisher identity. Do not execute a mismatching archive.

## Linux

From the directory containing the downloaded archive:

```sh
mkdir -p lugotui-bin
tar -xzf lugotui_v0.1.1_linux_amd64.tar.gz --strip-components=1 -C lugotui-bin
mkdir -p "$HOME/.local/bin"
install -m 755 ./lugotui-bin/lugotui "$HOME/.local/bin/lugotui"
export PATH="$HOME/.local/bin:$PATH"
```

Persist that `PATH` setting in your shell profile if needed. Substitute `arm64` for ARM64 machines.

## macOS

```sh
mkdir -p lugotui-bin
tar -xzf lugotui_v0.1.1_macos_arm64.tar.gz --strip-components=1 -C lugotui-bin
mkdir -p "$HOME/.local/bin"
install -m 755 ./lugotui-bin/lugotui "$HOME/.local/bin/lugotui"
export PATH="$HOME/.local/bin:$PATH"
```

Persist `PATH` in your shell profile (for example `~/.zprofile` with zsh). Intel Macs need the `amd64` archive. These binaries are not signed/notarized; Gatekeeper may block downloaded executables. Verify the source and checksum first, then use macOS's per-app approval flow if you trust the binary. Do not disable Gatekeeper globally. Building from source is an alternative.

## Windows PowerShell

```powershell
Expand-Archive .\lugotui_v0.1.1_windows_amd64.zip -DestinationPath .\lugotui-bin
$bin = Join-Path $env:LOCALAPPDATA 'LugoTUI\bin'
New-Item -ItemType Directory -Force $bin | Out-Null
Copy-Item .\lugotui-bin\lugotui_v0.1.1_windows_amd64\lugotui.exe (Join-Path $bin 'lugotui.exe')
$env:Path = "$bin;$env:Path"
```

Add `%LOCALAPPDATA%\LugoTUI\bin` to your **user** `Path` in Windows Environment Variables for future terminals. The assignment above changes only this PowerShell session. ARM64 machines need the `arm64` archive.

## Verify PATH independently of the download directory

Open a new terminal after persisting `PATH`. From another directory, with no repository checkout required:

```sh
lugotui --help
lugotui --version
lugotui --check
lugotui --app /absolute/path/to/app.lua
```

`--help` and `--version` report usage and version information.

PowerShell accepts `lugotui --app C:\path\to\app.lua`. A relative `--app` path is resolved from the current working directory. Without `--app`, the CLI uses the bundled demo (a repository checkout may select its demo file for live editing). `--check` runs headlessly; normal launch requires an interactive terminal. Default quit keys are `Ctrl+Q` and `Ctrl+C`; `F5` reloads.

## Install from Go source instead

Requires Go 1.26.0+:

```sh
go install github.com/VintlGvard/LugoTUI/cmd/lugotui@v0.1.1
go env GOBIN GOPATH
```

Add `GOBIN` to `PATH`, or the `bin` subdirectory of `GOPATH` when `GOBIN` is empty. This installs the Lua runner; custom Go callbacks require your own Go host. See [README](../README.md#1-go-library) and [IPC](socket-protocol.md) for those paths.

## Verification scope

Public module resolution, pkg.go.dev access and anonymous release download were checked for `v0.1.1`. Archive availability alone does not verify a terminal, PATH setup or OS. macOS/Windows installation and terminal smoke tests need native verification; do not infer them from Linux checks. Sandbox additionally requires Linux, bubblewrap and prlimit; see [sandbox requirements](sandbox.md).
