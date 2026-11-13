package tui_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestProgramLifecycleAndControl(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(60, 10)

	const src = `
local ui = require("ui")
ui.set("msg", "initial")
return {
	view = ui.column{
		id = "root",
		ui.text{ id = "lbl", bind = "msg" },
		ui.button{ id = "btn", text = "Fire", on_click = "fire" },
	},
	actions = {
		fire = function(payload)
			local suffix = payload and payload.suffix or "!"
			ui.set("msg", ui.get("msg") .. suffix)
			host.dispatch("fired", { msg = ui.get("msg") })
		end,
	},
}
`
	var (
		mwOrder      []string
		observedKeys []string
		renderCalls  atomic.Int32
		handledMsg   atomic.Value
	)
	ready := make(chan struct{})

	mw1 := func(next tui.EventHandler) tui.EventHandler {
		return func(r *tui.Runtime, ev tui.HostEvent) error {
			mwOrder = append(mwOrder, "mw1:"+ev.Action)
			return next(r, ev)
		}
	}
	mw2 := func(next tui.EventHandler) tui.EventHandler {
		return func(r *tui.Runtime, ev tui.HostEvent) error {
			mwOrder = append(mwOrder, "mw2:"+ev.Action)
			return next(r, ev)
		}
	}

	p := tui.NewProgram(
		tui.WithSource(src),
		tui.WithScreen(sim),
		tui.WithFPS(60),
		tui.WithMouse(true),
		tui.WithPaste(true),
		tui.WithAltScreen(true),
		tui.WithMiddleware(mw1, mw2),
		tui.WithHandlers(map[string]tui.ActionHandler{
			"fired": func(_ *tui.Runtime, payload any) error {
				if m, ok := payload.(map[string]any); ok {
					handledMsg.Store(m["msg"])
				}
				return nil
			},
		}),
		tui.WithStateObserver(func(key string, _, _ any) {
			observedKeys = append(observedKeys, key)
		}),
		tui.WithRendererFunc(func(screen tcell.Screen, rt *tui.Runtime, diagnostic string) {
			renderCalls.Add(1)
			rt.Draw(screen, diagnostic)
		}),
		tui.WithOnReady(func(_ *tui.Runtime) {
			close(ready)
		}),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()

	<-ready
	if err := p.Set("msg", "hello"); err != nil {
		t.Fatalf("p.Set failed: %v", err)
	}
	if err := p.Send("fire", map[string]any{"suffix": " world"}); err != nil {
		t.Fatalf("p.Send failed: %v", err)
	}
	if err := p.Patch("btn", map[string]any{"text": "Done"}); err != nil {
		t.Fatalf("p.Patch failed: %v", err)
	}
	if err := p.ExitAltScreen(); err != nil {
		t.Fatalf("p.ExitAltScreen failed: %v", err)
	}
	if err := p.EnterAltScreen(); err != nil {
		t.Fatalf("p.EnterAltScreen failed: %v", err)
	}
	p.Quit()

	if err := <-errCh; err != nil {
		t.Fatalf("Program.Run returned error: %v", err)
	}
	if got, _ := handledMsg.Load().(string); got != "hello world" {
		t.Fatalf("expected handledMsg 'hello world', got %q", got)
	}
	if len(mwOrder) != 2 || mwOrder[0] != "mw1:fired" || mwOrder[1] != "mw2:fired" {
		t.Fatalf("unexpected middleware order: %v", mwOrder)
	}
	if len(observedKeys) < 2 {
		t.Fatalf("expected state observer notifications, got %v", observedKeys)
	}
	if renderCalls.Load() < 2 {
		t.Fatalf("expected custom renderer to be invoked, got %d", renderCalls.Load())
	}
}

func TestPanicRecoveryRestoresTerminal(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 10)

	const src = `
local ui = require("ui")
return {
	view = ui.button{ id = "b", text = "Crash", on_click = "crash" },
	actions = {
		crash = function()
			host.dispatch("boom", {})
		end,
	},
}
`
	ready := make(chan struct{})
	p := tui.NewProgram(
		tui.WithSource(src),
		tui.WithScreen(sim),
		tui.WithOnReady(func(_ *tui.Runtime) { close(ready) }),
		tui.WithOnEvent(func(_ *tui.Runtime, ev tui.HostEvent) error {
			if ev.Action == "boom" {
				panic("simulated host handler explosion")
			}
			return nil
		}),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()
	<-ready
	_ = p.Send("crash", nil)

	err := <-errCh
	if !errors.Is(err, tui.ErrPanicRecovered) {
		t.Fatalf("expected ErrPanicRecovered, got: %v", err)
	}
	if !strings.Contains(err.Error(), "simulated host handler explosion") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}
}

func TestInputFilterDropsAndTransformsEvents(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 5)

	const src = `
local ui = require("ui")
return {
	view = ui.input{ id = "inp", bind = "val" },
}
`
	var finalVal string
	ready := make(chan struct{})
	p := tui.NewProgram(
		tui.WithSource(src),
		tui.WithScreen(sim),
		tui.WithInputFilter(func(ev tcell.Event) tcell.Event {
			if k, ok := ev.(*tcell.EventKey); ok && k.Key() == tcell.KeyRune {
				if k.Rune() == 'x' {
					return nil
				}
				if k.Rune() == 'a' {
					return tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone)
				}
			}
			return ev
		}),
		tui.WithStateObserver(func(key string, _, newVal any) {
			if key == "val" {
				finalVal, _ = newVal.(string)
			}
		}),
		tui.WithOnReady(func(_ *tui.Runtime) { close(ready) }),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()
	<-ready

	sim.InjectKey(tcell.KeyRune, 'a', tcell.ModNone)
	sim.InjectKey(tcell.KeyRune, 'x', tcell.ModNone)
	sim.InjectKey(tcell.KeyRune, 'b', tcell.ModNone)
	time.Sleep(30 * time.Millisecond)
	p.Quit()

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if finalVal != "Ab" {
		t.Fatalf("expected filtered input 'Ab', got %q", finalVal)
	}
}

func TestClipboardCopyCutPasteAndOSC52(t *testing.T) {
	var oscBuf bytes.Buffer
	clip := tui.NewOSC52Clipboard(&oscBuf)

	r, err := tui.Load(`
local ui = require("ui")
return {
	view = ui.input{ id = "inp", bind = "text", text = "hello world" },
	actions = {
		set_clip = function(p) ui.clipboard_set(p.val) end,
		read_clip = function() ui.set("from_lua", ui.clipboard_get()) end,
	},
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.SetClipboard(clip)

	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)); err != nil {
		t.Fatal(err)
	}
	got, err := clip.Read()
	if err != nil || got != "hello world" {
		t.Fatalf("expected clipboard 'hello world', got %q (%v)", got, err)
	}
	expectedOSC := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("hello world")) + "\x07"
	if oscBuf.String() != expectedOSC {
		t.Fatalf("expected OSC52 sequence %q, got %q", expectedOSC, oscBuf.String())
	}

	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl)); err != nil {
		t.Fatal(err)
	}
	if r.State("inp").Value != "" {
		t.Fatalf("expected empty input after Ctrl+X, got %q", r.State("inp").Value)
	}

	if err := r.Action("set_clip", map[string]any{"val": "pasted via lua"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl)); err != nil {
		t.Fatal(err)
	}
	if r.State("inp").Value != "pasted via lua" {
		t.Fatalf("expected 'pasted via lua' after Ctrl+V, got %q", r.State("inp").Value)
	}
	if err := r.Action("read_clip", nil); err != nil {
		t.Fatal(err)
	}
	if r.Model("from_lua") != "pasted via lua" {
		t.Fatalf("expected Lua clipboard_get 'pasted via lua', got %v", r.Model("from_lua"))
	}
}

func TestInlineModeAndDiffRendererConsistency(t *testing.T) {
	r, err := tui.Load(`
local ui = require("ui")
return {
	view = ui.column{
		id = "root",
		ui.text{ id = "t1", text = "Line 1" },
		ui.text{ id = "t2", text = "Line 2" },
		ui.text{ id = "t3", text = "Line 3" },
		ui.text{ id = "t4", text = "Line 4" },
	},
}
`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	r.SetInline(true, 2)
	if inline, h := r.Inline(); !inline || h != 2 || r.AltScreen() {
		t.Fatalf("unexpected inline state: inline=%v h=%d alt=%v", inline, h, r.AltScreen())
	}

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(20, 10)

	r.Draw(sim, "")
	c0, _, _, _ := sim.GetContent(0, 0)
	c2, _, _, _ := sim.GetContent(0, 2)
	if c0 != 'L' {
		t.Fatalf("expected 'L' at (0,0), got %q", c0)
	}
	if c2 == 'L' {
		t.Fatalf("expected row 2 to be clipped by InlineHeight=2, got %q", c2)
	}

	if err := r.Patch("t1", map[string]any{"text": "Updated"}); err != nil {
		t.Fatal(err)
	}
	r.Draw(sim, "")
	u0, _, _, _ := sim.GetContent(0, 0)
	if u0 != 'U' {
		t.Fatalf("expected 'U' at (0,0) after diff draw, got %q", u0)
	}
}

func TestWithHandlersIsolatesCallerMap(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 6)

	const src = `
local ui = require("ui")
return {
	view = ui.text{ id = "t", text = "ok" },
	actions = {
		trigger = function()
			host.dispatch("ping", {})
			host.dispatch("injected", {})
		end,
	},
}
`
	var origCalls, mutatedCalls, injectedCalls atomic.Int32
	handlers := map[string]tui.ActionHandler{
		"ping": func(_ *tui.Runtime, _ any) error {
			origCalls.Add(1)
			return nil
		},
	}
	ready := make(chan struct{})
	opt := tui.WithHandlers(handlers)

	handlers["ping"] = func(_ *tui.Runtime, _ any) error {
		mutatedCalls.Add(1)
		return nil
	}
	handlers["injected"] = func(_ *tui.Runtime, _ any) error {
		injectedCalls.Add(1)
		return nil
	}

	p := tui.NewProgram(
		tui.WithSource(src),
		tui.WithScreen(sim),
		opt,
		tui.WithMiddleware(func(next tui.EventHandler) tui.EventHandler {
			return func(r *tui.Runtime, ev tui.HostEvent) error {
				return next(r, ev)
			}
		}),
		tui.WithOnReady(func(_ *tui.Runtime) { close(ready) }),
	)

	handlers["ping"] = func(_ *tui.Runtime, _ any) error {
		mutatedCalls.Add(1)
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()
	<-ready

	if err := p.Send("trigger", nil); err != nil {
		t.Fatalf("p.Send failed: %v", err)
	}
	p.Quit()

	if err := <-errCh; err != nil {
		t.Fatalf("Program.Run failed: %v", err)
	}
	if origCalls.Load() != 1 || mutatedCalls.Load() != 0 || injectedCalls.Load() != 0 {
		t.Fatalf("expected isolated handler snapshot (orig=1, mutated=0, injected=0), got orig=%d mutated=%d injected=%d",
			origCalls.Load(), mutatedCalls.Load(), injectedCalls.Load())
	}
}

func TestWithHandlersAndWithOnEventChainInAnyOrder(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	sim.SetSize(40, 6)

	const src = `
local ui = require("ui")
return {
	view = ui.text{ id = "t", text = "ok" },
	actions = {
		trigger = function()
			host.dispatch("ping", {})
			host.dispatch("fallback", {})
		end,
	},
}
`
	var pingCalls, fallbackCalls atomic.Int32
	ready := make(chan struct{})

	p := tui.NewProgram(
		tui.WithSource(src),
		tui.WithScreen(sim),
		tui.WithHandlers(map[string]tui.ActionHandler{
			"ping": func(_ *tui.Runtime, _ any) error {
				pingCalls.Add(1)
				return nil
			},
		}),
		tui.WithOnEvent(func(_ *tui.Runtime, ev tui.HostEvent) error {
			if ev.Action == "fallback" {
				fallbackCalls.Add(1)
			}
			return nil
		}),
		tui.WithMiddleware(func(next tui.EventHandler) tui.EventHandler {
			return func(r *tui.Runtime, ev tui.HostEvent) error {
				return next(r, ev)
			}
		}),
		tui.WithOnReady(func(_ *tui.Runtime) { close(ready) }),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run()
	}()
	<-ready

	if err := p.Send("trigger", nil); err != nil {
		t.Fatalf("p.Send failed: %v", err)
	}
	p.Quit()

	if err := <-errCh; err != nil {
		t.Fatalf("Program.Run failed: %v", err)
	}
	if pingCalls.Load() != 1 || fallbackCalls.Load() != 1 {
		t.Fatalf("expected both WithHandlers and WithOnEvent to run (ping=1, fallback=1), got ping=%d fallback=%d",
			pingCalls.Load(), fallbackCalls.Load())
	}
}
