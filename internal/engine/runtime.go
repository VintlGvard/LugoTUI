package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/assets"
	lua "github.com/yuin/gopher-lua"
)

type Runtime struct {
	L                              *lua.LState
	Root                           *Node
	Nodes                          map[string]*Node
	State                          map[string]*WidgetState
	Data                           map[string]any
	Focus                          string
	Theme                          Theme
	Revision                       uint64
	AnimationRevision              uint64
	actions                        *lua.LTable
	keymap                         map[string]string
	Staged                         []Operation
	Events                         []HostEvent
	modalID, PreviousFocus, dragID string
	mouseDown                      bool
	tasks                          *taskState
	stagedFocus                    *string
}

type HostEvent struct {
	Action  string `json:"action"`
	Payload any    `json:"payload"`
}

func Load(source string) (*Runtime, error) { return loadRuntime(source, nil) }

func (r *Runtime) Reload(source string) (*Runtime, error) {
	if r.L == nil {
		return nil, errors.New("runtime closed")
	}
	return loadRuntime(source, r)
}

func (r *Runtime) Close() {
	if r.tasks != nil {
		for id := range r.tasks.active {
			r.CancelTask(id)
		}
		close(r.tasks.done)
		r.tasks = nil
	}
	if r.L != nil {
		r.L.Close()
		r.L = nil
	}
}

func (r *Runtime) DrainEvents() []HostEvent { events := r.Events; r.Events = nil; return events }

func Demo() string { return assets.DemoSource }

func (r *Runtime) restore(old *Runtime) {
	for id, n := range r.Nodes {
		s := &WidgetState{Anchor: -1, Value: str(n, "text", "")}
		if n.Type != "input" && n.Type != "textarea" {
			s.Value = ""
		}
		if old != nil && old.Nodes[id] != nil && old.Nodes[id].Type == n.Type && old.State[id] != nil {
			copy := *old.State[id]
			copy.Undo = append([]string(nil), copy.Undo...)
			copy.Redo = append([]string(nil), copy.Redo...)
			copy.collection = cloneCollectionState(copy.collection)
			copy.primitive = clonePrimitiveState(copy.primitive)
			copy.clusters = nil
			copy.positions = nil
			copy.indexed = false
			s = &copy
		}
		r.State[id] = s
		if b := str(n, "bind", ""); b != "" {
			if v, ok := r.Data[b].(string); ok {
				s.Value = v
			} else {
				r.Data[b] = s.Value
			}
		}
	}
	r.Focus = ""
	for n := range visibleNodes(r.Root) {
		if !focusable(n) {
			continue
		}
		if r.Focus == "" {
			r.Focus = n.ID
		}
		if old != nil && n.ID == old.Focus {
			r.Focus = n.ID
			break
		}
	}
}

func (r *Runtime) SetFocus(id string) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	if id == "" {
		r.Focus = ""
		return nil
	}
	n := r.Nodes[id]
	if n == nil || !focusable(n) {
		return fmt.Errorf("invalid focus: %s (expected visible, enabled focusable node)", id)
	}
	visible := false
	for node := range visibleNodes(r.Root) {
		if node == n {
			visible = true
			break
		}
	}
	if !visible {
		return fmt.Errorf("invalid focus: %s (hidden ancestor)", id)
	}
	scope := r.scope()
	if scope != r.Root {
		insideModal := false
		for node := range ordered(scope) {
			if node == n {
				insideModal = true
				break
			}
		}
		if !insideModal {
			return fmt.Errorf("invalid focus: %s (outside active modal)", id)
		}
	}
	r.Focus = id
	return nil
}

func (r *Runtime) Patch(id string, props map[string]any) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	r.Staged = []Operation{{ID: id, Props: props}}
	return r.Commit()
}

func (r *Runtime) Replace(id string, subtree *Node) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	r.Staged = []Operation{{ID: id, Tree: subtree, Replace: true}}
	return r.Commit()
}

func (r *Runtime) Commit() error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	if len(r.Staged) == 0 && r.stagedFocus == nil {
		return nil
	}
	defer func() { r.Staged = nil; r.stagedFocus = nil }()
	root := r.Root
	if len(r.Staged) > 0 {
		root = cloneNode(r.Root)
	}
	origins := map[*Node]int{}
	properties := map[*Node]map[string]int{}
	operationError := func(err error) error {
		var detail *nodeError
		if errors.As(err, &detail) {
			i := origins[detail.Node]
			if p := properties[detail.Node]; p != nil && p[detail.Property] != 0 {
				i = p[detail.Property]
			}
			if i == 0 && detail.Node != nil {
				for node, origin := range origins {
					if node.ID == detail.Node.ID && origin > i {
						i = origin
					}
				}
			}
			if i != 0 {
				return fmt.Errorf("operations[%d]: %w", i, err)
			}
		}
		return err
	}
	for i, op := range r.Staged {
		n := findNode(root, op.ID)
		if n == nil {
			return fmt.Errorf("operations[%d]: unknown id %q", i+1, op.ID)
		}
		if op.Replace {
			count := 0
			var checkShape func(*Node, int) error
			checkShape = func(node *Node, depth int) error {
				if node == nil {
					return errors.New("nil node")
				}
				count++
				if count > maxNodes || depth > 64 {
					return errors.New("tree exceeds limits")
				}
				for _, child := range node.Children {
					if err := checkShape(child, depth+1); err != nil {
						return err
					}
				}
				return nil
			}
			if err := checkShape(op.Tree, 0); err != nil {
				return fmt.Errorf("operations[%d]: %w", i+1, err)
			}
			var replace func(**Node)
			replace = func(p **Node) {
				if (*p).ID == op.ID {
					*p = cloneNode(op.Tree)
					for added := range ordered(*p) {
						origins[added] = i + 1
					}
					return
				}
				for i := range (*p).Children {
					replace(&(*p).Children[i])
				}
			}
			replace(&root)
		} else {
			if properties[n] == nil {
				properties[n] = map[string]int{}
			}
			for k, v := range op.Props {
				n.Props[k] = v
				properties[n][k] = i + 1
			}
		}
	}
	ids, e := validateTree(root)
	if e != nil {
		return operationError(e)
	}
	if len(r.Staged) > 0 {
		for node := range ordered(root) {
			for key, value := range node.Props {
				cloned, err := cloneValue(value, 0)
				if err != nil {
					return operationError(&nodeError{Node: node, Property: key, Err: err})
				}
				node.Props[key] = cloned
			}
		}
	}
	candidate := *r
	candidate.Root = root
	if e := candidate.validateActions(); e != nil {
		return operationError(e)
	}
	candidate.Nodes = ids
	candidate.State = map[string]*WidgetState{}
	candidate.Data = maps.Clone(r.Data)
	candidate.restore(r)
	if e := ValidateModel(candidate.Data); e != nil {
		return e
	}
	candidate.ReconcileFocus()
	if r.stagedFocus != nil {
		id := *r.stagedFocus
		n := candidate.Nodes[id]
		if n == nil || !focusable(n) {
			return fmt.Errorf("ui.focus: invalid focus: %s (expected visible, enabled focusable node)", id)
		}
		visible := false
		for node := range visibleNodes(candidate.Root) {
			if node == n {
				visible = true
				break
			}
		}
		if !visible {
			return fmt.Errorf("ui.focus: invalid focus: %s (hidden ancestor)", id)
		}
		candidate.Focus = id
		if r.modalID != "" {
			insideModal := false
			if modal := candidate.Nodes[r.modalID]; modal != nil {
				for node := range ordered(modal) {
					if node == n {
						insideModal = true
						break
					}
				}
			}
			if !insideModal {
				candidate.PreviousFocus = id
			}
		}
	}
	candidate.Revision++
	*r = candidate
	return nil
}

func (r *Runtime) Action(name string, event map[string]any) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	if name == "" {
		return nil
	}
	if r.actions == nil {
		return fmt.Errorf("unknown action %s", name)
	}
	f, ok := r.actions.RawGetString(name).(*lua.LFunction)
	if !ok {
		return fmt.Errorf("unknown action %s", name)
	}
	payload, err := toLua(r.L, event, "payload")
	if err != nil {
		return err
	}
	oldFocus := r.Focus
	oldPrevious := r.PreviousFocus
	oldData := r.Data
	r.Data = maps.Clone(oldData)
	oldEvents := len(r.Events)
	r.Staged = nil
	r.stagedFocus = nil
	committed := false
	defer func() {
		if !committed {
			r.Data = oldData
			r.PreviousFocus = oldPrevious
			r.Staged = nil
			r.stagedFocus = nil
			r.Events = r.Events[:oldEvents]
			r.Focus = oldFocus
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.L.SetContext(ctx)
	defer r.L.RemoveContext()
	if e := r.L.CallByParam(lua.P{Fn: f, Protect: true}, payload); e != nil {
		return e
	}
	if e := ValidateModel(r.Data); e != nil {
		return e
	}
	if e := r.Commit(); e != nil {
		return e
	}
	for id, n := range r.Nodes {
		if n.Type == "input" || n.Type == "textarea" {
			if key := str(n, "bind", ""); key != "" {
				if v, ok := r.Data[key].(string); ok && v != r.State[id].Value {
					r.State[id].Value = v
					r.State[id].Cursor = min(r.State[id].Cursor, len(graphemes(v)))
					r.State[id].Anchor = -1
				}
			}
		}
	}
	r.Revision++
	committed = true
	return nil
}
