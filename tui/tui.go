package tui

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/VintlGvard/LugoTUI/internal/runner"
	"github.com/gdamore/tcell/v2"
)

var ErrPanicRecovered = runner.ErrPanicRecovered

type Runtime struct{ runtime *engine.Runtime }

type Node = engine.Node

type Rect = engine.Rect

type WidgetState = engine.WidgetState

type HostEvent = engine.HostEvent

type TaskResult = engine.TaskResult

type NodeJSON = engine.NodeJSON

type Theme = engine.Theme

type Clipboard = engine.Clipboard

type OSC52Clipboard = engine.OSC52Clipboard

type StateObserver = engine.StateObserver

type InputFilter func(tcell.Event) tcell.Event

type EventHandler func(*Runtime, HostEvent) error

type ActionHandler func(*Runtime, any) error

type ViewFunc func(*Runtime) *Node

type EventMiddleware func(EventHandler) EventHandler

type Renderer interface {
	Draw(screen tcell.Screen, rt *Runtime, diagnostic string)
}

type RendererFunc func(screen tcell.Screen, rt *Runtime, diagnostic string)

func (f RendererFunc) Draw(screen tcell.Screen, rt *Runtime, diagnostic string) {
	f(screen, rt, diagnostic)
}

func NewOSC52Clipboard(out io.Writer) *OSC52Clipboard {
	return engine.NewOSC52Clipboard(out)
}

func DefaultTheme() Theme { return engine.DefaultTheme() }

const (
	CodeInvalidJSON       = engine.CodeInvalidJSON
	CodeUnknownCommand    = engine.CodeUnknownCommand
	CodeValidationError   = engine.CodeValidationError
	CodeRevisionConflict  = engine.CodeRevisionConflict
	CodeLuaError          = engine.CodeLuaError
	CodeTaskLimitExceeded = engine.CodeTaskLimitExceeded
	CodeInternalError     = engine.CodeInternalError
)

type ProtocolError = engine.ProtocolError

func ErrorCode(err error) string { return engine.ErrorCode(err) }

func Load(source string) (*Runtime, error) { return LoadNamed(source, "") }

func LoadNamed(source, name string) (*Runtime, error) {
	r, err := engine.LoadNamed(source, name)
	if err != nil {
		return nil, err
	}
	return &Runtime{runtime: r}, nil
}

func LoadView(root *Node, opts ...Option) (*Runtime, error) {
	var o Options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	var rt *Runtime
	goActions := make(map[string]func(*engine.Runtime, map[string]any) error, len(o.Actions))
	for name, fn := range o.Actions {
		if fn == nil {
			continue
		}
		fnCopy := fn
		goActions[name] = func(er *engine.Runtime, payload map[string]any) error {
			if rt == nil || rt.runtime != er {
				rt = &Runtime{runtime: er}
			}
			return fnCopy(rt, payload)
		}
	}
	r, err := engine.LoadNode(root, o.InitialData, goActions, o.AllowHostActions || o.OnEvent != nil)
	if err != nil {
		return nil, err
	}
	rt = &Runtime{runtime: r}
	applyOptionsToRuntime(rt, o)
	return rt, nil
}

func LoadViewFunc(fn ViewFunc, opts ...Option) (*Runtime, error) {
	if fn == nil {
		return nil, errors.New("nil view function")
	}
	var o Options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	var rt *Runtime
	goActions := make(map[string]func(*engine.Runtime, map[string]any) error, len(o.Actions))
	for name, h := range o.Actions {
		if h == nil {
			continue
		}
		hCopy := h
		goActions[name] = func(er *engine.Runtime, payload map[string]any) error {
			if rt == nil || rt.runtime != er {
				rt = &Runtime{runtime: er}
			}
			return hCopy(rt, payload)
		}
	}
	r, err := engine.LoadViewFunc(func(er *engine.Runtime) *engine.Node {
		if rt == nil || rt.runtime != er {
			rt = &Runtime{runtime: er}
		}
		return fn(rt)
	}, o.InitialData, goActions, o.AllowHostActions || o.OnEvent != nil)
	if err != nil {
		return nil, err
	}
	rt = &Runtime{runtime: r}
	applyOptionsToRuntime(rt, o)
	return rt, nil
}

func applyOptionsToRuntime(rt *Runtime, o Options) {
	if o.Theme != (Theme{}) {
		rt.SetTheme(o.Theme)
	}
	if o.Clipboard != nil {
		rt.SetClipboard(o.Clipboard)
	}
	for _, obs := range o.StateObservers {
		rt.AddStateObserver(obs)
	}
	if o.Inline {
		rt.SetInline(true, o.InlineHeight)
	} else if o.AltScreen != nil {
		rt.SetAltScreen(*o.AltScreen)
	}
}

func Demo() string { return engine.Demo() }

type Options struct {
	Theme                Theme
	View                 *Node
	ViewFunc             ViewFunc
	InitialData          map[string]any
	Actions              map[string]ActionHandler
	AllowHostActions     bool
	OnEvent              func(*Runtime, HostEvent) error
	Middlewares          []EventMiddleware
	InputFilter          InputFilter
	StateObservers       []StateObserver
	Renderer             Renderer
	Clipboard            Clipboard
	Screen               tcell.Screen
	ReservedKeys         *ReservedKeys
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
	OnReady              func(*Runtime)
}

type ReservedKeys = runner.ReservedKeys

func Run(args []string) error { return runner.Run(args) }

func toRunnerOptions(options Options, control <-chan runner.ControlMsg) runner.Options {
	var wrapped *Runtime
	wrapRuntime := func(r *engine.Runtime) *Runtime {
		if wrapped == nil || wrapped.runtime != r {
			wrapped = &Runtime{runtime: r}
		}
		return wrapped
	}
	opts := runner.Options{
		Theme:                options.Theme,
		View:                 options.View,
		InitialData:          options.InitialData,
		AllowHostActions:     options.AllowHostActions || options.OnEvent != nil || len(options.Actions) > 0 || options.View != nil || options.ViewFunc != nil,
		InputFilter:          options.InputFilter,
		StateObservers:       options.StateObservers,
		Clipboard:            options.Clipboard,
		Screen:               options.Screen,
		ReservedKeys:         options.ReservedKeys,
		AppFile:              options.AppFile,
		Source:               options.Source,
		SocketPath:           options.SocketPath,
		Check:                options.Check,
		Snapshot:             options.Snapshot,
		Sandbox:              options.Sandbox,
		Inline:               options.Inline,
		InlineHeight:         options.InlineHeight,
		AltScreen:            options.AltScreen,
		Mouse:                options.Mouse,
		BracketedPaste:       options.BracketedPaste,
		FPS:                  options.FPS,
		DisablePanicRecovery: options.DisablePanicRecovery,
		Control:              control,
	}
	if options.ViewFunc != nil {
		opts.ViewFunc = func(er *engine.Runtime) *engine.Node {
			return options.ViewFunc(wrapRuntime(er))
		}
	}
	if options.OnEvent != nil {
		opts.OnEvent = func(r *engine.Runtime, event engine.HostEvent) error {
			return options.OnEvent(wrapRuntime(r), event)
		}
	}
	if len(options.Middlewares) > 0 {
		opts.Middlewares = make([]runner.EventMiddleware, 0, len(options.Middlewares))
		for _, mw := range options.Middlewares {
			if mw == nil {
				continue
			}
			mwCopy := mw
			opts.Middlewares = append(opts.Middlewares, func(next runner.EventHandler) runner.EventHandler {
				wrappedNext := func(rt *Runtime, ev HostEvent) error {
					return next(rt.runtime, ev)
				}
				chained := mwCopy(wrappedNext)
				return func(er *engine.Runtime, ev engine.HostEvent) error {
					return chained(wrapRuntime(er), ev)
				}
			})
		}
	}
	if options.Renderer != nil {
		opts.Renderer = engine.RendererFunc(func(screen tcell.Screen, er *engine.Runtime, diagnostic string) {
			options.Renderer.Draw(screen, wrapRuntime(er), diagnostic)
		})
	}
	if options.OnReady != nil {
		opts.OnReady = func(er *engine.Runtime) {
			options.OnReady(wrapRuntime(er))
		}
	}
	return opts
}

func RunOptions(options Options) error {
	return runner.RunOptions(toRunnerOptions(options, nil))
}

func (r *Runtime) Revision() uint64 { return r.runtime.Revision }

func (r *Runtime) Focus() string { return r.runtime.Focus }

func (r *Runtime) SetFocus(id string) error { return r.runtime.SetFocus(id) }

func (r *Runtime) FocusDirection(dir string) bool { return r.runtime.FocusDirection(dir) }

func (r *Runtime) FocusNext() string { return r.runtime.FocusNext() }

func (r *Runtime) FocusPrev() string { return r.runtime.FocusPrev() }

func (r *Runtime) Theme() Theme { return r.runtime.Theme }

func (r *Runtime) SetTheme(theme Theme) { r.runtime.Theme = theme }

func (r *Runtime) Clipboard() Clipboard { return r.runtime.ClipboardAdapter() }

func (r *Runtime) SetClipboard(c Clipboard) { r.runtime.Clipboard = c }

func (r *Runtime) AddStateObserver(obs StateObserver) {
	if obs != nil {
		r.runtime.StateObservers = append(r.runtime.StateObservers, obs)
	}
}

func (r *Runtime) SetInline(inline bool, height int) {
	r.runtime.Inline = inline
	r.runtime.InlineHeight = height
	r.runtime.AltScreen = !inline
}

func (r *Runtime) Inline() (bool, int) { return r.runtime.Inline, r.runtime.InlineHeight }

func (r *Runtime) SetAltScreen(alt bool) {
	r.runtime.AltScreen = alt
	r.runtime.Inline = !alt
}

func (r *Runtime) AltScreen() bool { return r.runtime.AltScreen }

func (r *Runtime) Node(id string) *Node { return copyNode(r.runtime.Nodes[id]) }

func (r *Runtime) Model(key string) any { return copyValue(r.runtime.Data[key]) }

func (r *Runtime) Set(key string, value any) error {
	return r.runtime.Set(key, value)
}

func (r *Runtime) String(key string) string {
	v, _ := r.runtime.Data[key].(string)
	return v
}

func (r *Runtime) Int(key string) int {
	switch v := r.runtime.Data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (r *Runtime) Float(key string) float64 {
	switch v := r.runtime.Data[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func (r *Runtime) Bool(key string) bool {
	v, _ := r.runtime.Data[key].(bool)
	return v
}

func (r *Runtime) AddInt(key string, delta int) error {
	return r.Set(key, r.Int(key)+delta)
}

func (r *Runtime) ToggleBool(key string) error {
	return r.Set(key, !r.Bool(key))
}

func (r *Runtime) On(action string, handler ActionHandler) *Runtime {
	if handler == nil {
		return r
	}
	r.runtime.RegisterAction(action, func(er *engine.Runtime, payload map[string]any) error {
		return handler(r, payload)
	})
	return r
}

func (r *Runtime) Emit(action string, payload any) {
	r.runtime.Events = append(r.runtime.Events, HostEvent{Action: action, Payload: copyValue(payload)})
}

func (r *Runtime) ReconcileTree(root *Node) error {
	return r.runtime.ReconcileTree(root)
}

func (r *Runtime) State(id string) WidgetState {
	s := r.runtime.State[id]
	if s == nil {
		return WidgetState{}
	}
	return WidgetState{Value: s.Value, Cursor: s.Cursor, Anchor: s.Anchor,
		Scroll: s.Scroll, Selected: s.Selected,
		Undo: append([]string(nil), s.Undo...), Redo: append([]string(nil), s.Redo...)}
}

func (r *Runtime) Action(name string, event map[string]any) error {
	return r.runtime.Action(name, event)
}

func (r *Runtime) Patch(id string, props map[string]any) error { return r.runtime.Patch(id, props) }

func (r *Runtime) Replace(id string, subtree *Node) error { return r.runtime.Replace(id, subtree) }

func (r *Runtime) ApplyJSON(data []byte) error { return r.runtime.ApplyJSON(data) }

func (r *Runtime) Reload(source string) (*Runtime, error) {
	next, err := r.runtime.Reload(source)
	if err != nil {
		return nil, err
	}
	return &Runtime{runtime: next}, nil
}

func (r *Runtime) ReloadNamed(source, name string) (*Runtime, error) {
	next, err := r.runtime.ReloadNamed(source, name)
	if err != nil {
		return nil, err
	}
	return &Runtime{runtime: next}, nil
}

func (r *Runtime) Close() { r.runtime.Close() }

func (r *Runtime) Draw(screen tcell.Screen, diagnostic string) { r.runtime.Draw(screen, diagnostic) }

func (r *Runtime) Layout(w, h int) error { return engine.Layout(r.runtime.Root, w, h) }

func (r *Runtime) Key(event *tcell.EventKey) error { return r.runtime.Key(event) }

func (r *Runtime) Mouse(event *tcell.EventMouse) error { return r.runtime.Mouse(event) }

func (r *Runtime) Paste(text string) error { return r.runtime.Paste(text) }

func (r *Runtime) NeedsTick() bool { return r.runtime.NeedsTick() }

func (r *Runtime) Tick(now time.Time) error { return r.runtime.Tick(now) }

func (r *Runtime) AgentReply(data []byte) []byte { return r.runtime.AgentReply(data) }

func (r *Runtime) Snapshot() ([]byte, error) { return r.runtime.Snapshot() }

func (r *Runtime) ReconcileFocus() { r.runtime.ReconcileFocus() }

func (r *Runtime) CancelTask(id uint64) { r.runtime.CancelTask(id) }

func (r *Runtime) CancelTaskNamed(name string) { r.runtime.CancelTaskNamed(name) }

func (r *Runtime) StartTask(action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	return r.runtime.StartTask(action, work)
}

func (r *Runtime) StartTaskNamed(name, action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	return r.runtime.StartTaskNamed(name, action, work)
}

func (r *Runtime) StartTaskDebounced(name string, delay time.Duration, action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	return r.runtime.StartTaskDebounced(name, delay, action, work)
}

func (r *Runtime) TaskResults() <-chan TaskResult { return r.runtime.TaskResults() }

func (r *Runtime) HandleTask(result TaskResult) error { return r.runtime.HandleTask(result) }

func (r *Runtime) DrainEvents() []HostEvent { return r.runtime.DrainEvents() }

func (r *Runtime) Items(node *Node) []any {
	if node == nil {
		return nil
	}
	items := r.runtime.Items(node)
	if items == nil {
		return nil
	}
	return copyValue(items).([]any)
}

func copyNode(node *Node) *Node {
	if node == nil {
		return nil
	}
	copy := &Node{Type: node.Type, ID: node.ID, Rect: node.Rect, Clip: node.Clip}
	if node.Props != nil {
		copy.Props = copyValue(node.Props).(map[string]any)
	}
	for _, child := range node.Children {
		copy.Children = append(copy.Children, copyNode(child))
	}
	return copy
}

func copyValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		copy := make(map[string]any, len(value))
		for key, item := range value {
			copy[key] = copyValue(item)
		}
		return copy
	case []any:
		copy := make([]any, len(value))
		for i, item := range value {
			copy[i] = copyValue(item)
		}
		return copy
	case []string:
		return append([]string(nil), value...)
	case nil, string, bool, int, int64, float64:
		return value
	default:
		return nil
	}
}
