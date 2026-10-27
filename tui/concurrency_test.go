package tui_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func TestEmptyLuaPatch(t *testing.T) {
	r := mouseFixture(t, `return {view={type='list',id='list',items={'a'}},actions={clear=function()ui.patch('list',{items={}})end}}`)
	if err := r.Action("clear", nil); err != nil {
		t.Fatal(err)
	}
	if len(r.Items(r.Node("list"))) != 0 {
		t.Fatal("list not cleared")
	}
}

func TestClickThenBackspace(t *testing.T) {
	r := mouseFixture(t, `return {view={type='input',id='edit',text='abc'}}`)
	r.Mouse(tcell.NewEventMouse(2, 0, tcell.Button1, 0))
	r.Mouse(tcell.NewEventMouse(2, 0, tcell.ButtonNone, 0))
	if err := r.Key(tcell.NewEventKey(tcell.KeyBackspace2, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if r.State("edit").Value != "ac" {
		t.Fatal(r.State("edit").Value)
	}
}

func TestReloadCandidateDoesNotMutateOriginal(t *testing.T) {
	r := mouseFixture(t, `return {view={type='input',id='edit',bind='draft',text='original'}}`)
	next, err := r.Reload(`ui.set('draft','candidate');return {view={type='input',id='edit',bind='draft'}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if r.State("edit").Value != "original" {
		t.Fatal("candidate mutated original")
	}
	if err := next.Paste("candidate edit"); err != nil {
		t.Fatal(err)
	}
	if r.State("edit").Value != "original" {
		t.Fatal("shared editor state")
	}
}

func TestEmptyLuaLists(t *testing.T) {
	for _, source := range []string{`return {view={type='list',id='list',items={}}}`, `return {view={type='text',spans={}}}`} {
		r, err := tui.Load(source)
		if err != nil {
			t.Fatal(err)
		}
		r.Close()
	}
}

func TestReleaseReplaceFocusRollback(t *testing.T) {
	r, err := tui.Load(`ui.set('value','old')
return {view=ui.column{id='root',ui.input{id='existing',bind='value'},ui.box{id='slot'}},actions={
 bad=function()
  ui.set('value','bad');host.dispatch('bad',{})
  ui.replace('slot',ui.input{id='new',bind='new_value',text='new'})
  ui.patch('existing',{text='bad'});ui.focus('absent')
 end,
 good=function()ui.patch('existing',{text='good'});ui.focus('existing')end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	stateBefore := r.State("existing")
	rootBefore := rootNode(t, r)
	existingBefore := r.Node("existing")
	slotBefore := r.Node("slot")
	focusBefore := r.Focus()
	revisionBefore := r.Revision()
	eventsBefore := r.DrainEvents()

	if err := r.Action("bad", nil); err == nil {
		t.Fatal("invalid focus accepted")
	}
	if !reflect.DeepEqual(rootNode(t, r), rootBefore) || !reflect.DeepEqual(r.Node("existing"), existingBefore) || !reflect.DeepEqual(r.Node("slot"), slotBefore) || r.Node("new") != nil || str(r.Node("existing"), "text", "") != "" {
		t.Fatal("failed action changed tree")
	}
	if !reflect.DeepEqual(r.State("existing"), stateBefore) || r.Model("value") != "old" || r.Focus() != focusBefore || r.Revision() != revisionBefore || len(r.DrainEvents()) != 0 {
		t.Fatal("failed action changed state, data, focus, revision or events")
	}
	_ = eventsBefore
	if err := r.Action("good", nil); err != nil {
		t.Fatal("valid action after rollback:", err)
	}
	if str(r.Node("existing"), "text", "") != "good" || r.Focus() != "existing" || r.Revision() <= revisionBefore || r.Node("new") != nil || r.Model("value") != "old" {
		t.Fatal("valid action retained failed transaction changes")
	}
}

func TestReleaseExternalPublicAPI(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module consumer\n\ngo 1.24.0\n\nrequire github.com/VintlGvard/LugoTUI v0.0.0\n\nreplace github.com/VintlGvard/LugoTUI => " + strconv.Quote(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0600); err != nil {
		t.Fatal(err)
	}
	const source = `package consumer
import (
 "context"
 "testing"
 "time"
 "github.com/VintlGvard/LugoTUI/tui"
)
func TestPublicAPI(t *testing.T) {
 options := tui.Options{
  OnEvent: func(r *tui.Runtime, event tui.HostEvent) error { return nil },
  Theme: tui.DefaultTheme(),
  ReservedKeys: &tui.ReservedKeys{Quit: []string{"Ctrl+Q"}, Reload: []string{"F5"}},
 }
 var theme tui.Theme = options.Theme
 _ = theme
 r, err := tui.Load("return {view=ui.text{id='root'},actions={done=function(p)ui.set('value',p.value)end}}")
 if err != nil { t.Fatal(err) }
 defer r.Close()
 if err := options.OnEvent(r, tui.HostEvent{Action: "smoke"}); err != nil { t.Fatal(err) }
 if err := r.Patch("root", map[string]any{"text":"patched"}); err != nil { t.Fatal(err) }
 if err := r.Replace("root", &tui.Node{Type:"text", ID:"next", Props:map[string]any{"text":"replaced"}}); err != nil { t.Fatal(err) }
 if _, err := r.StartTask("done", func(context.Context) (map[string]any, error) {
  return map[string]any{"value":"done"}, nil
 }); err != nil { t.Fatal(err) }
 var results <-chan tui.TaskResult = r.TaskResults()
 select {
 case result := <-results:
  if err := r.HandleTask(result); err != nil { t.Fatal(err) }
 case <-time.After(5*time.Second): t.Fatal("task timeout")
 }
 if r.Model("value") != "done" { t.Fatal("task result missing") }
}
`
	if err := os.WriteFile(filepath.Join(dir, "consumer_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-mod=mod", ".")
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOWORK=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("external public API: %v (context: %v)\n%s", err, ctx.Err(), output)
	}
}

func TestReleaseFocusAfterGoUpdates(t *testing.T) {
	r, err := tui.Load(`return {view=ui.box{id='root',ui.input{id='editor'}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Patch("root", map[string]any{"hidden": true}); err != nil {
		t.Fatal(err)
	}
	if r.Focus() != "" {
		t.Fatal("hidden focus", r.Focus())
	}
	if err := r.Patch("root", map[string]any{"hidden": false}); err != nil {
		t.Fatal(err)
	}
	if r.Focus() != "editor" {
		t.Fatal("restored focus", r.Focus())
	}
	if err := r.Replace("editor", &tui.Node{Type: "text", ID: "editor"}); err != nil {
		t.Fatal(err)
	}
	if r.Focus() != "" {
		t.Fatal("nonfocusable focus", r.Focus())
	}
}

func TestReleasePropsOwnership(t *testing.T) {
	r, err := tui.Load(`return {view=ui.text{id='text'}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, replace := range []bool{false, true} {
		spans := []any{map[string]any{"text": "owned"}}
		props := map[string]any{"spans": spans}
		if replace {
			err = r.Replace("text", &tui.Node{Type: "text", ID: "text", Props: props})
		} else {
			err = r.Patch("text", props)
		}
		if err != nil {
			t.Fatal(err)
		}
		spans[0].(map[string]any)["text"] = make(chan int)
		if r.Node("text").Props["spans"].([]any)[0].(map[string]any)["text"] != "owned" {
			t.Fatal("caller alias")
		}
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if err := r.Patch("text", map[string]any{"items": []any{cycle}}); err == nil {
		t.Fatal("cyclic props accepted")
	}
}

func TestReleaseTaskCancellationAndStaleError(t *testing.T) {
	r, err := tui.Load(`return {view=ui.text{id='text'},actions={done=function()error('must not run')end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.TaskResults() != nil {
		t.Fatal("unstarted task channel")
	}
	id, err := r.StartTask("done", func(context.Context) (map[string]any, error) { return nil, errors.New("stale error") })
	if err != nil {
		t.Fatal(err)
	}
	var result tui.TaskResult
	select {
	case result = <-r.TaskResults():
	case <-time.After(time.Second):
		t.Fatal("task timeout")
	}
	r.CancelTask(id)
	if err := r.HandleTask(result); err != nil {
		t.Fatal("cancelled error surfaced", err)
	}
	next, err := r.Reload(`return {view=ui.text{id='text'}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if err := next.HandleTask(result); err != nil {
		t.Fatal("stale error surfaced", err)
	}
	stopped := make(chan struct{})
	_, err = r.StartTask("done", func(ctx context.Context) (map[string]any, error) { <-ctx.Done(); close(stopped); return nil, ctx.Err() })
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	r.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("worker not cancelled on Close")
	}
}
