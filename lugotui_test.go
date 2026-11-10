package lugotui_test

import (
	"testing"

	lugotui "github.com/VintlGvard/LugoTUI"
	"github.com/gdamore/tcell/v2"
)

func TestRootPackageProgramAndCheck(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 6)

	p := lugotui.NewProgram(
		lugotui.WithSource(`
local ui = require("ui")
return {
	view = ui.text{ id = "root", text = "Hello Root" },
}
`),
		lugotui.WithScreen(sim),
		lugotui.WithCheck(),
	)
	if err := p.Run(); err != nil {
		t.Fatalf("root NewProgram check failed: %v", err)
	}
}
