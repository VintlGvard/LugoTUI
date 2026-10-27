#!/usr/bin/env python3
"""Build validated release archives. Default: exact tag snapshot; --local: working tree.

No publication here. Output directory must not exist. Requires Go, Git, Python 3.12+.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[2]
TARGETS = [(system, arch) for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]
LEGAL = ("LICENSE", "LICENCE", "COPYING", "NOTICE", "AUTHORS", "PATENTS", "COPYRIGHT")
SOURCE_URL = "https://github.com/VintlGvard/LugoTUI"


def command(cmd, *args, cwd=ROOT, env=None):
    if cmd not in ("git", "go"):
        raise ValueError(f"Disallowed command: {cmd}")
    return subprocess.run([cmd, *args], cwd=cwd, env=env, text=True, check=True, capture_output=True, shell=False).stdout.strip()


def environment(system, arch):
    return dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="0", GOWORK="off", GOFLAGS="")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def validate_archive(path, root, expected):
    """Check safe names, exact payload bytes, executable mode and ZIP CRCs."""
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            if archive.testzip() is not None:
                raise RuntimeError(f"Bad ZIP CRC: {path}")
            names = archive.namelist()
            actual = {name: archive.read(name) for name in names}
    else:
        with tarfile.open(path) as archive:
            members = archive.getmembers()
            if any(not member.isfile() for member in members):
                raise RuntimeError(f"Non-file archive entry: {path}")
            names = [member.name for member in members]
            actual = {member.name: archive.extractfile(member).read() for member in members}
            if not archive.getmember(root + "/lugotui").mode & 0o111:
                raise RuntimeError(f"Binary is not executable: {path}")
    if len(names) != len(set(names)):
        raise RuntimeError(f"Duplicate archive entries: {path}")
    if any(PurePosixPath(name).is_absolute() or ".." in PurePosixPath(name).parts or "\\" in name for name in names):
        raise RuntimeError(f"Unsafe archive entry: {path}")
    wanted = {root + "/" + name: data for name, data in expected.items()}
    if actual != wanted:
        raise RuntimeError(f"Archive payload mismatch: {path}")


def licenses(source, common):
    modules = {}
    for system, arch in TARGETS:
        lines = command("go", "list", "-mod=readonly", "-deps", "-f",
                        "{{if .Module}}{{if not .Module.Main}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{if .Module.Replace}}|REPLACED{{end}}{{end}}{{end}}",
                        "./cmd/lugotui", cwd=source, env=environment(system, arch))
        for line in lines.splitlines():
            if not line:
                continue
            fields = line.split("|")
            if len(fields) != 3:
                raise RuntimeError(f"Replaced dependencies are not supported in releases: {line}")
            name, version, directory = fields
            modules[name] = (version, Path(directory))
    for name, (version, directory) in sorted(modules.items()):
        matches = sorted(path for path in directory.rglob("*") if path.is_file() and path.name.upper().startswith(LEGAL))
        if not any(path.name.upper().startswith(("LICENSE", "LICENCE", "COPYING")) for path in matches):
            raise RuntimeError(f"Missing dependency license: {name}@{version}")
        destination = common / "licenses" / (name.replace("/", "_") + "@" + version)
        for path in matches:
            target = destination / path.relative_to(directory)
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
    goroot = Path(command("go", "env", "GOROOT", cwd=source))
    go_legal = common / "licenses" / "Go"
    go_legal.mkdir(parents=True)
    go_lic = goroot / "LICENSE"
    if not go_lic.is_file():
        alt = Path("/usr/share/licenses/go/LICENSE")
        if alt.is_file():
            go_lic = alt
    if go_lic.is_file():
        shutil.copyfile(go_lic, go_legal / "LICENSE")
    for name in ("PATENTS", "AUTHORS", "CONTRIBUTORS"):
        if (goroot / name).is_file():
            shutil.copyfile(goroot / name, go_legal / name)

    for path in sorted((goroot / "src").rglob("*")):
        if path.is_file() and path.name.upper().startswith(LEGAL):
            target = go_legal / "src" / path.relative_to(goroot / "src")
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
    (common / "DEPENDENCIES.txt").write_text("".join(f"{name} {version}\n" for name, (version, _) in sorted(modules.items())))
    return len(modules)


def build(args):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._+-]*", args.version):
        raise ValueError("Version must be a safe archive filename component")
    if not args.local and not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", args.version):
        raise ValueError("Release tag must have form vMAJOR.MINOR.PATCH[-prerelease]")
    revision = "HEAD" if args.local else f"refs/tags/{args.version}^{{commit}}"
    commit = command("git", "rev-parse", "--verify", revision)
    if args.commit and commit != args.commit:
        raise RuntimeError(f"Tag commit {commit} differs from requested commit {args.commit}")
    if not args.local and command("git", "rev-parse", "HEAD") != commit:
        raise RuntimeError("Checkout HEAD must be exact tag commit")
    if args.dry_run:
        print(json.dumps({"version": args.version, "commit": commit, "local": args.local,
                          "targets": TARGETS, "output": str(args.output.resolve())}, indent=2))
        return

    args.output.mkdir(parents=True)
    with tempfile.TemporaryDirectory(prefix="lugotui-release-") as temporary:
        stage = Path(temporary)
        source = ROOT
        if not args.local:
            source = stage / "source"
            source.mkdir()
            snapshot = subprocess.check_output(["git", "archive", commit], cwd=ROOT)
            with tarfile.open(fileobj=io.BytesIO(snapshot)) as archive:
                archive.extractall(source, filter="data")
        common = stage / "common"
        common.mkdir()
        for name in ("LICENSE", "NOTICE", "README.md"):
            shutil.copyfile(source / name, common / name)
        shutil.copyfile(source / "internal/assets/demo.lua", common / "demo.lua")
        count = licenses(source, common)
        for system, arch in TARGETS:
            human_system = "macos" if system == "darwin" else system
            name = f"lugotui_{args.version}_{human_system}_{arch}"
            payload = stage / name
            shutil.copytree(common, payload)
            binary = payload / ("lugotui.exe" if system == "windows" else "lugotui")
            flags = f"-s -w -X main.version={args.version} -X main.commit={commit}"
            subprocess.run(["go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", f"-ldflags={flags}",
                            "-o", str(binary), "./cmd/lugotui"], cwd=source, env=environment(system, arch), check=True)
            info = command("go", "version", "-m", str(binary))

            for setting in (f"GOOS={system}", f"GOARCH={arch}", "CGO_ENABLED=0"):
                if setting not in info:
                    raise RuntimeError(f"Missing build setting {setting}: {info}")
            (payload / "BUILD.txt").write_text(
                f"LugoTUI {args.version}\nCommit: {commit}\nLocal working tree: {args.local}\n"
                f"Source: {SOURCE_URL}/tree/{commit}\nSource archive: {SOURCE_URL}/archive/{commit}.tar.gz\n"
                f"Toolchain: {command('go', 'version')}\n"
                f"Build: CGO_ENABLED=0 GOOS={system} GOARCH={arch} go build -mod=readonly -trimpath -buildvcs=false -ldflags='{flags}' ./cmd/lugotui\n"
                "Run: ./lugotui --app demo.lua (Windows: .\\lugotui.exe --app demo.lua)\n"
                "macOS binaries are unsigned and not notarized.\n\n" + info + "\n")
            host_system = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(platform.system())
            host_arch = {"x86_64": "amd64", "AMD64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine())
            if (system, arch) == (host_system, host_arch):
                exe = "./lugotui.exe" if system == "windows" else "./lugotui"
                version = subprocess.run([exe, "--version"], cwd=payload, text=True, check=True, capture_output=True, shell=False).stdout.strip()
                if args.version not in version or commit not in version:
                    raise RuntimeError(f"Incorrect CLI version: {version}")
                subprocess.run([exe, "--check"], cwd=payload, check=True, shell=False)
                subprocess.run([exe, "--app", "demo.lua", "--check"], cwd=payload, check=True, shell=False)
            files = sorted(path for path in payload.rglob("*") if path.is_file())
            expected = {path.relative_to(payload).as_posix(): path.read_bytes() for path in files}
            artifact = args.output / (name + (".zip" if system == "windows" else ".tar.gz"))
            if system == "windows":
                with zipfile.ZipFile(artifact, "x", zipfile.ZIP_DEFLATED) as archive:
                    for path in files:
                        archive.write(path, name + "/" + path.relative_to(payload).as_posix())
            else:
                with tarfile.open(artifact, "x:gz") as archive:
                    for path in files:
                        archive.add(path, arcname=name + "/" + path.relative_to(payload).as_posix(), recursive=False)
            validate_archive(artifact, name, expected)
            print(f"Validated {artifact.name}", flush=True)
    checksums = {path.name: sha256(path.read_bytes()) for path in sorted(args.output.iterdir())}
    if len(checksums) != 6:
        raise RuntimeError("Expected exactly six release archives")
    sums = args.output / "SHA256SUMS.txt"
    sums.write_text("".join(f"{digest}  {name}\n" for name, digest in checksums.items()))
    for line in sums.read_text().splitlines():
        digest, name = line.split("  ", 1)
        if sha256((args.output / name).read_bytes()) != digest:
            raise RuntimeError(f"Checksum mismatch: {name}")
    print(f"Validated six archives, SHA256SUMS.txt, {count} dependency modules and Go runtime licenses")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--commit", help="Expected full tag commit SHA")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--local", action="store_true", help="Build working tree, never for publication")
    parser.add_argument("--dry-run", action="store_true", help="Resolve tag and show plan without building or writing")
    build(parser.parse_args())


if __name__ == "__main__":
    main()
