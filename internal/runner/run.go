package runner

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/gdamore/tcell/v2"
)

var ErrPanicRecovered = errors.New("lugotui: panic recovered")

type ReservedKeys struct {
	Quit   []string
	Reload []string
}

type EventHandler func(*engine.Runtime, engine.HostEvent) error

type EventMiddleware func(EventHandler) EventHandler

type ControlKind int

const (
	ControlAction ControlKind = iota
	ControlSet
	ControlPatch
	ControlReload
	ControlQuit
	ControlAltScreen
)

type ControlMsg struct {
	Kind      ControlKind
	Action    string
	Payload   map[string]any
	Key       string
	Value     any
	ID        string
	Props     map[string]any
	AltScreen bool
	Reply     chan error
}

type Options struct {
	Theme                engine.Theme
	OnEvent              func(*engine.Runtime, engine.HostEvent) error
	Middlewares          []EventMiddleware
	InputFilter          func(tcell.Event) tcell.Event
	StateObservers       []engine.StateObserver
	Renderer             engine.Renderer
	Clipboard            engine.Clipboard
	Screen               tcell.Screen
	ReservedKeys         *ReservedKeys
	View                 *engine.Node
	ViewFunc             func(*engine.Runtime) *engine.Node
	InitialData          map[string]any
	GoActions            map[string]func(*engine.Runtime, map[string]any) error
	AllowHostActions     bool
	AppFile              string
	Source               string
	SocketPath           string
	Check                bool
	Snapshot             bool
	Sandbox              bool
	Inline               bool
	InlineHeight         int
	AltScreen            *bool
	Mouse                *bool
	BracketedPaste       *bool
	FPS                  int
	DisablePanicRecovery bool
	Control              <-chan ControlMsg
	OnReady              func(*engine.Runtime)
}

func Run(args []string) error {
	return RunCLI(args, "")
}

func RunCLI(args []string, buildVersion string) error {
	flags := flag.NewFlagSet("lugotui", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: lugotui [flags]")
		flags.PrintDefaults()
		fmt.Fprintln(flags.Output(), "\nExamples:\n  lugotui --app app.lua\n  lugotui --app app.lua --check\n  lugotui --app app.lua --snapshot\n  lugotui --sandbox --app app.lua --check")
	}
	version := flags.Bool("version", false, "print version, commit, and Go version and exit")
	file := flags.String("app", "", "Lua application file (default: embedded demo)")
	check := flags.Bool("check", false, "validate and render without terminal")
	socket := flags.String("socket", "", "local agent Unix socket path (permissions 0600)")
	sandbox := flags.Bool("sandbox", false, "Linux process isolation via bubblewrap; no network or host filesystem")
	snapshot := flags.Bool("snapshot", false, "print semantic JSON snapshot (implies check)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *version {
		if buildVersion == "" {
			buildVersion = "lugotui dev"
		}
		fmt.Println(buildVersion)
		return nil
	}
	return RunOptions(Options{AppFile: *file, SocketPath: *socket, Check: *check, Snapshot: *snapshot, Sandbox: *sandbox})
}

func applyRuntimeOptions(r *engine.Runtime, options Options) {
	r.Theme = options.Theme
	if options.Clipboard != nil {
		r.Clipboard = options.Clipboard
	} else if r.Clipboard == nil {
		r.Clipboard = engine.NewOSC52Clipboard(os.Stderr)
	}
	if len(options.StateObservers) > 0 {
		r.StateObservers = append([]engine.StateObserver(nil), options.StateObservers...)
	}
	r.Inline = options.Inline
	r.InlineHeight = options.InlineHeight
	if options.AltScreen != nil {
		r.AltScreen = *options.AltScreen
	} else {
		r.AltScreen = !options.Inline
	}
}

func drawRuntime(options Options, screen tcell.Screen, r *engine.Runtime, diagnostic string) {
	if options.Renderer != nil {
		options.Renderer.Draw(screen, r, diagnostic)
		return
	}
	r.Draw(screen, diagnostic)
}

func RunOptions(options Options) error {
	if _, err := reservedKeys(options.ReservedKeys); err != nil {
		return err
	}
	if options.Sandbox {
		return Sandbox(options)
	}
	allowHost := options.AllowHostActions || options.OnEvent != nil || len(options.Middlewares) > 0
	var (
		r          *engine.Runtime
		err        error
		loadSource func() (string, error)
	)
	switch {
	case options.ViewFunc != nil:
		loadSource = func() (string, error) { return "", nil }
		r, err = engine.LoadViewFunc(options.ViewFunc, options.InitialData, options.GoActions, allowHost)
	case options.View != nil:
		loadSource = func() (string, error) { return "", nil }
		r, err = engine.LoadNode(options.View, options.InitialData, options.GoActions, allowHost)
	default:
		loadSource = func() (string, error) {
			if options.Source != "" {
				if len(options.Source) > 1<<20 {
					return "", fmt.Errorf("application source exceeds 1 MiB")
				}
				return options.Source, nil
			}
			return readSource(options.AppFile)
		}
		var source string
		source, err = loadSource()
		if err != nil {
			return err
		}
		r, err = engine.LoadWithConfig(source, options.AppFile, options.InitialData, options.GoActions, allowHost)
	}
	if err != nil {
		return err
	}
	applyRuntimeOptions(r, options)
	if options.Check || options.Snapshot {
		defer r.Close()
		s := tcell.NewSimulationScreen("UTF-8")
		if err = s.Init(); err != nil {
			return err
		}
		defer s.Fini()
		s.SetSize(100, 30)
		drawRuntime(options, s, r, "")
		if options.Snapshot {
			b, e := r.Snapshot()
			if e != nil {
				return e
			}
			fmt.Println(string(b))
		} else {
			fmt.Printf("OK: %d named nodes, 100x30 layout\n", len(r.Nodes))
		}
		return nil
	}
	screen := options.Screen
	if screen == nil {
		screen, err = tcell.NewScreen()
		if err != nil {
			r.Close()
			return err
		}
		if err = screen.Init(); err != nil {
			r.Close()
			return err
		}
	}
	return runScreen(options, screen, r, loadSource)
}

func readSource(path string) (string, error) {
	if path == "" {
		return engine.Demo(), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if len(data) > limit {
		return "", fmt.Errorf("application source exceeds 1 MiB: %s", path)
	}
	return string(data), nil
}

func buildEventHandler(options Options) EventHandler {
	base := options.OnEvent
	if base == nil {
		base = func(*engine.Runtime, engine.HostEvent) error { return nil }
	}
	h := EventHandler(base)
	for i := len(options.Middlewares) - 1; i >= 0; i-- {
		if mw := options.Middlewares[i]; mw != nil {
			h = mw(h)
		}
	}
	return h
}

func runScreen(options Options, screen tcell.Screen, r *engine.Runtime, readSource func() (string, error)) (runErr error) {
	defer func() {
		screen.HideCursor()
		screen.DisableMouse()
		screen.DisablePaste()
		screen.Fini()
	}()
	defer func() { r.Close() }()
	if !options.DisablePanicRecovery {
		defer func() {
			if rec := recover(); rec != nil {
				runErr = fmt.Errorf("%w: %v", ErrPanicRecovered, rec)
			}
		}()
	}
	bindings, err := reservedKeys(options.ReservedKeys)
	if err != nil {
		return err
	}
	applyRuntimeOptions(r, options)
	if options.Mouse == nil || *options.Mouse {
		screen.EnableMouse(tcell.MouseDragEvents)
	}
	if options.BracketedPaste == nil || *options.BracketedPaste {
		screen.EnablePaste()
	}
	if options.OnReady != nil {
		options.OnReady(r)
	}
	events := make(chan tcell.Event, 32)
	done := make(chan struct{})
	requests := make(chan engine.AgentRequest, 4)
	var stop func()
	if options.SocketPath != "" {
		stop, err = engine.ServeAgent(options.SocketPath, requests, done)
		if err != nil {
			return err
		}
	}
	defer func() {
		close(done)
		if stop != nil {
			stop()
		}
	}()
	paste := false
	var pasted strings.Builder
	go screen.ChannelEvents(events, done)
	signals := make(chan os.Signal, 1)
	notifySignals(signals)
	defer signal.Stop(signals)
	diagnostic := ""
	pending := make(chan struct{}, 1)
	var fileChanges <-chan struct{}
	if options.AppFile != "" && readSource != nil {
		ch := make(chan struct{}, 1)
		fileChanges = ch
		go func() {
			ticker := time.NewTicker(150 * time.Millisecond)
			defer ticker.Stop()
			var lastMod time.Time
			var lastSize int64
			if fi, err := os.Stat(options.AppFile); err == nil {
				lastMod = fi.ModTime()
				lastSize = fi.Size()
			}
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					fi, err := os.Stat(options.AppFile)
					if err != nil {
						continue
					}
					if !fi.ModTime().Equal(lastMod) || fi.Size() != lastSize {
						lastMod = fi.ModTime()
						lastSize = fi.Size()
						select {
						case ch <- struct{}{}:
						default:
						}
					}
				}
			}
		}()
	}
	doReload := func() {
		if readSource == nil {
			return
		}
		var source string
		source, err = readSource()
		if err == nil {
			var candidate *engine.Runtime
			candidate, err = r.Reload(source)
			if err == nil {
				r.Close()
				r = candidate
				applyRuntimeOptions(r, options)
				diagnostic = ""
			}
		}
	}
	hasEventHandler := options.OnEvent != nil || len(options.Middlewares) > 0
	eventHandler := buildEventHandler(options)
	dispatch := func() bool {
		if !hasEventHandler {
			if options.SocketPath == "" {
				r.DrainEvents()
			}
			return false
		}
		batch := r.DrainEvents()
		for _, event := range batch {
			if event.Action == "__alt_screen" {
				if enter, ok := event.Payload.(bool); ok {
					r.AltScreen = enter
					r.Inline = !enter
					screen.Sync()
				}
				continue
			}
			if e := eventHandler(r, event); e != nil {
				diagnostic = e.Error()
			}
		}
		if len(r.Events) != 0 {
			select {
			case pending <- struct{}{}:
			default:
			}
		}
		return len(batch) != 0
	}
	dispatch()
	drawRuntime(options, screen, r, diagnostic)
	tickInterval := 16 * time.Millisecond
	if options.FPS > 0 && options.FPS <= 240 {
		tickInterval = time.Second / time.Duration(options.FPS)
	}
	var ticker *time.Ticker
	var ticks <-chan time.Time
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	for {
		if r.NeedsTick() {
			if ticker == nil {
				ticker = time.NewTicker(tickInterval)
				ticks = ticker.C
			}
		} else if ticker != nil {
			ticker.Stop()
			ticker, ticks = nil, nil
		}
		revision, focus, previousDiagnostic := r.Revision, r.Focus, diagnostic
		redraw := false
		err = nil
		select {
		case sig := <-signals:
			if isResizeSignal(sig) {
				screen.Sync()
				redraw = true
			} else {
				return nil
			}
		case <-fileChanges:
			doReload()
			redraw = true
		case now := <-ticks:
			if r.NeedsTick() {
				before := r.AnimationRevision
				err = r.Tick(now)
				redraw = before != r.AnimationRevision
			}
		case <-pending:
		case result := <-r.TaskResults():
			err = r.HandleTask(result)
		case msg := <-options.Control:
			var ctrlErr error
			switch msg.Kind {
			case ControlQuit:
				if msg.Reply != nil {
					msg.Reply <- nil
				}
				return nil
			case ControlReload:
				doReload()
				ctrlErr = err
				redraw = true
			case ControlAction:
				ctrlErr = r.Action(msg.Action, msg.Payload)
				if ctrlErr == nil {
					diagnostic = ""
				}
				redraw = true
			case ControlSet:
				if r.Data == nil {
					r.Data = map[string]any{}
				}
				oldVal := r.Data[msg.Key]
				r.Data[msg.Key] = msg.Value
				if ctrlErr = engine.ValidateModel(r.Data); ctrlErr != nil {
					r.Data[msg.Key] = oldVal
				} else {
					for id, n := range r.Nodes {
						if (n.Type == "input" || n.Type == "textarea") && r.State[id] != nil {
							if b, _ := n.Props["bind"].(string); b == msg.Key {
								if s, ok := msg.Value.(string); ok {
									r.State[id].Value = s
									r.State[id].Cursor = min(r.State[id].Cursor, len([]rune(s)))
									r.State[id].Anchor = -1
								}
							}
						}
					}
					r.Revision++
					for _, fn := range r.StateObservers {
						if fn != nil {
							fn(msg.Key, oldVal, msg.Value)
						}
					}
					diagnostic = ""
					redraw = true
				}
			case ControlPatch:
				ctrlErr = r.Patch(msg.ID, msg.Props)
				if ctrlErr == nil {
					diagnostic = ""
				}
				redraw = true
			case ControlAltScreen:
				r.AltScreen = msg.AltScreen
				r.Inline = !msg.AltScreen
				screen.Sync()
				redraw = true
			}
			if msg.Reply != nil {
				msg.Reply <- ctrlErr
			}
			err = ctrlErr
		case request := <-requests:
			var queued []engine.HostEvent
			if hasEventHandler {
				queued, r.Events = r.Events, nil
			}
			reply := r.AgentReply(request.Data)
			if hasEventHandler {
				r.Events = append(queued, r.Events...)
			}
			request.Reply <- reply
		case event := <-events:
			if event == nil {
				return nil
			}
			if options.InputFilter != nil {
				event = options.InputFilter(event)
				if event == nil {
					continue
				}
			}
			err = nil
			userAction := false
			switch e := event.(type) {
			case *tcell.EventPaste:
				paste = e.Start()
				if paste {
					pasted.Reset()
				} else {
					userAction = true
					err = r.Paste(pasted.String())
					pasted.Reset()
				}
			case *tcell.EventKey:
				if paste {
					if pasted.Len() < 1<<20 {
						switch e.Key() {
						case tcell.KeyRune:
							pasted.WriteRune(e.Rune())
						case tcell.KeyEnter:
							pasted.WriteByte('\n')
						case tcell.KeyTab:
							pasted.WriteByte('\t')
						}
					}
					continue
				}
				if bindings[normalizeKey(e.Name())] == "quit" {
					return nil
				}
				if bindings[normalizeKey(e.Name())] == "reload" {
					doReload()
				} else {
					userAction = true
					err = r.Key(e)
				}
			case *tcell.EventMouse:
				userAction = true
				err = r.Mouse(e)
			case *tcell.EventResize:
				screen.Sync()
			}
			if userAction && err == nil {
				diagnostic = ""
			}
			redraw = true
		}
		if err != nil {
			diagnostic = err.Error()
		}
		called := dispatch()
		if redraw || called || revision != r.Revision || focus != r.Focus || previousDiagnostic != diagnostic {
			drawRuntime(options, screen, r, diagnostic)
		}
	}
}

func normalizeKey(name string) string {
	if start := strings.Index(name, "Rune["); start >= 0 {
		return strings.ReplaceAll(name[:start], "-", "+") + name[start:]
	}
	return strings.ReplaceAll(name, "-", "+")
}

func reservedKeys(keys *ReservedKeys) (map[string]string, error) {
	if keys == nil {
		keys = &ReservedKeys{Quit: []string{"Ctrl-Q", "Ctrl-C"}, Reload: []string{"F5"}}
	}
	bindings := make(map[string]string)
	for _, group := range []struct {
		action string
		names  []string
	}{{"quit", keys.Quit}, {"reload", keys.Reload}} {
		for _, name := range group.names {
			normalized := normalizeKey(name)
			if !validKeyName(name) {
				return nil, fmt.Errorf("unknown reserved key %q", name)
			}
			if previous := bindings[normalized]; previous != "" {
				return nil, fmt.Errorf("duplicate reserved key %q (%s/%s)", name, previous, group.action)
			}
			bindings[normalized] = group.action
		}
	}
	return bindings, nil
}

var knownKeyNames = sync.OnceValue(func() map[string]bool {
	set := make(map[string]bool, 16*len(tcell.KeyNames))
	for bits := 0; bits < 16; bits++ {
		var modifiers tcell.ModMask
		for i, flag := range []tcell.ModMask{tcell.ModShift, tcell.ModCtrl, tcell.ModAlt, tcell.ModMeta} {
			if bits&(1<<i) != 0 {
				modifiers |= flag
			}
		}
		for key := range tcell.KeyNames {
			set[normalizeKey(tcell.NewEventKey(key, 0, modifiers).Name())] = true
		}
	}
	return set
})

func validKeyName(name string) bool {
	name = normalizeKey(name)
	if knownKeyNames()[name] {
		return true
	}
	if start := strings.Index(name, "Rune["); start >= 0 && strings.HasSuffix(name, "]") {
		runes := []rune(name[start+5 : len(name)-1])
		if len(runes) == 1 {
			for bits := 0; bits < 16; bits++ {
				var modifiers tcell.ModMask
				for i, flag := range []tcell.ModMask{tcell.ModShift, tcell.ModCtrl, tcell.ModAlt, tcell.ModMeta} {
					if bits&(1<<i) != 0 {
						modifiers |= flag
					}
				}
				if normalizeKey(tcell.NewEventKey(tcell.KeyRune, runes[0], modifiers).Name()) == name {
					return true
				}
			}
		}
	}
	return false
}
