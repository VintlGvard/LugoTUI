package tui

import (
	"errors"
	"sync"

	"github.com/VintlGvard/LugoTUI/internal/runner"
	"github.com/gdamore/tcell/v2"
)

type Option func(*Options)

func WithAppFile(path string) Option {
	return func(o *Options) { o.AppFile = path }
}

func WithSource(luaSource string) Option {
	return func(o *Options) { o.Source = luaSource }
}

func WithView(root *Node) Option {
	return func(o *Options) { o.View = root }
}

func WithViewFunc(fn ViewFunc) Option {
	return func(o *Options) { o.ViewFunc = fn }
}

func WithInitialData(data map[string]any) Option {
	return func(o *Options) {
		if o.InitialData == nil {
			o.InitialData = make(map[string]any, len(data))
		}
		for k, v := range data {
			o.InitialData[k] = v
		}
	}
}

func WithState(key string, val any) Option {
	return func(o *Options) {
		if o.InitialData == nil {
			o.InitialData = map[string]any{}
		}
		o.InitialData[key] = val
	}
}

func WithTheme(theme Theme) Option {
	return func(o *Options) { o.Theme = theme }
}

func WithOnEvent(fn EventHandler) Option {
	return func(o *Options) {
		o.OnEvent = fn
		o.AllowHostActions = true
	}
}

func WithAction(name string, fn ActionHandler) Option {
	if name == "" || fn == nil {
		return func(*Options) {}
	}
	return WithHandlers(map[string]ActionHandler{name: fn})
}

func WithHandlers(handlers map[string]ActionHandler) Option {
	return func(o *Options) {
		if o.Actions == nil {
			o.Actions = make(map[string]ActionHandler, len(handlers))
		}
		for k, v := range handlers {
			if v != nil {
				o.Actions[k] = v
			}
		}
		o.AllowHostActions = true
		prev := o.OnEvent
		o.OnEvent = func(r *Runtime, ev HostEvent) error {
			if h, ok := handlers[ev.Action]; ok && h != nil {
				if err := h(r, ev.Payload); err != nil {
					return err
				}
			}
			if prev != nil {
				return prev(r, ev)
			}
			return nil
		}
	}
}

func WithMiddleware(mw ...EventMiddleware) Option {
	return func(o *Options) {
		o.Middlewares = append(o.Middlewares, mw...)
	}
}

func WithInputFilter(filter InputFilter) Option {
	return func(o *Options) { o.InputFilter = filter }
}

func WithStateObserver(obs ...StateObserver) Option {
	return func(o *Options) {
		o.StateObservers = append(o.StateObservers, obs...)
	}
}

func WithRenderer(renderer Renderer) Option {
	return func(o *Options) { o.Renderer = renderer }
}

func WithRendererFunc(fn func(screen tcell.Screen, rt *Runtime, diagnostic string)) Option {
	return func(o *Options) { o.Renderer = RendererFunc(fn) }
}

func WithClipboard(clipboard Clipboard) Option {
	return func(o *Options) { o.Clipboard = clipboard }
}

func WithScreen(screen tcell.Screen) Option {
	return func(o *Options) { o.Screen = screen }
}

func WithReservedKeys(keys ReservedKeys) Option {
	return func(o *Options) { o.ReservedKeys = &keys }
}

func WithSocket(path string) Option {
	return func(o *Options) { o.SocketPath = path }
}

func WithCheck() Option {
	return func(o *Options) { o.Check = true }
}

func WithSnapshot() Option {
	return func(o *Options) { o.Snapshot = true }
}

func WithSandbox() Option {
	return func(o *Options) { o.Sandbox = true }
}

func WithAltScreen(enabled bool) Option {
	return func(o *Options) {
		v := enabled
		o.AltScreen = &v
		if enabled {
			o.Inline = false
		}
	}
}

func WithInline(height int) Option {
	return func(o *Options) {
		o.Inline = true
		o.InlineHeight = height
		v := false
		o.AltScreen = &v
	}
}

func WithMouse(enabled bool) Option {
	return func(o *Options) {
		v := enabled
		o.Mouse = &v
	}
}

func WithPaste(enabled bool) Option {
	return func(o *Options) {
		v := enabled
		o.BracketedPaste = &v
	}
}

func WithFPS(fps int) Option {
	return func(o *Options) { o.FPS = fps }
}

func WithoutPanicRecovery() Option {
	return func(o *Options) { o.DisablePanicRecovery = true }
}

func WithOnReady(fn func(*Runtime)) Option {
	return func(o *Options) { o.OnReady = fn }
}

type Program struct {
	options Options
	control chan runner.ControlMsg
	done    chan struct{}
	once    sync.Once
}

func NewProgram(opts ...Option) *Program {
	var o Options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return &Program{
		options: o,
		control: make(chan runner.ControlMsg, 32),
		done:    make(chan struct{}),
	}
}

func (p *Program) Run() error {
	defer p.once.Do(func() { close(p.done) })
	return runner.RunOptions(toRunnerOptions(p.options, p.control))
}

func (p *Program) sendControl(msg runner.ControlMsg) error {
	msg.Reply = make(chan error, 1)
	select {
	case <-p.done:
		return errors.New("program stopped")
	case p.control <- msg:
	}
	select {
	case <-p.done:
		return errors.New("program stopped")
	case err := <-msg.Reply:
		return err
	}
}

func (p *Program) Send(action string, payload map[string]any) error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlAction, Action: action, Payload: payload})
}

func (p *Program) Set(key string, value any) error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlSet, Key: key, Value: value})
}

func (p *Program) Patch(id string, props map[string]any) error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlPatch, ID: id, Props: props})
}

func (p *Program) Reload() error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlReload})
}

func (p *Program) EnterAltScreen() error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlAltScreen, AltScreen: true})
}

func (p *Program) ExitAltScreen() error {
	return p.sendControl(runner.ControlMsg{Kind: runner.ControlAltScreen, AltScreen: false})
}

func (p *Program) Quit() {
	select {
	case <-p.done:
	case p.control <- runner.ControlMsg{Kind: runner.ControlQuit}:
	}
}

func RunApp(appFile string, opts ...Option) error {
	all := make([]Option, 0, len(opts)+1)
	all = append(all, WithAppFile(appFile))
	all = append(all, opts...)
	return NewProgram(all...).Run()
}

func RunSource(luaSource string, opts ...Option) error {
	all := make([]Option, 0, len(opts)+1)
	all = append(all, WithSource(luaSource))
	all = append(all, opts...)
	return NewProgram(all...).Run()
}

func QuickRun(luaSource string, handlers map[string]ActionHandler, opts ...Option) error {
	all := make([]Option, 0, len(opts)+2)
	all = append(all, WithSource(luaSource), WithHandlers(handlers))
	all = append(all, opts...)
	return NewProgram(all...).Run()
}

func QuickView(root *Node, handlers map[string]ActionHandler, opts ...Option) error {
	all := make([]Option, 0, len(opts)+2)
	all = append(all, WithView(root), WithHandlers(handlers))
	all = append(all, opts...)
	return NewProgram(all...).Run()
}

func QuickViewFunc(fn ViewFunc, handlers map[string]ActionHandler, opts ...Option) error {
	all := make([]Option, 0, len(opts)+2)
	all = append(all, WithViewFunc(fn), WithHandlers(handlers))
	all = append(all, opts...)
	return NewProgram(all...).Run()
}
