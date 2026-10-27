package tui

import (
	"context"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/engine"
	"github.com/VintlGvard/LugoTUI/internal/runner"
	"github.com/gdamore/tcell/v2"
)

type Runtime struct{ runtime *engine.Runtime }

type Node = engine.Node

type Rect = engine.Rect

type WidgetState = engine.WidgetState

type HostEvent = engine.HostEvent

type TaskResult = engine.TaskResult

type NodeJSON = engine.NodeJSON

type Theme = engine.Theme

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

func Load(source string) (*Runtime, error) {
	r, err := engine.Load(source)
	if err != nil {
		return nil, err
	}
	return &Runtime{runtime: r}, nil
}

func Demo() string { return engine.Demo() }

type Options struct {
	Theme        Theme
	OnEvent      func(*Runtime, HostEvent) error
	ReservedKeys *ReservedKeys
	AppFile      string
	SocketPath   string
	Check        bool
	Snapshot     bool
	Sandbox      bool
}

type ReservedKeys = runner.ReservedKeys

func Run(args []string) error { return runner.Run(args) }

func RunOptions(options Options) error {
	opts := runner.Options{
		Theme: options.Theme, ReservedKeys: options.ReservedKeys,
		AppFile: options.AppFile, SocketPath: options.SocketPath,
		Check: options.Check, Snapshot: options.Snapshot, Sandbox: options.Sandbox,
	}
	if options.OnEvent != nil {
		var wrapped *Runtime
		opts.OnEvent = func(r *engine.Runtime, event engine.HostEvent) error {
			if wrapped == nil || wrapped.runtime != r {
				wrapped = &Runtime{runtime: r}
			}
			return options.OnEvent(wrapped, event)
		}
	}
	return runner.RunOptions(opts)
}

func (r *Runtime) Revision() uint64 { return r.runtime.Revision }

func (r *Runtime) Focus() string { return r.runtime.Focus }

func (r *Runtime) SetFocus(id string) error { return r.runtime.SetFocus(id) }

func (r *Runtime) Theme() Theme { return r.runtime.Theme }

func (r *Runtime) SetTheme(theme Theme) { r.runtime.Theme = theme }

func (r *Runtime) Node(id string) *Node { return copyNode(r.runtime.Nodes[id]) }

func (r *Runtime) Model(key string) any { return copyValue(r.runtime.Data[key]) }

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

func (r *Runtime) StartTask(action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	return r.runtime.StartTask(action, work)
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
		panic("tui: unexpected internal data type")
	}
}
