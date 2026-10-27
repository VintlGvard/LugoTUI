package tui_test

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestReloadAndRollback(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	setFocus(t, r, "prompt")
	pasteInto(t, r, "prompt", "界🙂 draft")
	r.Key(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	s := r.State("prompt")
	next, e := r.Reload(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	if next.State("prompt").Value != s.Value || next.State("prompt").Cursor != 2 || next.Focus() != "prompt" {
		t.Fatal("state lost")
	}
	if e = r.ApplyJSON([]byte(`{"revision":0,"operations":[{"op":"patch","id":"answer","props":{"text":"changed"}},{"op":"patch","id":"absent","props":{"text":"bad"}}]}`)); e == nil {
		t.Fatal("invalid patch accepted")
	}
	if str(r.Node("answer"), "text", "") == "changed" {
		t.Fatal("partial commit")
	}
	if _, e = r.Reload("while true do end"); e == nil {
		t.Fatal("timeout absent")
	}
}

func TestUnicodeInputAndSimulation(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if e = s.Init(); e != nil {
		t.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(80, 24)
	r.Draw(s, "")
	setFocus(t, r, "prompt")
	for _, c := range "a界" {
		if e = r.Key(tcell.NewEventKey(tcell.KeyRune, c, 0)); e != nil {
			t.Fatal(e)
		}
	}
	r.Key(tcell.NewEventKey(tcell.KeyBackspace2, 0, 0))
	if r.State("prompt").Value != "a" {
		t.Fatal("grapheme deletion")
	}
	r.Draw(s, "")
}

func TestValidation(t *testing.T) {
	for _, source := range []string{`return {view={type='unknown'}}`, `return {view={type='row',{type='input',id='x'},{type='input',id='x'}}}`, `return {view={type='text',width=-1}}`, `return {view={type='text',surprise=true}}`} {
		if r, e := tui.Load(source); e == nil {
			r.Close()
			t.Fatal("invalid tree accepted", source)
		}
	}
}

func TestAgentPacket(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e = r.ApplyJSON([]byte(`{"revision":0,"operations":[{"op":"patch","id":"answer","props":{"text":"agent"}}]}`)); e != nil {
		t.Fatal(e)
	}
	if str(r.Node("answer"), "text", "") != "agent" {
		t.Fatal("patch not applied")
	}
	if e = r.ApplyJSON([]byte(`{"revision":0,"operations":[]}`)); e == nil {
		t.Fatal("stale revision accepted")
	}
	if e = r.ApplyJSON([]byte(`{"revision":1,"operations":[{"op":"replace","id":"answer","tree":{"type":"button","id":"new","props":{"text":"Click"}}}]}`)); e != nil {
		t.Fatal(e)
	}
	if r.Node("new") == nil || r.Node("answer") != nil {
		t.Fatal("replace failed")
	}
}

func TestScroll(t *testing.T) {
	r, e := tui.Load(`return {view={type='scroll',id='scroll',{type='text',id='long',height=20,text='top'}}}`)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if e = s.Init(); e != nil {
		t.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(20, 4)
	r.Draw(s, "")
	r.Mouse(tcell.NewEventMouse(1, 1, tcell.WheelDown, 0))
	r.Draw(s, "")
	if r.Node("long").Rect.Y != -3 || r.Node("long").Clip.H > 4 {
		t.Fatal("scroll clipping", r.Node("long").Rect, r.Node("long").Clip)
	}
}

func TestModelActionRollback(t *testing.T) {
	r := mouseFixture(t, `return {view={type='input',id='input',bind='draft'},actions={good=function()ui.set('draft','updated')end,bad=function()ui.set('draft','broken');ui.patch('absent',{text='bad'})end}}`)
	if e := r.Action("good", nil); e != nil {
		t.Fatal(e)
	}
	if r.State("input").Value != "updated" {
		t.Fatal("binding")
	}
	if e := r.Action("bad", nil); e == nil {
		t.Fatal("invalid action")
	}
	if r.Model("draft") != "updated" || r.State("input").Value != "updated" {
		t.Fatal("rollback")
	}
}

func TestWorkspaceKeymapAndMessages(t *testing.T) {
	r := mouseFixture(t, tui.Demo())
	if e := r.Key(tcell.NewEventKey(tcell.KeyCtrlP, 0, 0)); e != nil {
		t.Fatal(e)
	}
	r.ReconcileFocus()
	if r.Focus() != "command_filter" {
		t.Fatal("palette", r.Focus())
	}
	if e := r.Action("command", map[string]any{"index": 2, "value": "Focus scratchpad"}); e != nil {
		t.Fatal(e)
	}
	r.ReconcileFocus()
	if r.Focus() != "notes" {
		t.Fatal("palette focus", r.Focus())
	}
	reply := r.AgentReply([]byte(`{"method":"message","action":"agent.text","payload":{"text":"Hello"}}`))
	if str(r.Node("answer"), "text", "") != "Hello" {
		t.Fatal(string(reply))
	}
}

func TestTasks(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	_, e = r.StartTask("agent.text", func(context.Context) (map[string]any, error) { return map[string]any{"text": "async"}, nil })
	if e != nil {
		t.Fatal(e)
	}
	var result tui.TaskResult
	select {
	case result = <-r.TaskResults():
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	if e = r.HandleTask(result); e != nil {
		t.Fatal(e)
	}
	if str(r.Node("answer"), "text", "") != "async" {
		t.Fatal("task not handled")
	}
	next, e := r.Reload(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	if e = next.HandleTask(result); e != nil {
		t.Fatal(e)
	}
	canceled := make(chan struct{})
	_, e = r.StartTask("agent.text", func(ctx context.Context) (map[string]any, error) {
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	})
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("not canceled")
	}
}

func TestRepeatedReloadResize(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { r.Close() }()
	screen := tcell.NewSimulationScreen("UTF-8")
	if e = screen.Init(); e != nil {
		t.Fatal(e)
	}
	defer screen.Fini()
	for i := 0; i < 50; i++ {
		screen.SetSize(1+i%80, 1+i%30)
		r.Draw(screen, "")
		next, e := r.Reload(tui.Demo())
		if e != nil {
			t.Fatal(e)
		}
		r.Close()
		r = next
	}
}

func BenchmarkTextareaCachedDraw(b *testing.B) {
	r, e := tui.Load(`return {view={type='textarea',id='edit'}}`)
	if e != nil {
		b.Fatal(e)
	}
	defer r.Close()
	if e = r.Paste(strings.Repeat("hello world\n", 10000)); e != nil {
		b.Fatal(e)
	}
	s := tcell.NewSimulationScreen("UTF-8")
	if e = s.Init(); e != nil {
		b.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	r.Draw(s, "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Draw(s, "")
	}
}

const fullModelSource = `
for i=1,1024 do ui.set("k"..i, "value") end
return {view=ui.column{id="root", children={
  ui.input{id="input", bind="k1"}, ui.button{id="button", text="button"}
}}, actions={overflow=function()
  ui.set("k1", "changed")
  ui.focus("button")
  host.dispatch("event", {})
  ui.patch("input", {bind="overflow"})
end}}
`

func TestLoadValidatesRestoredModel(t *testing.T) {
	source := `local children={}
for i=1,1025 do children[i]=ui.input{id="i"..i, bind="k"..i} end
return {view=ui.column{id="root", children=children}}`
	r, err := tui.Load(source)
	if r != nil {
		r.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "model exceeds 1024 keys") {
		t.Fatalf("Load error = %v, want model key limit", err)
	}
}

func TestTransactionModelLimitRollback(t *testing.T) {
	for _, method := range []string{"Patch", "Action", "Reload"} {
		t.Run(method, func(t *testing.T) {
			r, err := tui.Load(fullModelSource)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			rootBefore := rootNode(t, r)
			inputBefore := r.Node("input")
			stateBefore := r.State("input")
			focusBefore := r.Focus()
			revisionBefore := r.Revision()
			eventsBefore := r.DrainEvents()

			switch method {
			case "Patch":
				err = r.Patch("input", map[string]any{"bind": "overflow"})
			case "Action":
				err = r.Action("overflow", nil)
			case "Reload":
				var next *tui.Runtime
				next, err = r.Reload(`return {view=ui.input{id="input", bind="overflow"}}`)
				if next != nil {
					next.Close()
				}
			}
			if err == nil || !strings.Contains(err.Error(), "model exceeds 1024 keys") {
				t.Fatalf("error = %v, want model key limit", err)
			}
			if !reflect.DeepEqual(rootNode(t, r), rootBefore) || !reflect.DeepEqual(r.Node("input"), inputBefore) || !reflect.DeepEqual(r.State("input"), stateBefore) || r.State("input").Value != "value" {
				t.Fatal("failed transaction changed tree or widget state")
			}
			if r.Model("k1") != "value" || r.Model("overflow") != nil || r.Focus() != focusBefore || r.Revision() != revisionBefore || len(r.DrainEvents()) != 0 {
				t.Fatal("failed transaction changed model, focus, revision or events")
			}
			_ = eventsBefore
		})
	}
}

const conversionScript = `return {view={type='text',id='out',text='before'}, actions={
 echo=function(payload) host.dispatch('echo',payload) end,
 slots=function(payload) assert(payload.items[1]==nil and payload.items[2]=='last') end,
 get=function() host.dispatch('get',ui.get('bad')) end,
 patch=function() ui.patch('out',{text='ok'}); ui.patch('out',{text=function() end}) end,
 replace=function() ui.patch('out',{text='ok'}); ui.replace('out',{type='text',id=false}) end
}}`

func TestLuaPayloadConversion(t *testing.T) {
	r, err := tui.Load(conversionScript)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	payload := map[string]any{"items": []string{"a", "b"}, "number": int64(1 << 53)}
	if err := r.Action("echo", payload); err != nil {
		t.Fatal(err)
	}
	events := r.DrainEvents()
	if len(events) != 1 {
		t.Fatalf("events: %#v", events)
	}
	got := events[0].Payload.(map[string]any)
	if got["number"] != float64(1<<53) || got["items"].([]any)[1] != "b" {
		t.Fatalf("payload: %#v", got)
	}
	if err := r.Action("slots", map[string]any{"items": []any{nil, "last"}}); err != nil {
		t.Fatal(err)
	}
}

func TestLuaStrictConversionPaths(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, tc := range []struct {
		name    string
		value   any
		message string
	}{
		{"unknown", make(chan int), "unsupported Go type chan int"},
		{"integer", int64(1<<53) + 1, "integer exceeds exact Lua range"},
		{"negative integer", -int64(1<<53) - 1, "integer exceeds exact Lua range"},
		{"nan", math.NaN(), "nonfinite number"},
		{"infinity", math.Inf(1), "nonfinite number"},
		{"cycle", cycle, "value nesting exceeds 32"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := tui.Load(conversionScript)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			err = r.Action("echo", map[string]any{"items": []any{tc.value}})
			if err == nil || !strings.Contains(err.Error(), "payload.items[1]") || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestLuaNodeParserContext(t *testing.T) {
	for _, tc := range []struct{ view, message string }{
		{`{id='x'}`, `type must be string`},
		{`{type=12,id='x'}`, `type must be string`},
		{`{type='text',id=12}`, `id must be string`},
		{`{type='text',id=false}`, `id must be string`},
		{`{type='text',id='x',text=function() end}`, `node id="x" type="text" property "text"`},
		{`{type='column',id='root',children={{type=false}}}`, `children[1]`},
		{`{type='column',children={42}}`, `children[1]`},
		{`{type='column',children=false}`, `property children`},
		{`{type='column',children={oops={type='text'}}}`, `property children`},
	} {
		r, err := tui.Load("return {view=" + tc.view + "}")
		if err == nil {
			r.Close()
			t.Fatalf("accepted %s", tc.view)
		}
		if !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("%s: %v", tc.view, err)
		}
	}
	for _, action := range []string{"patch", "replace"} {
		r, err := tui.Load(conversionScript)
		if err != nil {
			t.Fatal(err)
		}
		err = r.Action(action, nil)
		r.Close()
		if err == nil || !strings.Contains(err.Error(), "operations[2]") {
			t.Fatalf("%s: %v", action, err)
		}
	}
}
