package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/VintlGvard/LugoTUI/internal/engine"
)

func Sandbox(options Options) error {
	if options.OnEvent != nil || options.ReservedKeys != nil || options.Theme != (engine.Theme{}) {
		return errors.New("sandbox cannot serialize OnEvent, ReservedKeys, or Theme options")
	}
	if runtime.GOOS != "linux" {
		return errors.New("process sandbox requires Linux")
	}
	if options.SocketPath != "" {
		return errors.New("sandbox socket transport is disabled; no host directories are exposed")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return errors.New("sandbox requires bubblewrap; refusing unisolated execution")
	}
	limit, err := exec.LookPath("prlimit")
	if err != nil {
		return errors.New("sandbox requires prlimit; refusing unisolated execution")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	args := []string{"--as=1073741824", "--cpu=120", "--nofile=64", "--", bwrap, "--unshare-all", "--die-with-parent", "--new-session", "--clearenv", "--setenv", "TERM", os.Getenv("TERM"), "--setenv", "LANG", "C.UTF-8", "--tmpfs", "/", "--proc", "/proc", "--dev", "/dev", "--ro-bind", binary, "/app", "--chdir", "/"}
	if options.AppFile != "" {
		path, err := filepath.Abs(options.AppFile)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return errors.New("sandbox application must be regular file <= 1 MiB")
		}
		args = append(args, "--ro-bind", path, "/ui.lua")
	}
	args = append(args, "--", "/app")
	if options.AppFile != "" {
		args = append(args, "--app", "/ui.lua")
	}
	if options.Check {
		args = append(args, "--check")
	}
	if options.Snapshot {
		args = append(args, "--snapshot")
	}
	cmd := exec.Command(limit, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("sandbox execution failed (no fallback): %w", err)
	}
	return nil
}
