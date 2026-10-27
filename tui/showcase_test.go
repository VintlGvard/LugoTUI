package tui_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

const showcaseSource = `
local ui = require('ui')

local function card(id, title, children)
 return ui.panel {id = id, title = ' ' .. title .. ' ', padding = 1, grow = 1, children = children}
end
local function button(id, label, action)
 return ui.button {id = id, text = label, on_click = action, height = 1, bg = '#283e4a'}
end

local function submit()
 local text = ui.get('prompt') or ''
 if text == '' then ui.set('status', 'Enter text first'); return end
 local id = (ui.get('request_id') or 0) + 1
 ui.set('request_id', id)
 ui.set('status', 'Request ' .. id .. ' running; send again to supersede it')
 host.dispatch('request.start', {request_id = id, text = text})
end

if ui.get('status') == nil then ui.set('status', 'Ready') end

return {
 api_version = 1,
 view = function()
  return ui.box {id = 'root',
   ui.column {padding = 1, gap = 1, grow = 1,
    ui.text {text = 'SHOWCASE · Ctrl+Q exit · F6 reload · F2 edit', height = 1, bold = true},
    card('request_card', 'Lua composition / model binding', {
     ui.input {id = 'prompt', bind = 'prompt', text = 'Hello from Lua', height = 1, on_submit = 'submit'},
     ui.text {bind = 'prompt', height = 1, fg = '#a3be8c'},
     ui.row {height = 1, gap = 2,
      button('send', 'Run async task', 'submit'),
      button('edit', 'Open dynamic dialog', 'edit')},
     ui.text {id = 'status', bind = 'status', text = 'Ready', height = 2}
    }),
    ui.panel {id = 'host_result', title = ' Go result ', height = 4,
     ui.text {text = 'Go will replace this subtree when the worker finishes.'}}
   },
   ui.box {id = 'dialog', hidden = true}
  }
 end,
 keymap = {F2 = 'edit'},
 actions = {
  submit = submit,
  edit = function()
   ui.set('dialog_text', ui.get('prompt') or '')
   ui.replace('dialog', ui.overlay {
    id = 'dialog', modal = true, position = 'absolute', left = '10%', top = '20%',
    width = '80%', height = 8, border = true, padding = 1, z_index = 10,
    title = ' New subtree; focus committed atomically ', bg = '#20303c',
    ui.input {id = 'dialog_input', bind = 'dialog_text', height = 1, on_submit = 'accept'},
    button('accept', 'Use text', 'accept')
   })
   ui.focus('dialog_input')
  end,
  accept = function()
   ui.set('prompt', ui.get('dialog_text') or '')
   ui.replace('dialog', ui.box {id = 'dialog', hidden = true})
   ui.focus('prompt')
  end,
  ['request.done'] = function(e)
   if e.request_id ~= ui.get('request_id') then return end
   ui.set('status', 'Request ' .. e.request_id .. ' complete')
   host.dispatch('result.ready', {text = e.text})
  end
 }
}
`

func showcaseOnEvent(r *tui.Runtime, event tui.HostEvent) error {
	if event.Action != "request.start" && event.Action != "result.ready" {
		return nil
	}
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: payload must be object", event.Action)
	}
	text, ok := payload["text"].(string)
	if !ok || len(text) > 4096 {
		return fmt.Errorf("%s: text must be string <= 4096 bytes", event.Action)
	}
	if event.Action == "result.ready" {
		return r.Replace("host_result", &tui.Node{
			Type: "panel", ID: "host_result", Props: map[string]any{"title": " Go result ", "height": 4.0},
			Children: []*tui.Node{{Type: "text", ID: "host_text", Props: map[string]any{"text": text}}},
		})
	}
	id, ok := payload["request_id"].(float64)
	if !ok || id < 1 || id > 1<<53 || id != float64(int64(id)) {
		return fmt.Errorf("request.start: invalid request_id")
	}
	if err := r.Patch("host_result", map[string]any{"title": " Go worker running "}); err != nil {
		return err
	}
	_, err := r.StartTask("request.done", func(ctx context.Context) (map[string]any, error) {
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return map[string]any{"request_id": id, "text": strings.ToUpper(text)}, nil
		}
	})
	return err
}

func TestShowcase(t *testing.T) {
	r, err := tui.Load(showcaseSource)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)
	r.Draw(s, "")
	click := func(id string) {
		t.Helper()
		if err := r.SetFocus(id); err != nil {
			t.Fatal(err)
		}
		if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
			t.Fatal(err)
		}
	}
	dispatch := func() {
		t.Helper()
		for _, event := range r.DrainEvents() {
			if err := showcaseOnEvent(r, event); err != nil {
				t.Fatal(err)
			}
		}
	}
	click("edit")
	if r.Focus() != "dialog_input" || r.Node("dialog_input") == nil {
		t.Fatal("transactional focus", r.Focus())
	}
	if err := r.Action("accept", nil); err != nil {
		t.Fatal(err)
	}
	if r.Focus() != "prompt" || r.Node("dialog_input") != nil {
		t.Fatal("dialog not replaced")
	}
	click("send")
	dispatch()
	if r.Node("host_result").Props["title"] != " Go worker running " {
		t.Fatal("Go Patch not applied")
	}
	if err := r.SetFocus("prompt"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlA, 0, 0)); err != nil {
		t.Fatal(err)
	}
	r.Paste("latest request")
	click("send")
	dispatch()
	deadline := time.After(3 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case result := <-r.TaskResults():
			if err := r.HandleTask(result); err != nil {
				t.Fatal(err)
			}
			dispatch()
		case <-deadline:
			t.Fatal("worker timeout")
		}
	}
	r.Draw(s, "")
	if r.Node("host_text") == nil || r.Node("host_text").Props["text"] != "LATEST REQUEST" {
		t.Fatal("Go Replace did not show latest result")
	}
	if !strings.Contains(r.State("status").Value, "complete") {
		t.Fatal("bound task status", r.State("status").Value)
	}
	if err := r.Action("request.done", map[string]any{"request_id": float64(1), "text": "stale"}); err != nil {
		t.Fatal(err)
	}
	if len(r.DrainEvents()) != 0 || r.Node("host_text").Props["text"] != "LATEST REQUEST" {
		t.Fatal("stale request escaped")
	}
}

func TestShowcaseInvalidEvent(t *testing.T) {
	for _, payload := range []any{nil, map[string]any{"text": true}, map[string]any{"text": "x", "request_id": "wrong"}} {
		if err := showcaseOnEvent(nil, tui.HostEvent{Action: "request.start", Payload: payload}); err == nil {
			t.Fatal("invalid event accepted")
		}
	}
}
