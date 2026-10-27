package engine_test

import (
	"bufio"
	"context"
	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/gdamore/tcell/v2"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func node(kind, id string, props map[string]any, children ...*engine.Node) *engine.Node {
	return &engine.Node{Type: kind, ID: id, Props: props, Children: children}
}
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".sock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "agent.sock")
}

func TestWrappedIntrinsicHeight(t *testing.T) {
	root := node("column", "root", nil, node("text", "text", map[string]any{"text": "hello world again", "shrink": 0.0}), node("text", "after", map[string]any{"text": "end"}))
	engine.Layout(root, 11, 10)
	if root.Children[0].Rect.H != 2 || root.Children[1].Rect.Y != 2 {
		t.Fatal(root.Children[0].Rect, root.Children[1].Rect)
	}
	root.Children[0].Props["text"] = "short"
	engine.Layout(root, 11, 10)
	if root.Children[0].Rect.H != 1 {
		t.Fatal("stale measure")
	}
}

func TestLimitedWrap(t *testing.T) {
	for _, text := range []string{"hello world again", "界🙂 test\nnext", "longwordwithoutspaces", "a\n\nb"} {
		all := engine.Lines(text, 6, true)
		for limit := 0; limit <= len(all); limit++ {
			got := engine.LimitedLines(text, 6, true, limit)
			if len(got) != limit {
				t.Fatalf("limit %d got %v", limit, got)
			}
			for i := range got {
				if got[i] != all[i] {
					t.Fatal("prefix mismatch")
				}
			}
		}
	}
}

func BenchmarkLargeScene(b *testing.B) {
	children := make([]*engine.Node, 1000)
	for i := range children {
		children[i] = node("text", "", map[string]any{"text": "row", "height": 1.0, "shrink": 0.0})
	}
	root := node("column", "", nil, children...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Layout(root, 120, 40)
	}
}

func TestLayout(t *testing.T) {
	root := node("row", "", nil, node("box", "a", map[string]any{"grow": 1.0}), node("box", "b", map[string]any{"grow": 2.0}))
	engine.Layout(root, 10, 4)
	if root.Children[0].Rect.W+root.Children[1].Rect.W != 10 {
		t.Fatal("rounding loses cells")
	}
	root = node("row", "", nil, node("box", "a", map[string]any{"width": 8.0, "min_width": 6.0}), node("box", "b", map[string]any{"width": 8.0}))
	engine.Layout(root, 10, 3)
	if root.Children[0].Rect.W != 6 || root.Children[1].Rect.W != 4 {
		t.Fatalf("shrink: %+v %+v", root.Children[0].Rect, root.Children[1].Rect)
	}
	root = node("box", "", map[string]any{"padding": 1.0}, node("text", "a", map[string]any{"position": "absolute", "left": 0.0, "right": 0.0, "height": 1.0}))
	engine.Layout(root, 10, 4)
	if root.Children[0].Rect != (engine.Rect{X: 1, Y: 1, W: 8, H: 1}) {
		t.Fatal(root.Children[0].Rect)
	}
}

func BenchmarkLayout(b *testing.B) {
	r, e := engine.Load(engine.Demo())
	if e != nil {
		b.Fatal(e)
	}
	defer r.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Layout(r.Root, 120, 40)
	}
}

func TestEditorCacheResize(t *testing.T) {
	s := &engine.WidgetState{Value: "abcd\nef"}
	g, p := s.EditorIndex(4, true)
	_, again := s.EditorIndex(4, true)
	if &p[0] != &again[0] {
		t.Fatal("cache missed")
	}
	_, narrow := s.EditorIndex(2, true)
	if narrow[len(g)][1] <= p[len(g)][1] {
		t.Fatal("resize")
	}
	s.Value = "x"
	g, _ = s.EditorIndex(2, true)
	if len(g) != 1 {
		t.Fatal("stale text")
	}
}

func TestWordWrap(t *testing.T) {
	if got := engine.Lines("hello world again", 11, true); !reflect.DeepEqual(got, []string{"hello world", "again"}) {
		t.Fatal(got)
	}
}

func TestWrapWidth(t *testing.T) {
	for _, line := range engine.Lines("a bbbbbbbbb cc", 10, true) {
		if engine.TextWidth(line) > 10 {
			t.Fatal(line)
		}
	}
}

func TestConcurrentAgentReplies(t *testing.T) {
	requests := make(chan engine.AgentRequest, 4)
	done := make(chan struct{})
	path := socketPath(t)
	stop, err := engine.ServeAgent(path, requests, done)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { close(done); stop() }()
	go func() {
		for {
			select {
			case req := <-requests:
				req.Reply <- []byte(`{"ok":true}`)
			case <-done:
				return
			}
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("unix", path)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(time.Second))
			reader := bufio.NewReader(conn)
			for j := 0; j < 10; j++ {
				if _, err := conn.Write([]byte("{}\n")); err != nil {
					t.Error(err)
					return
				}
				if _, err := reader.ReadString('\n'); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestAgentStopWithoutExternalCancellation(t *testing.T) {
	requests := make(chan engine.AgentRequest)
	done := make(chan struct{})
	defer close(done)
	path := socketPath(t)
	stop, err := engine.ServeAgent(path, requests, done)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		stop()
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.Write([]byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Error("stop blocked while waiting for request receiver")
	}
}

func TestServeAgentDoneClosesListenerAndWorkers(t *testing.T) {
	for _, mode := range []string{"idle", "send", "reply"} {
		t.Run(mode, func(t *testing.T) {
			path := socketPath(t)
			requests := make(chan engine.AgentRequest)
			done := make(chan struct{})
			stop, err := engine.ServeAgent(path, requests, done)
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			conn, err := net.Dial("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if mode != "idle" {
				if _, err := conn.Write([]byte("{}\n")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "reply" {
				select {
				case <-requests:
				case <-time.After(time.Second):
					t.Fatal("worker did not send request")
				}
			}
			close(done)
			conn.SetReadDeadline(time.Now().Add(time.Second))
			if _, err := conn.Read(make([]byte, 1)); err == nil {
				t.Fatal("connection still open")
			} else if e, ok := err.(net.Error); ok && e.Timeout() {
				t.Fatal("worker did not close connection after done")
			}
			deadline := time.Now().Add(time.Second)
			for {
				next, err := net.DialTimeout("unix", path, time.Second)
				if err != nil {
					break
				}
				next.Close()
				if time.Now().After(deadline) {
					t.Fatal("listener still accepts after done")
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}

func TestServeAgentStopBeforeDoneIsIdempotent(t *testing.T) {
	stop, err := engine.ServeAgent(socketPath(t), make(chan engine.AgentRequest), make(chan struct{}))
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); stop() }()
		}
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("stop blocked with done open")
	}
}

func TestCorruptModelConversion(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, tc := range []struct {
		value   any
		message string
	}{
		{make(chan int), "unsupported Go type chan int"}, {int64(1<<53) + 1, "integer exceeds exact Lua range"},
		{-int64(1<<53) - 1, "integer exceeds exact Lua range"}, {math.NaN(), "nonfinite number"}, {math.Inf(1), "nonfinite number"}, {cycle, "value nesting exceeds 32"},
	} {
		r, err := engine.Load(`return {view=ui.text{},actions={get=function()host.dispatch('get',ui.get('bad'))end}}`)
		if err != nil {
			t.Fatal(err)
		}
		r.Data["bad"] = map[string]any{"items": []any{tc.value}}
		err = r.Action("get", nil)
		r.Close()
		if err == nil || !strings.Contains(err.Error(), "model.bad.items[1]") || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("ui.get error: %v", err)
		}
	}
}

func TestBindingRemovalFallback(t *testing.T) {
	r, err := engine.Load(`ui.set('body','one\ntwo\nthree');return {view=ui.column{ui.text{id='body',bind='body',shrink=0},ui.text{id='after',text='end'}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(11, 10)
	r.Draw(screen, "")
	delete(r.Data, "body")
	r.Nodes["body"].Props["text"] = "fallback\nline"
	r.Draw(screen, "")
	if r.Nodes["body"].Rect.H != 2 {
		t.Fatal("stale binding after removal", r.Nodes["body"].Rect)
	}
}

func TestModelValidationLimit(t *testing.T) {
	if e := engine.ValidateModel(map[string]any{"big": strings.Repeat("x", engine.MaxText)}); e == nil {
		t.Fatal("limit absent")
	}
}

func TestReleaseInvalidPayloadBeforeLua(t *testing.T) {
	r, err := engine.Load(`local count=0
return {view=ui.text{id='root'},actions={
 tick=function()count=count+1;ui.set('count',count);host.dispatch('tick',{count=count})end,
 done=function(payload)ui.set('result',payload.value)end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	gate := make(chan struct{})
	started := make(chan struct{})
	id, err := r.StartTask("done", func(ctx context.Context) (map[string]any, error) {
		close(started)
		select {
		case <-gate:
			return map[string]any{"value": "finished"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("task did not start")
	}
	top, root, revision := r.L.GetTop(), r.Root, r.Revision
	data := make(map[string]any, len(r.Data))
	for k, v := range r.Data {
		data[k] = v
	}
	events := append([]engine.HostEvent(nil), r.Events...)
	if err := r.Action("tick", map[string]any{"invalid": make(chan int)}); err == nil {
		t.Fatal("unsupported Go payload accepted")
	}
	if r.L.GetTop() != top || r.Root != root || r.Revision != revision || !reflect.DeepEqual(r.Data, data) || !reflect.DeepEqual(r.Events, events) || len(r.Staged) != 0 {
		t.Fatal("invalid payload changed Lua stack or runtime")
	}
	close(gate)
	select {
	case result := <-r.TaskResults():
		if result.ID != id || result.Err != nil {
			t.Fatalf("task result = %+v", result)
		}
		if err := r.HandleTask(result); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("invalid payload canceled pending task")
	}
	if r.Data["result"] != "finished" {
		t.Fatal("pending task result not applied")
	}
	if err := r.Action("tick", nil); err != nil {
		t.Fatal(err)
	}
	if r.Data["count"] != float64(1) || len(r.Events) != 1 || r.Events[0].Action != "tick" || r.L.GetTop() != top {
		t.Fatal("invalid payload executed Lua closure")
	}
}
