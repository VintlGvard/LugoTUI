package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VintlGvard/LugoTUI/internal/assets"
	lua "github.com/yuin/gopher-lua"
)

var ErrQuit = errors.New("lugotui: quit")

type GoActionFunc = func(rt *Runtime, payload map[string]any) error

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
	spatialNav                     bool
	Staged                         []Operation
	Events                         []HostEvent
	modalID, PreviousFocus, dragID string
	mouseDown                      bool
	tasks                          *taskState
	stagedFocus                    *string
	ChunkName                      string
	Clipboard                      Clipboard
	StateObservers                 []StateObserver
	Inline                         bool
	InlineHeight                   int
	AltScreen                      bool
	closed                         bool
	inAction                       bool
	GoActions                      map[string]func(*Runtime, map[string]any) error
	AllowHostActions               bool
	ViewFunc                       func(*Runtime) *Node
	cellBuf                        *cellBuffer
	layoutValid                    bool
	lastLayoutW                    int
	lastLayoutH                    int
	lastLayoutRev                  uint64
	lastScrollDigest               uint64
}

type HostEvent struct {
	Action  string `json:"action"`
	Payload any    `json:"payload"`
}

func (r *Runtime) isClosed() bool { return r == nil || r.closed }

func (r *Runtime) hasLuaAction(name string) bool {
	return r.actions != nil && r.actions.RawGetString(name).Type() == lua.LTFunction
}

func (r *Runtime) hasGoAction(name string) bool {
	return r.GoActions != nil && r.GoActions[name] != nil
}

func (r *Runtime) hasAction(name string) bool {
	if name == "" {
		return false
	}
	return r.hasLuaAction(name) || r.hasGoAction(name) || r.AllowHostActions
}

func cloneGoActions(src map[string]func(*Runtime, map[string]any) error) map[string]func(*Runtime, map[string]any) error {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]func(*Runtime, map[string]any) error, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (r *Runtime) RegisterAction(name string, fn func(*Runtime, map[string]any) error) {
	if name == "" || fn == nil {
		return
	}
	if r.GoActions == nil {
		r.GoActions = map[string]func(*Runtime, map[string]any) error{}
	}
	r.GoActions[name] = fn
}

func Load(source string) (*Runtime, error) { return loadRuntime(source, "", nil) }

func LoadNamed(source, chunkName string) (*Runtime, error) {
	return loadRuntime(source, chunkName, nil)
}

func LoadWithConfig(source, chunkName string, initialData map[string]any, goActions map[string]func(*Runtime, map[string]any) error, allowHostActions bool) (*Runtime, error) {
	var seed *Runtime
	if len(initialData) > 0 || len(goActions) > 0 || allowHostActions {
		seed = &Runtime{
			Data:             cloneData(initialData),
			ChunkName:        chunkName,
			AltScreen:        true,
			GoActions:        cloneGoActions(goActions),
			AllowHostActions: allowHostActions,
		}
	}
	return loadRuntime(source, chunkName, seed)
}

func LoadNode(root *Node, data map[string]any, goActions map[string]func(*Runtime, map[string]any) error, allowHostActions bool) (*Runtime, error) {
	if root == nil {
		return nil, errors.New("view root is nil")
	}
	clonedRoot := cloneNode(root)
	AutoAssignIDs(clonedRoot)
	r := &Runtime{
		Root:             clonedRoot,
		State:            map[string]*WidgetState{},
		Data:             cloneData(data),
		ChunkName:        "<go>",
		AltScreen:        true,
		GoActions:        cloneGoActions(goActions),
		AllowHostActions: allowHostActions,
	}
	r.bindInlineActions(clonedRoot)
	if err := sanitizeGoTreeProps(clonedRoot); err != nil {
		return nil, err
	}
	nodes, err := validateTree(clonedRoot)
	if err != nil {
		return nil, err
	}
	r.Nodes = nodes
	if err := r.validateActions(); err != nil {
		return nil, err
	}
	r.restore(nil)
	if err := ValidateModel(r.Data); err != nil {
		return nil, err
	}
	return r, nil
}

func LoadViewFunc(viewFn func(*Runtime) *Node, data map[string]any, goActions map[string]func(*Runtime, map[string]any) error, allowHostActions bool) (*Runtime, error) {
	if viewFn == nil {
		return nil, errors.New("view function is nil")
	}
	r := &Runtime{
		Nodes:            map[string]*Node{},
		State:            map[string]*WidgetState{},
		Data:             cloneData(data),
		ChunkName:        "<go-view>",
		AltScreen:        true,
		GoActions:        cloneGoActions(goActions),
		AllowHostActions: allowHostActions,
		ViewFunc:         viewFn,
	}
	root := viewFn(r)
	if root == nil {
		return nil, errors.New("view function returned nil root")
	}
	if err := r.reconcileRoot(root); err != nil {
		return nil, err
	}
	return r, nil
}

func sanitizeGoTreeProps(root *Node) error {
	for node := range ordered(root) {
		for key, value := range node.Props {
			cloned, err := cloneValue(value, 0)
			if err != nil {
				return &nodeError{Node: node, Property: key, Err: err}
			}
			node.Props[key] = cloned
		}
	}
	return nil
}

func (r *Runtime) reconcileRoot(next *Node) error {
	if next == nil {
		return errors.New("nil view root")
	}
	cloned := cloneNode(next)
	AutoAssignIDs(cloned)
	candidate := *r
	candidate.GoActions = cloneGoActions(r.GoActions)
	candidate.bindInlineActions(cloned)
	if err := sanitizeGoTreeProps(cloned); err != nil {
		return err
	}
	ids, err := validateTree(cloned)
	if err != nil {
		return err
	}
	candidate.Root = cloned
	if err := candidate.validateActions(); err != nil {
		return err
	}
	candidate.Nodes = ids
	candidate.State = map[string]*WidgetState{}
	candidate.Data = cloneData(r.Data)
	candidate.restore(r)
	if err := ValidateModel(candidate.Data); err != nil {
		return err
	}
	candidate.ReconcileFocus()
	candidate.Revision = r.Revision + 1
	candidate.layoutValid = false
	*r = candidate
	return nil
}

func (r *Runtime) syncBoundWidgets() {
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
}

func (r *Runtime) ReconcileTree(next *Node) error {
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	return r.reconcileRoot(next)
}

func (r *Runtime) Set(key string, value any) error {
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	if key == "" {
		return errors.New("empty model key")
	}
	cloned, err := cloneValue(value, 0)
	if err != nil {
		return err
	}
	if r.inAction {
		if cloned == nil {
			delete(r.Data, key)
		} else {
			r.Data[key] = cloned
		}
		return nil
	}
	oldVal, existed := r.Data[key]
	if cloned == nil {
		delete(r.Data, key)
	} else {
		r.Data[key] = cloned
	}
	if err := ValidateModel(r.Data); err != nil {
		if existed {
			r.Data[key] = oldVal
		} else {
			delete(r.Data, key)
		}
		return err
	}
	r.syncBoundWidgets()
	if r.ViewFunc != nil {
		if err := r.reconcileRoot(r.ViewFunc(r)); err != nil {
			if existed {
				r.Data[key] = oldVal
			} else {
				delete(r.Data, key)
			}
			r.syncBoundWidgets()
			return err
		}
	} else {
		r.Revision++
	}
	if !existed || !equalDataVal(oldVal, cloned) {
		r.notifyState(key, oldVal, cloned)
	}
	return nil
}

func (r *Runtime) Update(patch map[string]any) error {
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	if len(patch) == 0 {
		return nil
	}
	if r.inAction {
		for k, v := range patch {
			if err := r.Set(k, v); err != nil {
				return err
			}
		}
		return nil
	}
	oldData := cloneData(r.Data)
	for k, v := range patch {
		if k == "" {
			r.Data = oldData
			return errors.New("empty model key")
		}
		cloned, err := cloneValue(v, 0)
		if err != nil {
			r.Data = oldData
			return err
		}
		if cloned == nil {
			delete(r.Data, k)
		} else {
			r.Data[k] = cloned
		}
	}
	if err := ValidateModel(r.Data); err != nil {
		r.Data = oldData
		return err
	}
	r.syncBoundWidgets()
	if r.ViewFunc != nil {
		if err := r.reconcileRoot(r.ViewFunc(r)); err != nil {
			r.Data = oldData
			r.syncBoundWidgets()
			return err
		}
	} else {
		r.Revision++
	}
	for k := range patch {
		oldVal := oldData[k]
		newVal := r.Data[k]
		if !equalDataVal(oldVal, newVal) {
			r.notifyState(k, oldVal, newVal)
		}
	}
	return nil
}

func (r *Runtime) String(key string, fallback ...string) string {
	if v, ok := r.Data[key].(string); ok {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

func (r *Runtime) Int(key string, fallback ...int) int {
	switch v := r.Data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}

func (r *Runtime) Float(key string, fallback ...float64) float64 {
	if f, ok := floatVal(r.Data[key]); ok {
		return f
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}

func (r *Runtime) Bool(key string, fallback ...bool) bool {
	if v, ok := r.Data[key].(bool); ok {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return false
}

func (r *Runtime) Strings(key string) []string {
	switch v := r.Data[key].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (r *Runtime) Map(key string) map[string]any {
	if m, ok := r.Data[key].(map[string]any); ok {
		if cloned, err := cloneValue(m, 0); err == nil {
			if cm, ok := cloned.(map[string]any); ok {
				return cm
			}
		}
	}
	return nil
}

func (r *Runtime) AddInt(key string, delta int) error {
	return r.Set(key, r.Int(key)+delta)
}

func (r *Runtime) ToggleBool(key string) error {
	return r.Set(key, !r.Bool(key))
}

func (r *Runtime) Toggle(key string) bool {
	next := !r.Bool(key)
	_ = r.Set(key, next)
	return next
}

func (r *Runtime) Quit() error {
	return ErrQuit
}

func (r *Runtime) Reload(source string) (*Runtime, error) {
	if r.isClosed() {
		return nil, errors.New("runtime closed")
	}
	if source == "" && r.ViewFunc != nil {
		if err := r.reconcileRoot(r.ViewFunc(r)); err != nil {
			return nil, err
		}
		return r, nil
	}
	return loadRuntime(source, r.ChunkName, r)
}

func (r *Runtime) ReloadNamed(source, chunkName string) (*Runtime, error) {
	if r.isClosed() {
		return nil, errors.New("runtime closed")
	}
	if source == "" && r.ViewFunc != nil {
		if err := r.reconcileRoot(r.ViewFunc(r)); err != nil {
			return nil, err
		}
		return r, nil
	}
	return loadRuntime(source, chunkName, r)
}

func (r *Runtime) Close() {
	r.closed = true
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
	if r.isClosed() {
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

func (r *Runtime) FocusDirection(dir string) bool {
	return r.focusDirection(dir)
}

func (r *Runtime) FocusNext() string {
	next := r.nextFocusable(false)
	if next != "" {
		r.Focus = next
	}
	return next
}

func (r *Runtime) FocusPrev() string {
	prev := r.nextFocusable(true)
	if prev != "" {
		r.Focus = prev
	}
	return prev
}

func (r *Runtime) Patch(id string, props map[string]any) error {
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	r.Staged = append(r.Staged, Operation{ID: id, Props: props})
	if r.inAction {
		return nil
	}
	return r.Commit()
}

func (r *Runtime) Replace(id string, subtree *Node) error {
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	r.Staged = append(r.Staged, Operation{ID: id, Tree: subtree, Replace: true})
	if r.inAction {
		return nil
	}
	return r.Commit()
}

func (r *Runtime) Commit() error {
	if r.isClosed() {
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
					clonedSub := cloneNode(op.Tree)
					if clonedSub.ID == "" {
						clonedSub.ID = op.ID
					}
					AutoAssignIDs(clonedSub)
					*p = clonedSub
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
	candidate := *r
	candidate.GoActions = cloneGoActions(r.GoActions)
	candidate.bindInlineActions(root)
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
	candidate.Root = root
	if e := candidate.validateActions(); e != nil {
		return operationError(e)
	}
	candidate.Nodes = ids
	candidate.State = map[string]*WidgetState{}
	candidate.Data = cloneData(r.Data)
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
	if r.isClosed() {
		return errors.New("runtime closed")
	}
	if name == "" {
		return nil
	}
	if !r.hasAction(name) {
		return fmt.Errorf("unknown action %s", name)
	}
	oldFocus := r.Focus
	oldPrevious := r.PreviousFocus
	oldModal := r.modalID
	oldData := r.Data
	r.Data = cloneData(oldData)
	oldGlobals := snapshotLuaGlobals(r.L)
	oldEvents := len(r.Events)
	r.Staged = nil
	r.stagedFocus = nil
	r.inAction = true
	committed := false
	defer func() {
		r.inAction = false
		if !committed {
			r.Data = oldData
			restoreLuaGlobals(r.L, oldGlobals)
			r.PreviousFocus = oldPrevious
			r.modalID = oldModal
			r.Staged = nil
			r.stagedFocus = nil
			r.Events = r.Events[:oldEvents]
			r.Focus = oldFocus
		}
	}()
	ranAny := false
	if r.hasLuaAction(name) {
		f := r.actions.RawGetString(name).(*lua.LFunction)
		payload, err := toLua(r.L, event, "payload")
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		r.L.SetContext(ctx)
		defer r.L.RemoveContext()
		if e := r.L.CallByParam(lua.P{Fn: f, Protect: true}, payload); e != nil {
			return e
		}
		ranAny = true
	}
	quitRequested := false
	if !ranAny && r.hasGoAction(name) {
		if e := r.GoActions[name](r, event); e != nil {
			if errors.Is(e, ErrQuit) {
				quitRequested = true
			} else {
				return e
			}
		}
		ranAny = true
	}
	if !ranAny && r.AllowHostActions {
		clonedEvent, err := cloneValue(event, 0)
		if err != nil {
			return err
		}
		r.Events = append(r.Events, HostEvent{Action: name, Payload: clonedEvent})
	}
	if e := ValidateModel(r.Data); e != nil {
		return e
	}
	if e := r.Commit(); e != nil {
		return e
	}
	if r.ViewFunc != nil {
		if e := r.reconcileRoot(r.ViewFunc(r)); e != nil {
			return e
		}
	}
	r.syncBoundWidgets()
	r.Revision++
	committed = true
	if len(r.StateObservers) > 0 {
		for k, v := range r.Data {
			if oldVal, exists := oldData[k]; !exists || !equalDataVal(oldVal, v) {
				r.notifyState(k, oldVal, v)
			}
		}
		for k, oldVal := range oldData {
			if _, exists := r.Data[k]; !exists {
				r.notifyState(k, oldVal, nil)
			}
		}
	}
	if quitRequested {
		return ErrQuit
	}
	return nil
}

func equalDataVal(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		return false
	}
}

func cloneData(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		if cloned, err := cloneValue(v, 0); err == nil {
			out[k] = cloned
		} else {
			out[k] = v
		}
	}
	return out
}

func snapshotLuaGlobals(L *lua.LState) map[lua.LValue]lua.LValue {
	if L == nil || L.G == nil || L.G.Global == nil {
		return nil
	}
	snap := map[lua.LValue]lua.LValue{}
	L.G.Global.ForEach(func(k, v lua.LValue) {
		snap[k] = cloneLuaValue(L, v, 0)
	})
	return snap
}

func restoreLuaGlobals(L *lua.LState, snap map[lua.LValue]lua.LValue) {
	if L == nil || L.G == nil || L.G.Global == nil || snap == nil {
		return
	}
	var toDelete []lua.LValue
	L.G.Global.ForEach(func(k, _ lua.LValue) {
		if _, ok := snap[k]; !ok {
			toDelete = append(toDelete, k)
		}
	})
	for _, k := range toDelete {
		L.G.Global.RawSet(k, lua.LNil)
	}
	for k, v := range snap {
		L.G.Global.RawSet(k, v)
	}
}

func cloneLuaValue(L *lua.LState, v lua.LValue, depth int) lua.LValue {
	if depth > 8 {
		return v
	}
	t, ok := v.(*lua.LTable)
	if !ok || t == L.G.Global {
		return v
	}
	copyTbl := L.NewTable()
	t.ForEach(func(k, val lua.LValue) {
		copyTbl.RawSet(k, cloneLuaValue(L, val, depth+1))
	})
	return copyTbl
}
