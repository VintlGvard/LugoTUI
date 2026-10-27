package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/VintlGvard/LugoTUI/internal/runner"
)

var version, commit string

func buildVersion(info *debug.BuildInfo) string {
	v, c := version, commit
	if info != nil {
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		if c == "" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					c = setting.Value
				}
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	if c == "" {
		c = "unknown"
	}
	return fmt.Sprintf("lugotui %s (commit %s; %s)", v, c, runtime.Version())
}

func main() {
	info, _ := debug.ReadBuildInfo()
	if err := runner.RunCLI(os.Args[1:], buildVersion(info)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
