package runner

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/gdamore/tcell/v2"
)

const loopSource = `local ui=require('ui')
return {view=ui.text{id='text',text='before'}, keymap={F1='emit',['Ctrl+Q']='emit',['Ctrl+C']='emit',F5='emit'}, actions={
 emit=function() host.dispatch('start',{}) end,
 done=function() ui.patch('text',{text='after'}); host.dispatch('done',{}) end,
 change=function() ui.patch('text',{text='changed'}) end}}
`

type observedScreen struct {
	tcell.SimulationScreen
	shows chan string
}

func (s *observedScreen) Show() {
	s.SimulationScreen.Show()
	cells, _, _ := s.GetContents()
	var text strings.Builder
	for _, cell := range cells {
		text.WriteString(string(cell.Runes))
	}
	s.shows <- text.String()
}
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("loop timed out")
		var zero T
		return zero
	}
}
func startLoop(t *testing.T, options Options, source func() (string, error)) (*observedScreen, <-chan error) {
	t.Helper()
	r, err := engine.Load(loopSource)
	if err != nil {
		t.Fatal(err)
	}
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), shows: make(chan string, 128)}
	if err = s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(60, 8)
	done := make(chan error, 1)
	go func() { done <- runScreen(options, s, r, source) }()
	receive(t, s.shows)
	t.Cleanup(func() {
		s.PostEventWait(tcell.NewEventKey(tcell.KeyExit, 0, 0))
		if err := receive(t, done); err != nil {
			t.Error(err)
		}
		s.Fini()
	})
	return s, done
}
func testKeys() *ReservedKeys { return &ReservedKeys{Quit: []string{"Exit"}, Reload: []string{"F5"}} }

func TestAnimationRedrawOnlyOnFrameChange(t *testing.T) {
	r, err := engine.Load(`local ui=require('ui'); return {view=ui.spinner{id='spinner',frames={'x','y'},interval=500}}`)
	if err != nil {
		t.Fatal(err)
	}
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), shows: make(chan string, 128)}
	if err = s.Init(); err != nil {
		r.Close()
		t.Fatal(err)
	}
	s.SetSize(20, 2)
	done := make(chan error, 1)
	go func() { done <- runScreen(Options{ReservedKeys: testKeys()}, s, r, nil) }()
	t.Cleanup(func() {
		s.PostEventWait(tcell.NewEventKey(tcell.KeyExit, 0, 0))
		if err := receive(t, done); err != nil {
			t.Error(err)
		}
		s.Fini()
	})
	first := receive(t, s.shows)
	select {
	case <-s.shows:
		t.Fatal("unchanged animation requested redraw")
	case <-time.After(100 * time.Millisecond):
	}
	if next := receive(t, s.shows); next == first {
		t.Fatal("spinner frame did not change")
	}
}

func TestSocketRedrawAndEvents(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".runner-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(dir); err != nil {
			t.Error(err)
		}
	})
	path := dir + "/agent.sock"
	s, _ := startLoop(t, Options{SocketPath: path, ReservedKeys: testKeys()}, nil)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	request := func(data string) string {
		t.Helper()
		if _, err := conn.Write([]byte(data + "\n")); err != nil {
			t.Fatal(err)
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return line
	}
	request(`{"method":"snapshot"}`)
	request(`{"method":"events"}`)
	select {
	case <-s.shows:
		t.Fatal("read-only socket request redrew")
	default:
	}
	request(`{"method":"message","action":"change"}`)
	receive(t, s.shows)
	request(`{"method":"message","action":"emit"}`)
	receive(t, s.shows)
	if reply := request(`{"method":"events"}`); !strings.Contains(reply, `"action":"start"`) {
		t.Fatal(reply)
	}
}

func TestCallbackTaskAndReentrantError(t *testing.T) {
	seen := make(chan string, 8)
	callback := func(r *engine.Runtime, event engine.HostEvent) error {
		seen <- event.Action
		if event.Action == "start" {
			_, err := r.StartTask("done", func(context.Context) (map[string]any, error) { return map[string]any{}, nil })
			if err != nil {
				return err
			}
			r.Events = append(r.Events, engine.HostEvent{Action: "nested"})
			return errors.New("callback failure")
		}
		return nil
	}
	s, _ := startLoop(t, Options{OnEvent: callback, ReservedKeys: testKeys()}, nil)
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	got := map[string]bool{}
	for len(got) < 3 {
		got[receive(t, seen)] = true
	}
	if !got["start"] || !got["nested"] || !got["done"] {
		t.Fatal(got)
	}
	for {
		text := receive(t, s.shows)
		if strings.Contains(text, "after") && strings.Contains(text, "callback failure") {
			break
		}
	}
}

func TestReloadCancelsTask(t *testing.T) {
	cancelled := make(chan struct{})
	started := make(chan struct{}, 1)
	completed := make(chan struct{}, 1)
	count := 0
	callback := func(r *engine.Runtime, event engine.HostEvent) error {
		if event.Action == "done" {
			completed <- struct{}{}
		}
		if event.Action == "start" {
			count++
			waitForCancellation := count == 1
			_, err := r.StartTask("done", func(ctx context.Context) (map[string]any, error) {
				if !waitForCancellation {
					return map[string]any{}, nil
				}
				<-ctx.Done()
				close(cancelled)
				return nil, ctx.Err()
			})
			started <- struct{}{}
			return err
		}
		return nil
	}
	s, _ := startLoop(t, Options{OnEvent: callback, ReservedKeys: testKeys()}, func() (string, error) { return loopSource, nil })
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	receive(t, started)
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF5, 0, 0))
	receive(t, cancelled)
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	receive(t, started)
	receive(t, completed)
}

func TestTaskErrorDiagnostic(t *testing.T) {
	s, _ := startLoop(t, Options{ReservedKeys: testKeys(), OnEvent: func(r *engine.Runtime, _ engine.HostEvent) error {
		_, err := r.StartTask("done", func(context.Context) (map[string]any, error) { return nil, errors.New("worker failure") })
		return err
	}}, nil)
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	for {
		if strings.Contains(receive(t, s.shows), "worker failure") {
			break
		}
	}
}

func TestFileWatcherAutoReload(t *testing.T) {
	dir := t.TempDir()
	appFile := filepath.Join(dir, "app.lua")
	v1 := "local ui=require('ui'); return {view=ui.text{id='t',text='version-one'}}"
	if err := os.WriteFile(appFile, []byte(v1), 0600); err != nil {
		t.Fatal(err)
	}
	s, _ := startLoop(t, Options{AppFile: appFile, ReservedKeys: testKeys()}, func() (string, error) {
		return readSource(appFile)
	})
	time.Sleep(100 * time.Millisecond)
	v2 := "local ui=require('ui'); return {view=ui.text{id='t',text='version-two-updated'}}"
	if err := os.WriteFile(appFile, []byte(v2), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		if strings.Contains(receive(t, s.shows), "version-two-updated") {
			break
		}
	}
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(appFile, []byte("broken syntax !!!"), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		if strings.Contains(receive(t, s.shows), "parse error") {
			break
		}
	}
	time.Sleep(100 * time.Millisecond)
	v3 := "local ui=require('ui'); return {view=ui.text{id='t',text='version-three-fixed'}}"
	if err := os.WriteFile(appFile, []byte(v3), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		if strings.Contains(receive(t, s.shows), "version-three-fixed") {
			break
		}
	}
}

func TestReadSource(t *testing.T) {
	path := t.TempDir() + "/app.lua"
	for _, size := range []int{0, 1 << 20, 1<<20 + 1} {
		want := strings.Repeat("x", size)
		if err := os.WriteFile(path, []byte(want), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := readSource(path)
		if size > 1<<20 {
			if err == nil {
				t.Fatal("oversized source accepted")
			}
		} else if err != nil || got != want {
			t.Fatalf("size %d: length %d, error %v", size, len(got), err)
		}
	}
	if _, err := readSource(path + ".missing"); err == nil {
		t.Fatal("missing source accepted")
	}
	if got, err := readSource(""); err != nil || got != engine.Demo() {
		t.Fatalf("embedded source: %v", err)
	}
}

func TestReservedKeys(t *testing.T) {
	if normalizeKey("Alt-Rune[-]") != "Alt+Rune[-]" || normalizeKey("Rune[-]") == normalizeKey("Rune[+]") {
		t.Fatal("rune punctuation normalized")
	}
	if _, err := reservedKeys(&ReservedKeys{Quit: []string{"Rune[-]", "Rune[+]"}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		keys         *ReservedKeys
		quit, reload bool
	}{
		{"default", nil, true, true}, {"disabled", &ReservedKeys{}, false, false}, {"override", &ReservedKeys{Quit: []string{"Exit"}, Reload: []string{"F4"}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bindings, err := reservedKeys(tc.keys)
			if err != nil {
				t.Fatal(err)
			}
			if (bindings["Ctrl+Q"] == "quit") != tc.quit || (bindings["F5"] == "reload") != tc.reload {
				t.Fatal(bindings)
			}
		})
	}
	for _, keys := range []*ReservedKeys{{Quit: []string{"bogus"}}, {Quit: []string{"Ctrl-Q", "Ctrl+Q"}}, {Quit: []string{"F5"}, Reload: []string{"F5"}}} {
		if _, err := reservedKeys(keys); err == nil {
			t.Fatal("invalid keys accepted", keys)
		}
	}
	for _, name := range []string{"Ctrl-Q", "Ctrl+Q", "Alt+Rune[x]", "Rune[x]", "Rune[-]", "F5"} {
		if !validKeyName(name) {
			t.Fatal(name)
		}
	}
}

func TestDisabledAndOverriddenKeysLoop(t *testing.T) {
	seen := make(chan string, 8)
	s, _ := startLoop(t, Options{ReservedKeys: &ReservedKeys{Quit: []string{"Exit"}}, OnEvent: func(_ *engine.Runtime, event engine.HostEvent) error { seen <- event.Action; return nil }}, nil)
	for _, key := range []tcell.Key{tcell.KeyCtrlQ, tcell.KeyCtrlC, tcell.KeyF5} {
		s.PostEventWait(tcell.NewEventKey(key, 0, 0))
		if receive(t, seen) != "start" {
			t.Fatal("key not forwarded")
		}
	}
}

func TestDefaultQuitLoop(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyCtrlQ, tcell.KeyCtrlC} {
		r, err := engine.Load(loopSource)
		if err != nil {
			t.Fatal(err)
		}
		s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), shows: make(chan string, 8)}
		if err = s.Init(); err != nil {
			t.Fatal(err)
		}
		s.SetSize(60, 8)
		done := make(chan error, 1)
		go func() { done <- runScreen(Options{}, s, r, nil) }()
		receive(t, s.shows)
		s.PostEventWait(tcell.NewEventKey(key, 0, 0))
		if err := receive(t, done); err != nil {
			t.Fatal(err)
		}
		s.Fini()
	}
}

func TestAutonomousAnimationStopsWhenInactive(t *testing.T) {
	source := `local ui=require('ui')
return {view=ui.spinner{id='spin',frames={'A','B'},interval=16},keymap={F1='stop'},
actions={stop=function() ui.patch('spin',{running=false}) end}}`
	r, err := engine.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), shows: make(chan string, 128)}
	if err = s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(20, 2)
	done := make(chan error, 1)
	go func() { done <- runScreen(Options{ReservedKeys: testKeys()}, s, r, nil) }()
	t.Cleanup(func() {
		s.PostEventWait(tcell.NewEventKey(tcell.KeyExit, 0, 0))
		if err := receive(t, done); err != nil {
			t.Error(err)
		}
		s.Fini()
	})
	initial := receive(t, s.shows)
	animationDeadline := time.NewTimer(time.Second)
	defer animationDeadline.Stop()
animated:
	for {
		select {
		case frame := <-s.shows:
			if frame != initial {
				break animated
			}
		case <-animationDeadline.C:
			t.Fatal("spinner did not animate without input")
		}
	}
	s.PostEventWait(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	quiet := time.NewTimer(80 * time.Millisecond)
	defer quiet.Stop()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-s.shows:
			if !quiet.Stop() {
				select {
				case <-quiet.C:
				default:
				}
			}
			quiet.Reset(80 * time.Millisecond)
		case <-quiet.C:
			return
		case <-deadline.C:
			t.Fatal("inactive animation kept redrawing")
		}
	}
}

func TestSandboxRejectsHostOptions(t *testing.T) {
	for _, options := range []Options{{OnEvent: func(*engine.Runtime, engine.HostEvent) error { return nil }}, {ReservedKeys: &ReservedKeys{}}, {Theme: engine.Theme{Foreground: "red"}}} {
		if err := Sandbox(options); err == nil || !strings.Contains(err.Error(), "cannot serialize") {
			t.Fatal(err)
		}
	}
}
