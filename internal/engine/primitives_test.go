package engine

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	lua "github.com/yuin/gopher-lua"
)

func primitiveTestRuntime(kind string, props map[string]any) (*Runtime, *Node) {
	n := &Node{Type: kind, ID: "primitive", Props: props}
	return &Runtime{Root: n, Nodes: map[string]*Node{n.ID: n}, State: map[string]*WidgetState{}}, n
}

func TestPrimitiveValidation(t *testing.T) {
	tests := []struct {
		kind, key string
		value     any
	}{
		{"spinner", "frames", []any{}}, {"spinner", "frames", []any{"x\ny"}}, {"spinner", "interval", float64(0)}, {"spinner", "preset", "missing"},
		{"progress", "percent", math.NaN()}, {"progress", "percent", 1.1}, {"progress", "fill", "界"}, {"progress", "empty", "ab"}, {"progress", "gradient", []any{"red"}},
		{"paginator", "per_page", float64(0)}, {"paginator", "page", 1.5}, {"paginator", "mode", "bad"}, {"timer", "duration", float64(-1)},
		{"stopwatch", "running", "yes"}, {"help", "keys", []any{map[string]any{"keys": []any{}}}}, {"help", "full", float64(1)}, {"timer", "frames", []any{"a"}}, {"text", "percent", 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.key, func(t *testing.T) {
			_, n := primitiveTestRuntime(tt.kind, map[string]any{tt.key: tt.value})
			if validatePrimitive(n) == nil {
				t.Fatal("invalid property accepted")
			}
		})
	}
	for _, kind := range []string{"spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor"} {
		_, n := primitiveTestRuntime(kind, map[string]any{})
		if err := validatePrimitive(n); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCursorComponent(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	r, n := primitiveTestRuntime("cursor", map[string]any{"mode": "blink", "blink_speed": float64(500)})
	if !r.NeedsTick() {
		t.Fatal("cursor not scheduled")
	}
	p := r.primitive(n)
	if p.CursorBlinked {
		t.Fatal("initially blinked")
	}
	if err := r.Tick(epoch); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(epoch.Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if !p.CursorBlinked {
		t.Fatal("cursor did not blink")
	}
}

func TestTickInvalidatesVisibleChanges(t *testing.T) {
	for _, kind := range []string{"cursor", "input", "textarea", "spinner", "timer", "stopwatch", "progress"} {
		t.Run(kind, func(t *testing.T) {
			r, n := primitiveTestRuntime(kind, map[string]any{"duration": float64(2000), "interval": float64(1000)})
			r.Focus = n.ID
			r.primitive(n)
			if kind == "progress" {
				n.Props["percent"] = float64(1)
			}
			epoch := time.Unix(0, 0)
			check := func(offset time.Duration, changed bool) {
				t.Helper()
				before := r.AnimationRevision
				if err := r.Tick(epoch.Add(offset)); err != nil {
					t.Fatal(err)
				}
				if r.Revision != 0 {
					t.Fatal("animation changed protocol revision")
				}
				if got := r.AnimationRevision != before; got != changed {
					t.Fatalf("at %v: redraw = %v, want %v", offset, got, changed)
				}
			}
			check(0, false)
			check(16*time.Millisecond, kind == "progress")
			check(time.Second, true)
			check(time.Second, false)
		})
	}
}

func TestTickSkipsUnchangedFrames(t *testing.T) {
	for _, kind := range []string{"cursor", "input", "textarea", "spinner"} {
		t.Run(kind, func(t *testing.T) {
			r, n := primitiveTestRuntime(kind, map[string]any{
				"blink_speed": float64(500), "cursor_blink_speed": float64(500),
				"interval": float64(100), "frames": []any{"x", "y"},
			})
			r.Focus = n.ID
			epoch := time.Unix(0, 0)
			for _, offset := range []time.Duration{0, time.Second} {
				if err := r.Tick(epoch.Add(offset)); err != nil {
					t.Fatal(err)
				}
			}
			if r.AnimationRevision != 0 {
				t.Fatal("full animation cycles requested redraw")
			}
		})
	}
}

func TestSpinnerPresets(t *testing.T) {
	for _, preset := range []string{"line", "dot", "mini_dot", "jump", "pulse", "points", "globe", "moon", "monkey", "meter", "hamburger", "ellipsis"} {
		_, n := primitiveTestRuntime("spinner", map[string]any{"preset": preset})
		frames := primitiveFrames(n)
		if len(frames) == 0 {
			t.Fatalf("preset %s has no frames", preset)
		}
	}
}

func TestPrimitiveClocks(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	r, n := primitiveTestRuntime("timer", map[string]any{"duration": float64(1000)})
	if !r.NeedsTick() {
		t.Fatal("timer not scheduled")
	}
	if err := r.Tick(epoch); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(epoch.Add(400 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if got := r.primitive(n).Elapsed; got != 400*time.Millisecond {
		t.Fatalf("elapsed %v", got)
	}
	handled, err := r.keyPrimitive(n, tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !handled || err != nil {
		t.Fatal(handled, err)
	}
	r.Tick(epoch.Add(time.Hour))
	if r.primitive(n).Elapsed != 400*time.Millisecond {
		t.Fatal("paused clock advanced")
	}
	r.keyPrimitive(n, tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	r.Tick(epoch.Add(2 * time.Hour))
	r.Tick(epoch.Add(2*time.Hour + 600*time.Millisecond))
	if r.NeedsTick() || !r.primitive(n).TimedOut {
		t.Fatal("timer did not finish")
	}
	n.Props["reset_key"] = float64(1)
	if !r.NeedsTick() || r.primitive(n).Elapsed != 0 {
		t.Fatal("reset failed")
	}
	n.Props["duration"] = float64(2000)
	r.Tick(epoch)
	r.Tick(epoch.Add(time.Second))
	if r.primitive(n).Elapsed != time.Second {
		t.Fatal("duration reset failed")
	}
	r, n = primitiveTestRuntime("stopwatch", map[string]any{})
	r.Tick(epoch)
	r.Tick(epoch.Add(1500 * time.Millisecond))
	r.Tick(epoch.Add(time.Second))
	if r.primitive(n).Elapsed != 1500*time.Millisecond {
		t.Fatal("clock moved backwards")
	}
	r.keyPrimitive(n, tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if r.primitive(n).Elapsed != 0 {
		t.Fatal("stopwatch reset failed")
	}
}

func TestPrimitiveZeroEpochAndVisibility(t *testing.T) {
	r, n := primitiveTestRuntime("stopwatch", map[string]any{})
	if err := r.Tick(time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(time.Time{}.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if r.primitive(n).Elapsed != time.Second {
		t.Fatal("zero epoch lost")
	}
	n.Props["hidden"] = true
	if r.NeedsTick() {
		t.Fatal("hidden widget scheduled")
	}
}

func TestPrimitiveTimeoutCallbackOnce(t *testing.T) {
	r, n := primitiveTestRuntime("timer", map[string]any{"duration": float64(10), "on_timeout": "done"})
	r.L = lua.NewState()
	defer r.Close()
	r.Data = map[string]any{}
	r.actions = r.L.NewTable()
	count := 0
	r.actions.RawSetString("done", r.L.NewFunction(func(L *lua.LState) int { count++; return 0 }))
	epoch := time.Now()
	if err := r.Tick(epoch); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(epoch.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	r.Tick(epoch.Add(2 * time.Second))
	if count != 1 || !r.primitive(n).TimedOut {
		t.Fatalf("count %d", count)
	}
}

func TestPrimitiveSpinnerProgressPagination(t *testing.T) {
	epoch := time.Now()
	r, n := primitiveTestRuntime("spinner", map[string]any{"frames": []any{"a", "b", "c"}, "interval": float64(20)})
	r.Tick(epoch)
	r.Tick(epoch.Add(100 * time.Millisecond))
	if r.primitive(n).Frame != 2 {
		t.Fatal("frame interval mismatch")
	}
	r, n = primitiveTestRuntime("progress", map[string]any{"percent": float64(1), "duration": float64(100)})
	r.Tick(epoch)
	r.Tick(epoch.Add(50 * time.Millisecond))
	if r.primitive(n).Percent != 0.5 {
		t.Fatal("progress not animated")
	}
	n.Props["percent"] = float64(0)
	r.NeedsTick()
	r.Tick(epoch.Add(100 * time.Millisecond))
	if r.primitive(n).Percent != 0.25 {
		t.Fatal("progress did not retarget")
	}
	r.Tick(epoch.Add(150 * time.Millisecond))
	if r.NeedsTick() {
		t.Fatal("progress kept ticking")
	}
	n.Props["animated"] = false
	n.Props["percent"] = 0.7
	if r.primitive(n).Percent != 0.7 {
		t.Fatal("static progress wrong")
	}
	r, n = primitiveTestRuntime("paginator", map[string]any{"total": float64(25), "per_page": float64(10)})
	for i := 0; i < 5; i++ {
		r.keyPrimitive(n, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if r.primitive(n).Page != 2 {
		t.Fatal("page not clamped")
	}
	n.Props["total"] = float64(1)
	if r.primitive(n).Page != 0 {
		t.Fatal("total change not clamped")
	}
	copy := clonePrimitiveState(r.primitive(n))
	copy.Page = 5
	if r.primitive(n).Page != 0 {
		t.Fatal("cloned state aliases")
	}
}

func TestPrimitiveDotsKeepActiveVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 1)
	for _, clip := range []Rect{{X: 0, Y: 0, W: 3, H: 1}, {X: 2, Y: 0, W: 1, H: 1}} {
		for _, page := range []float64{0, 1, 50, 99} {
			screen.Clear()
			r, n := primitiveTestRuntime("paginator", map[string]any{"mode": "dots", "page": page, "per_page": float64(1), "total": float64(100)})
			r.paintPrimitive(screen, n, tcell.StyleDefault, Rect{X: 0, Y: 0, W: 3, H: 1}, clip)
			active := 0
			for x := clip.X; x < clip.X+clip.W; x++ {
				glyph, _, _, _ := screen.GetContent(x, 0)
				if glyph == '●' {
					active++
				}
			}
			if active != 1 {
				t.Fatalf("page %v clip %+v active dots %d", page, clip, active)
			}
		}
	}
}

func TestPrimitivePaintAndBindings(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 5)
	r, n := primitiveTestRuntime("progress", map[string]any{"percent": float64(0.5), "animated": false, "show_percentage": false, "fill": "#", "empty": ".", "gradient": []any{"red", "blue"}})
	if !r.paintPrimitive(screen, n, tcell.StyleDefault, Rect{X: 0, Y: 0, W: 10, H: 1}, Rect{X: 2, Y: 0, W: 5, H: 1}) {
		t.Fatal("not painted")
	}
	for x := 2; x < 7; x++ {
		got, _, _, _ := screen.GetContent(x, 0)
		want := '#'
		if x >= 5 {
			want = '.'
		}
		if got != want {
			t.Fatalf("cell %d=%q want %q", x, got, want)
		}
	}
	r, n = primitiveTestRuntime("help", map[string]any{"keys": []any{map[string]any{"keys": []any{"x", "Ctrl+C"}, "help": "exit", "action": "exit", "enabled": false}}})
	r.L = lua.NewState()
	defer r.Close()
	r.Data = map[string]any{}
	r.actions = r.L.NewTable()
	count := 0
	r.actions.RawSetString("exit", r.L.NewFunction(func(L *lua.LState) int { count++; return 0 }))
	if handled, _ := r.bindingKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); handled {
		t.Fatal("disabled binding fired")
	}
	n.Props["keys"].([]any)[0].(map[string]any)["enabled"] = true
	if handled, err := r.bindingKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); !handled || err != nil || count != 1 {
		t.Fatal(handled, err, count)
	}
	if handled, err := r.keyPrimitive(n, tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone)); !handled || err != nil || !r.primitive(n).Full {
		t.Fatal("help toggle failed")
	}
	if !primitiveKeyMatches("ctrl+c", tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)) {
		t.Fatal("control key mismatch")
	}
}

func TestTimerStopwatchPropagateAndRollbackOnChangeError(t *testing.T) {
	r, err := Load(`return {
  view = {type = "stopwatch", id = "sw", running = false, on_change = "boom"},
  actions = {boom = function() error("clock_boom") end}
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.Focus = "sw"
	err = r.Key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if err == nil || !strings.Contains(err.Error(), "clock_boom") {
		t.Fatalf("expected clock_boom error from stopwatch on_change, got %v", err)
	}
	if r.primitive(r.Nodes["sw"]).Running {
		t.Fatal("stopwatch running state was not rolled back after failed on_change")
	}
}

func TestTimerStopwatchPausePersistsAcrossDrawAndTick(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 5)

	r, err := Load(`return {
  view = {
    type = "column",
    id = "root",
    { type = "timer", id = "tm", duration = 5000, running = true },
    { type = "stopwatch", id = "sw", running = true }
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	epoch := time.Now()
	for _, id := range []string{"tm", "sw"} {
		if err := r.SetFocus(id); err != nil {
			t.Fatal(err)
		}
		if err := r.Key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)); err != nil {
			t.Fatal(err)
		}
		r.Draw(screen, "")
		if r.primitive(r.Nodes[id]).Running {
			t.Fatalf("%s did not remain paused after Draw", id)
		}
	}
	if r.NeedsTick() {
		t.Fatal("NeedsTick should be false when both timer and stopwatch are paused")
	}
	if err := r.Tick(epoch); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(epoch.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if r.primitive(r.Nodes["tm"]).Elapsed != 0 || r.primitive(r.Nodes["sw"]).Elapsed != 0 {
		t.Fatal("paused clocks advanced during Tick after Draw")
	}
}

func TestProgressPercentFormatValidationAndHelpToggle(t *testing.T) {
	for _, badFmt := range []string{"%999999999.0f", "%100f", "%.100f", "%s", "%f %f", "%*[1]f", "%"} {
		_, n := primitiveTestRuntime("progress", map[string]any{"percent_format": badFmt})
		if validatePrimitive(n) == nil {
			t.Fatalf("unsafe percent_format %q was accepted", badFmt)
		}
	}
	_, okNode := primitiveTestRuntime("progress", map[string]any{"percent_format": " %3.0f%%"})
	if err := validatePrimitive(okNode); err != nil {
		t.Fatalf("valid percent_format rejected: %v", err)
	}

	r, err := Load(`return {
  view = {
    type = "column",
    id = "root",
    { type = "help", id = "h", keys = { { keys = {"Space"}, help = "go", action = "go" }, { keys = {"?"}, help = "qmark", action = "qm" } } },
    { type = "text", id = "after", text = "tail" }
  },
  actions = {
    go = function() host.dispatch("space_act", {}) end,
    qm = function() host.dispatch("qmark_act", {}) end
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Key(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)); err != nil {
		t.Fatal(err)
	}
	if ev := r.DrainEvents(); len(ev) != 1 || ev[0].Action != "space_act" {
		t.Fatalf("expected space_act from help binding, got %+v", ev)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 10)
	r.Draw(screen, "")
	yBefore := r.Nodes["after"].Rect.Y
	if handled, err := r.keyPrimitive(r.Nodes["h"], tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone)); !handled || err != nil {
		t.Fatalf("help ? toggle failed: %v %v", handled, err)
	}
	r.Draw(screen, "")
	if r.Nodes["after"].Rect.Y <= yBefore {
		t.Fatalf("help layout did not expand after ? toggle: before=%d after=%d", yBefore, r.Nodes["after"].Rect.Y)
	}
}
