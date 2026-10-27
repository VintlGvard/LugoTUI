package tui_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestPublicLifecycle(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(120, 40)
	r.Draw(s, "")
	if e = r.Action("agent.text", map[string]any{"text": "hello"}); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Snapshot(); e != nil {
		t.Fatal(e)
	}
	next, e := r.Reload(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	next.Close()
	next.Close()
	if _, e = next.Reload(tui.Demo()); e == nil {
		t.Fatal("closed reload")
	}
}

func TestPublicPatchReplace(t *testing.T) {
	r, err := tui.Load(`return {view=ui.column{id='root',ui.text{id='label',text='old'}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Patch("label", map[string]any{"text": "new"}); err != nil {
		t.Fatal(err)
	}
	if r.Node("label").Props["text"] != "new" {
		t.Fatal("patch not applied")
	}
	root, revision := rootNode(t, r), r.Revision()
	if err := r.Patch("label", map[string]any{"width": -1.0}); err == nil {
		t.Fatal("invalid patch accepted")
	}
	if !reflect.DeepEqual(rootNode(t, r), root) || r.Revision() != revision {
		t.Fatal("patch not atomic")
	}
	subtree := &tui.Node{Type: "input", ID: "editor", Props: map[string]any{"text": "hello"}}
	if err := r.Replace("label", subtree); err != nil {
		t.Fatal(err)
	}
	subtree.Props["text"] = "caller mutation"
	if r.Node("editor").Props["text"] != "hello" || r.Node("label") != nil {
		t.Fatal("replacement not cloned")
	}
	root, revision = rootNode(t, r), r.Revision()
	for _, bad := range []*tui.Node{nil, {Type: "box", ID: "bad", Children: []*tui.Node{nil}}} {
		if err := r.Replace("editor", bad); err == nil {
			t.Fatal("invalid replacement accepted")
		}
		if !reflect.DeepEqual(rootNode(t, r), root) || r.Revision() != revision {
			t.Fatal("replace not atomic")
		}
	}
	cycle := &tui.Node{Type: "box", ID: "cycle"}
	cycle.Children = []*tui.Node{cycle}
	if err := r.Replace("editor", cycle); err == nil {
		t.Fatal("cyclic replacement accepted")
	}
	r.Close()
	if err := r.Patch("editor", nil); err == nil {
		t.Fatal("closed patch")
	}
	if err := r.Replace("editor", subtree); err == nil {
		t.Fatal("closed replace")
	}
}

func TestOperationValidationContext(t *testing.T) {
	r, err := tui.Load(`return {view=ui.column{id='root',ui.text{id='first'},ui.button{id='target'}},actions={bad=function()ui.patch('first',{text='ok'});ui.patch('target',{align='middle'})end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	check := func(err error, parts ...string) {
		t.Helper()
		if err == nil {
			t.Fatal("expected error")
		}
		for _, part := range parts {
			if !strings.Contains(err.Error(), part) {
				t.Fatalf("%q missing %q", err, part)
			}
		}
	}
	check(r.Action("bad", nil), "operations[2]", "target", "button", "property", "align")
	check(r.Replace("first", &tui.Node{Type: "text", ID: "target"}), "operations[1]", "duplicate id")
	packet := func(op string) []byte {
		return []byte(fmt.Sprintf(`{"revision":%d,"operations":[{"op":"patch","id":"first","props":{"text":"ok"}},%s]}`, r.Revision(), op))
	}
	check(r.ApplyJSON(packet(`{"op":"patch","id":"target","props":{"on_click":"missing"}}`)), "operations[2]", "target", "button", "on_click", "unknown action")
	check(r.ApplyJSON(packet(`{"op":"replace","id":"target","tree":{"type":"text","id":"replacement","props":{"width":false}}}`)), "operations[2]", "replacement", "text", "width")
	check(r.ApplyJSON(packet(`{"op":"replace","id":"target","tree":{"type":42}}`)), "operations[2].tree")
	check(r.ApplyJSON(packet(`{"op":"unknown","id":"target"}`)), "operations[2]", "unknown operation")
	if r.Node("first").Props["text"] != nil {
		t.Fatal("invalid operations committed")
	}
	if err := r.ApplyJSON([]byte(fmt.Sprintf(`{"revision":%d,"operations":[{"op":"replace","id":"target","tree":{"type":"button","id":"target","props":{"align":"middle"}}},{"op":"patch","id":"target","props":{"align":"center"}}]}`, r.Revision()))); err != nil {
		t.Fatal("final-tree validation regressed", err)
	}
}

func TestModalStagedFocusPreservesReturn(t *testing.T) {
	r, err := tui.Load(`return {view=ui.box{ui.input{id='first'},ui.input{id='second'},ui.overlay{id='modal',modal=true,hidden=true,ui.input{id='inside'},ui.input{id='other'}}},actions={open=function()ui.patch('modal',{hidden=false})end,focus=function()ui.focus('other')end,close=function()ui.patch('modal',{hidden=true})end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	setFocus(t, r, "second")
	for _, action := range []string{"open", "focus", "close"} {
		if err := r.Action(action, nil); err != nil {
			t.Fatal(err)
		}
		r.ReconcileFocus()
	}
	if r.Focus() != "second" {
		t.Fatal("modal return focus", r.Focus())
	}
}

func TestTransactionalFocus(t *testing.T) {
	const source = `return {view=ui.column{id='root',ui.input{id='old'},ui.box{id='slot'}},actions={
 create=function()ui.replace('slot',ui.box{id='slot',ui.input{id='new'}});ui.focus('new')end,
 before=function()ui.focus('next');ui.replace('slot',ui.box{id='slot',ui.input{id='next'}})end,
 only=function()ui.focus('old')end,
 missing=function()ui.set('bad',true);host.dispatch('bad',{});ui.patch('old',{text='bad'});ui.focus('absent')end,
 disabled=function()ui.patch('old',{disabled=true});ui.focus('old')end,
 hidden=function()ui.replace('slot',ui.box{id='slot',hidden=true,ui.input{id='hidden'}});ui.focus('hidden')end,
 removed=function()ui.focus('old');ui.replace('old',ui.text{id='old'})end,
 fail=function()ui.focus('old');error('fail')end}}
 `
	r, err := tui.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, tc := range []struct{ action, focus string }{{"create", "new"}, {"before", "next"}, {"only", "old"}, {"before", "next"}} {
		if err := r.Action(tc.action, nil); err != nil {
			t.Fatal(tc.action, err)
		}
		if r.Focus() != tc.focus {
			t.Fatal(tc.action, r.Focus())
		}
	}
	for _, action := range []string{"missing", "disabled", "hidden", "removed", "fail"} {
		root, revision, focus, events := rootNode(t, r), r.Revision(), r.Focus(), len(r.DrainEvents())
		if err := r.Action(action, nil); err == nil {
			t.Fatal("expected failure", action)
		}
		if !reflect.DeepEqual(rootNode(t, r), root) || r.Revision() != revision || r.Focus() != focus || len(r.DrainEvents()) != events || r.Model("bad") != nil {
			t.Fatal("rollback", action)
		}
		if err := r.Action("only", nil); err != nil || r.Focus() != "old" {
			t.Fatal("stale focus", err, r.Focus())
		}
	}
	if candidate, err := tui.Load(`ui.focus('initial');return {view=ui.input{id='initial'}}`); err == nil {
		candidate.Close()
		t.Fatal("initialization mutation accepted")
	}
}
