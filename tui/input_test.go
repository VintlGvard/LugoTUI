package tui_test

import (
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestMouseButtonSingleActivation(t *testing.T) {
	r := mouseFixture(t, `return {view={type='button',id='button',text='click',on_click='click'},actions={click=function() host.dispatch('click',{}) end}}`)
	for _, p := range [][2]int{{1, 0}, {2, 0}, {3, 0}} {
		if e := r.Mouse(tcell.NewEventMouse(p[0], p[1], tcell.Button1, 0)); e != nil {
			t.Fatal(e)
		}
	}
	r.Mouse(tcell.NewEventMouse(3, 0, tcell.ButtonNone, 0))
	events := r.DrainEvents()
	if len(events) != 1 {
		t.Fatalf("one press produced %d actions", len(events))
	}
}

func TestMouseListSelection(t *testing.T) {
	r := mouseFixture(t, `return {view={type='list',id='list',items={'one','two','three'},on_select='select'},actions={select=function(e) host.dispatch('select',e) end}}`)
	r.Mouse(tcell.NewEventMouse(0, 1, tcell.Button1, 0))
	if r.State("list").Selected != 1 || len(r.DrainEvents()) != 1 {
		t.Fatal("click did not dispatch selection")
	}
	r.Mouse(tcell.NewEventMouse(0, 1, tcell.ButtonNone, 0))
	r.Mouse(tcell.NewEventMouse(0, 1, tcell.WheelDown, 0))
	if r.State("list").Selected != 2 {
		t.Fatal("wheel")
	}
}

func TestMouseDragAndRelease(t *testing.T) {
	r := mouseFixture(t, `return {view={type='input',id='edit',text='abcdef'}}`)
	r.Mouse(tcell.NewEventMouse(1, 0, tcell.Button1, 0))
	r.Mouse(tcell.NewEventMouse(4, 0, tcell.Button1, 0))
	s := r.State("edit")
	if s.Anchor != 1 || s.Cursor != 4 {
		t.Fatal(s)
	}
	r.Mouse(tcell.NewEventMouse(4, 0, tcell.ButtonNone, 0))
	r.Key(tcell.NewEventKey(tcell.KeyRune, 'X', 0))
	s = r.State("edit")
	if s.Value != "aXef" {
		t.Fatal(s.Value)
	}
}

func TestMouseOpaqueOverlay(t *testing.T) {
	r := mouseFixture(t, `return {view={type='box',{type='button',id='behind',position='absolute',width=10,height=2,on_click='click'},{type='overlay',id='cover',position='absolute',width=10,height=2,z_index=1}},actions={click=function() host.dispatch('bad',{}) end}}`)
	r.Mouse(tcell.NewEventMouse(1, 0, tcell.Button1, 0))
	if len(r.DrainEvents()) != 0 {
		t.Fatal("click passed through overlay")
	}
}

func TestWheelOverScrollChild(t *testing.T) {
	r := mouseFixture(t, `return {view={type='scroll',id='viewport',{type='text',id='content',height=100,text='content'}}}`)
	r.Mouse(tcell.NewEventMouse(1, 1, tcell.WheelDown, 0))
	if r.State("viewport").Scroll != 3 {
		t.Fatal("child swallowed wheel")
	}
	r.Mouse(tcell.NewEventMouse(1, 1, tcell.WheelUp, 0))
	if r.State("viewport").Scroll != 0 {
		t.Fatal("wheel up")
	}
}

func TestMouseModalBlocksOutside(t *testing.T) {
	r := mouseFixture(t, `return {view={type='box',{type='button',id='outside',position='absolute',width=10,height=2},{type='overlay',id='modal',modal=true,position='absolute',left=20,width=10,height=3,{type='input',id='inside'}}}}`)
	r.Mouse(tcell.NewEventMouse(1, 0, tcell.Button1, 0))
	if r.Focus() != "inside" {
		t.Fatal("modal focus escaped", r.Focus())
	}
}

func TestModalPasteSpans(t *testing.T) {
	r, e := tui.Load(`return {view={type='column',{type='input',id='main'},{type='overlay',id='dialog',modal=true,hidden=true,{type='input',id='inside'}},{type='text',id='rich',spans={{text='red',fg='red'},{text=' plain'}}}}}`)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	screen := tcell.NewSimulationScreen("UTF-8")
	if e = screen.Init(); e != nil {
		t.Fatal(e)
	}
	defer screen.Fini()
	screen.SetSize(40, 10)
	r.Draw(screen, "")
	if e = r.Patch("dialog", map[string]any{"hidden": false}); e != nil {
		t.Fatal(e)
	}
	r.Draw(screen, "")
	if r.Focus() != "inside" {
		t.Fatal("modal focus", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	r.Draw(screen, "")
	if r.Focus() != "main" {
		t.Fatal("focus restore", r.Focus())
	}
	if e = r.Paste("hello\nworld"); e != nil {
		t.Fatal(e)
	}
	if r.State("main").Value != "hello world" {
		t.Fatal("paste")
	}
	r.Key(tcell.NewEventKey(tcell.KeyCtrlZ, 0, 0))
	if r.State("main").Value != "" {
		t.Fatal("paste undo")
	}
}

func TestHostEventQueue(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for i := 0; i < 2; i++ {
		if e = r.Action("send", map[string]any{"value": "hello"}); e != nil {
			t.Fatal(e)
		}
	}

	next, e := r.Reload(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	if len(next.DrainEvents()) != 2 {
		t.Fatal("reload lost event")
	}
}

func TestHiddenAncestorFocus(t *testing.T) {
	r, e := tui.Load(`return {view={type='column',{type='box',hidden=true,{type='input',id='hidden'}},{type='input',id='visible'}}}`)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if r.Focus() != "visible" {
		t.Fatal(r.Focus())
	}
}

func TestTextareaNavigation(t *testing.T) {
	r, e := tui.Load(`return {view={type='textarea',id='edit',text='abc\nx\nxyz'}}`)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 4)
	r.Draw(screen, "")
	r.Key(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	r.Key(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if r.State("edit").Cursor != 5 {
		t.Fatal(r.State("edit").Cursor)
	}
}

func TestJSONInputLimits(t *testing.T) {
	r, e := tui.Load(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e = r.ApplyJSON([]byte(`{"revision":0,"operations":[{"op":"patch","id":"answer","props":{"width":"20%garbage"}}]}`)); e == nil {
		t.Fatal("bad percentage")
	}
}

func TestListKeyboardAndEmpty(t *testing.T) {
	r := mouseFixture(t, `return {view={type='column',children={{type='list',id='list',height=2,items={'a','b','c','d'}}}}}`)
	r.Key(tcell.NewEventKey(tcell.KeyEnd, 0, 0))
	if r.State("list").Selected != 3 {
		t.Fatal("end")
	}
	r.Key(tcell.NewEventKey(tcell.KeyPgUp, 0, 0))
	if r.State("list").Selected != 1 {
		t.Fatal("page")
	}
	if err := r.Patch("list", map[string]any{"items": []any{}}); err != nil {
		t.Fatal(err)
	}
	if e := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); e != nil {
		t.Fatal(e)
	}
}

func TestActionValidation(t *testing.T) {
	for _, source := range []string{`return {view={type='button',id='b',on_click='missing'}}`, `return {view={type='text'},keymap={F1='missing'}}`} {
		r, e := tui.Load(source)
		if e == nil {
			r.Close()
			t.Fatal("unknown action accepted")
		}
	}
}

func TestModelLimits(t *testing.T) {

	r := mouseFixture(t, `return {view={type='text'},actions={big=function() ui.set('big',string.rep('x',1048576)) end}}`)
	if e := r.Action("big", nil); e == nil {
		t.Fatal("oversized action accepted")
	}
	if r.Model("big") != nil {
		t.Fatal("model not rolled back")
	}
}

func TestReloadModalFocus(t *testing.T) {
	r := mouseFixture(t, tui.Demo())
	setFocus(t, r, "prompt")
	r.Action("palette", nil)
	r.ReconcileFocus()
	next, e := r.Reload(tui.Demo())
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	next.Key(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	next.ReconcileFocus()
	if next.Focus() != "prompt" {
		t.Fatal("return focus lost")
	}
}

func TestScrollKeyboard(t *testing.T) {
	r := mouseFixture(t, `return {view={type='scroll',id='s',{type='text',height=100,text='long'}}}`)
	r.Key(tcell.NewEventKey(tcell.KeyPgDn, 0, 0))
	if r.State("s").Scroll <= 0 {
		t.Fatal("scroll key")
	}
	r.Key(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	if r.State("s").Scroll != 0 {
		t.Fatal("home")
	}
}
