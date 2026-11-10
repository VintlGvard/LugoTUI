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

	rt, err := lugotui.LoadViewFunc(
		func(r *lugotui.Runtime) *lugotui.Node {
			return lugotui.Column(
				lugotui.Textf("Counter: %d", r.Int("n")).SetID("lbl"),
				lugotui.Button("+1", "inc").SetID("btn"),
			)
		},
		lugotui.WithState("n", 41),
		lugotui.WithAction("inc", func(r *lugotui.Runtime, _ any) error {
			return r.AddInt("n", 1)
		}),
	)
	if err != nil {
		t.Fatalf("root LoadViewFunc failed: %v", err)
	}
	defer rt.Close()
	if err := rt.Action("inc", nil); err != nil {
		t.Fatalf("Action(inc): %v", err)
	}
	if got := rt.Int("n"); got != 42 {
		t.Fatalf("expected n=42, got %d", got)
	}
}
