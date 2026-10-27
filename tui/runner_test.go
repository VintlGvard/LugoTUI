package tui_test

import (
	"path/filepath"
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
)

func TestHeadlessOptions(t *testing.T) {
	if err := tui.RunOptions(tui.Options{Check: true}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidArguments(t *testing.T) {
	if err := tui.Run([]string{"unexpected"}); err == nil {
		t.Fatal("extra argument accepted")
	}
	if err := tui.RunOptions(tui.Options{AppFile: filepath.Join(t.TempDir(), "absent.lua"), Check: true}); err == nil {
		t.Fatal("missing application accepted")
	}
}

func TestPublicRunnerOptions(t *testing.T) {
	options := tui.Options{
		Check:        true,
		Theme:        tui.Theme{Background: "black"},
		ReservedKeys: &tui.ReservedKeys{Quit: []string{"Ctrl+Q"}, Reload: []string{"F6"}},
		OnEvent:      func(*tui.Runtime, tui.HostEvent) error { t.Error("headless mode called callback"); return nil },
	}
	if err := tui.RunOptions(options); err != nil {
		t.Fatal(err)
	}
}
