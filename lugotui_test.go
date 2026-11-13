package lugotui_test

import (
	"bytes"
	"strings"
	"sync/atomic"
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

func TestRootPackageHelpers(t *testing.T) {
	items := lugotui.Map([]string{"alpha", "beta"}, func(i int, item string) *lugotui.Node {
		return lugotui.Textf("%d:%s", i, item).SetID(item)
	})
	if len(items) != 2 {
		t.Fatalf("expected 2 mapped nodes, got %d", len(items))
	}

	var clipOut bytes.Buffer
	clip := lugotui.NewOSC52Clipboard(&clipOut)
	if err := clip.Write("root-clip"); err != nil {
		t.Fatalf("clip.Write failed: %v", err)
	}
	if !strings.Contains(clipOut.String(), "\x1b]52;c;") {
		t.Fatalf("expected OSC52 escape sequence, got %q", clipOut.String())
	}

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 6)

	var renderCalls atomic.Int32
	ready := make(chan struct{})
	p := lugotui.NewProgram(
		lugotui.WithView(lugotui.Column(items...)),
		lugotui.WithScreen(sim),
		lugotui.WithClipboard(clip),
		lugotui.WithRenderer(lugotui.NewScreenRenderer(func(screen tcell.Screen, rt *lugotui.Runtime, diagnostic string) {
			renderCalls.Add(1)
			rt.Draw(screen, diagnostic)
		})),
		lugotui.WithOnReady(func(_ *lugotui.Runtime) { close(ready) }),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()
	<-ready
	p.Quit()

	if err := <-errCh; err != nil {
		t.Fatalf("Program.Run failed: %v", err)
	}
	if renderCalls.Load() < 1 {
		t.Fatalf("expected custom screen renderer to run, got %d calls", renderCalls.Load())
	}
}
