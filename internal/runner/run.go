package runner

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/gdamore/tcell/v2"
)

type ReservedKeys struct {
	Quit   []string
	Reload []string
}

type Options struct {
	Theme        engine.Theme
	OnEvent      func(*engine.Runtime, engine.HostEvent) error
	ReservedKeys *ReservedKeys
	AppFile      string
	SocketPath   string
	Check        bool
	Snapshot     bool
	Sandbox      bool
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
	var version *bool
	if buildVersion != "" {
		version = flags.Bool("version", false, "print version, commit, and Go version and exit")
	}
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
	if version != nil && *version {
		fmt.Println(buildVersion)
		return nil
	}
	appFile := *file
	if appFile == "" && !*check && !*snapshot {
		if _, err := os.Stat("internal/assets/demo.lua"); err == nil {
			appFile = "internal/assets/demo.lua"
		}
	}
	return RunOptions(Options{AppFile: appFile, SocketPath: *socket, Check: *check, Snapshot: *snapshot, Sandbox: *sandbox})
}

func RunOptions(options Options) error {
	if _, err := reservedKeys(options.ReservedKeys); err != nil {
		return err
	}
	if options.Sandbox {
		return Sandbox(options)
	}
	loadSource := func() (string, error) { return readSource(options.AppFile) }
	source, err := loadSource()
	if err != nil {
		return err
	}
	r, err := engine.Load(source)
	if err != nil {
		return err
	}
	defer func() { r.Close() }()
	r.Theme = options.Theme
	if options.Check || options.Snapshot {
		s := tcell.NewSimulationScreen("UTF-8")
		if err = s.Init(); err != nil {
			return err
		}
		defer s.Fini()
		s.SetSize(100, 30)
		r.Draw(s, "")
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
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err = screen.Init(); err != nil {
		return err
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

func runScreen(options Options, screen tcell.Screen, r *engine.Runtime, readSource func() (string, error)) error {
	defer screen.Fini()
	defer func() { r.Close() }()
	bindings, err := reservedKeys(options.ReservedKeys)
	if err != nil {
		return err
	}
	screen.EnableMouse(tcell.MouseDragEvents)
	screen.EnablePaste()
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
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
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
		var source string
		source, err = readSource()
		if err == nil {
			var candidate *engine.Runtime
			candidate, err = r.Reload(source)
			if err == nil {
				r.Close()
				r = candidate
				diagnostic = ""
			}
		}
	}
	dispatch := func() bool {
		if options.OnEvent == nil {
			if options.SocketPath == "" {
				r.DrainEvents()
			}
			return false
		}
		batch := r.DrainEvents()
		for _, event := range batch {
			if e := options.OnEvent(r, event); e != nil {
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
	r.Draw(screen, diagnostic)
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
				ticker = time.NewTicker(16 * time.Millisecond)
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
		case <-signals:
			return nil
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
		case request := <-requests:
			var queued []engine.HostEvent
			if options.OnEvent != nil {
				queued, r.Events = r.Events, nil
			}
			reply := r.AgentReply(request.Data)
			if options.OnEvent != nil {
				r.Events = append(queued, r.Events...)
			}
			request.Reply <- reply
		case event := <-events:
			if event == nil {
				return nil
			}
			err = nil
			switch e := event.(type) {
			case *tcell.EventPaste:
				paste = e.Start()
				if paste {
					pasted.Reset()
				} else {
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
					err = r.Key(e)
				}
			case *tcell.EventMouse:
				err = r.Mouse(e)
			case *tcell.EventResize:
				screen.Sync()
			}
			redraw = true
		}
		if err != nil {
			diagnostic = err.Error()
		}
		called := dispatch()
		if redraw || called || revision != r.Revision || focus != r.Focus || previousDiagnostic != diagnostic {
			r.Draw(screen, diagnostic)
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

func validKeyName(name string) bool {
	name = normalizeKey(name)
	for bits := 0; bits < 16; bits++ {
		var modifiers tcell.ModMask
		for i, flag := range []tcell.ModMask{tcell.ModShift, tcell.ModCtrl, tcell.ModAlt, tcell.ModMeta} {
			if bits&(1<<i) != 0 {
				modifiers |= flag
			}
		}
		for key := range tcell.KeyNames {
			if normalizeKey(tcell.NewEventKey(key, 0, modifiers).Name()) == name {
				return true
			}
		}
		if start := strings.Index(name, "Rune["); start >= 0 && strings.HasSuffix(name, "]") {
			runes := []rune(name[start+5 : len(name)-1])
			if len(runes) == 1 && normalizeKey(tcell.NewEventKey(tcell.KeyRune, runes[0], modifiers).Name()) == name {
				return true
			}
		}
	}
	return false
}
