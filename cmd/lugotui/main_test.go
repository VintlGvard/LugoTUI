package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() { version, commit = oldVersion, oldCommit })
	for _, tc := range []struct {
		name, version, commit string
		info                  *debug.BuildInfo
		want                  string
	}{
		{name: "dev", want: "lugotui dev (commit unknown; "},
		{name: "local", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}}, want: "lugotui dev (commit abc; "},
		{name: "go install", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, want: "lugotui v1.2.3 (commit unknown; "},
		{name: "release override", version: "v2.0.0", commit: "release", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}}, want: "lugotui v2.0.0 (commit release; "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			version, commit = tc.version, tc.commit
			if got, want := buildVersion(tc.info), tc.want+runtime.Version()+")"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestCLIExitBehavior(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "lugotui")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-ldflags=-X main.version=v1.2.3 -X main.commit=testcommit", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string
	}{
		{"version", []string{"--version"}, 0, "lugotui v1.2.3 (commit testcommit; " + runtime.Version() + ")", ""},
		{"help", []string{"--help"}, 0, "", "Examples:"},
		{"short help", []string{"-h"}, 0, "", "Usage: lugotui [flags]"},
		{"unknown", []string{"--unknown"}, 1, "", "flag provided but not defined"},
		{"missing value", []string{"--app"}, 1, "", "flag needs an argument"},
		{"invalid bool", []string{"--check=maybe"}, 1, "", "invalid boolean value"},
		{"positional", []string{"app.lua"}, 1, "", "unexpected arguments"},
		{"version preserves errors", []string{"--version", "--unknown"}, 1, "", "flag provided but not defined"},
		{"version preserves positional error", []string{"--version", "extra"}, 1, "", "unexpected arguments"},
		{"version false", []string{"--version=false", "--check"}, 0, "OK:", ""},
		{"missing app", []string{"--check", "--app", filepath.Join(t.TempDir(), "missing.lua")}, 1, "", "missing.lua"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...)
			cmd.Dir = t.TempDir()
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code || !strings.Contains(stdout.String(), tc.stdout) || !strings.Contains(stderr.String(), tc.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, &stdout, &stderr)
			}
			if tc.stdout == "" && stdout.Len() != 0 {
				t.Fatalf("unexpected stdout: %s", &stdout)
			}
			if tc.stderr == "" && stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", &stderr)
			}
			if tc.name == "help" {
				for _, flag := range []string{"-app", "-check", "-sandbox", "-snapshot", "-socket", "-version", "default: embedded demo"} {
					if !strings.Contains(stderr.String(), flag) {
						t.Errorf("help missing %q", flag)
					}
				}
			}
		})
	}
}
