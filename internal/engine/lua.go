package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func loadRuntime(source, chunkName string, old *Runtime) (*Runtime, error) {
	if len(source) > maxText {
		return nil, errors.New("script exceeds 1 MiB")
	}
	if chunkName == "" {
		if old != nil && old.ChunkName != "" {
			chunkName = old.ChunkName
		} else {
			chunkName = "<string>"
		}
	}
	r := &Runtime{Nodes: map[string]*Node{}, State: map[string]*WidgetState{}, Data: map[string]any{}, ChunkName: chunkName, AltScreen: true}
	if old != nil {
		r.Data = cloneData(old.Data)
		r.Revision = old.Revision + 1
		r.Theme = old.Theme
		r.Clipboard = old.Clipboard
		r.StateObservers = old.StateObservers
		r.Inline = old.Inline
		r.InlineHeight = old.InlineHeight
		r.AltScreen = old.AltScreen
	}
	L := lua.NewState(lua.Options{SkipOpenLibs: true, CallStackSize: 128, RegistrySize: 1024, RegistryMaxSize: 8192, MinimizeStackMemory: true})
	r.L = L
	success := false
	defer func() {
		if !success {
			L.Close()
		}
	}()
	for _, open := range []lua.LGFunction{lua.OpenBase, lua.OpenTable, lua.OpenString, lua.OpenMath} {
		open(L)
	}
	for _, name := range []string{"dofile", "loadfile", "load", "loadstring", "collectgarbage", "print"} {
		L.SetGlobal(name, lua.LNil)
	}
	if strTbl, ok := L.GetGlobal("string").(*lua.LTable); ok {
		L.SetField(strTbl, "rep", L.NewFunction(func(L *lua.LState) int {
			s := L.CheckString(1)
			n := L.CheckInt(2)
			sep := L.OptString(3, "")
			if n <= 0 {
				L.Push(lua.LString(""))
				return 1
			}
			total := int64(len(s))*int64(n) + int64(len(sep))*int64(n-1)
			if int64(len(s)) > maxText || int64(n) > maxText || total > maxText || total < 0 {
				L.RaiseError("string.rep exceeds 1 MiB")
			}
			if sep == "" {
				L.Push(lua.LString(strings.Repeat(s, n)))
				return 1
			}
			var b strings.Builder
			b.Grow(int(total))
			for i := 0; i < n; i++ {
				if i > 0 {
					b.WriteString(sep)
				}
				b.WriteString(s)
			}
			L.Push(lua.LString(b.String()))
			return 1
		}))
	}
	ui := L.NewTable()
	for _, kind := range []string{"box", "row", "column", "text", "input", "textarea", "button", "list", "scroll", "overlay", "panel", "table", "viewport", "tree", "filepicker", "spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor"} {
		kind := kind
		L.SetField(ui, kind, L.NewFunction(func(L *lua.LState) int {
			t := L.CheckTable(1)
			t.RawSetString("type", lua.LString(kind))
			L.Push(t)
			return 1
		}))
	}
	L.SetField(ui, "textinput", L.GetField(ui, "input"))
	L.SetField(ui, "key", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.CheckTable(1))
		return 1
	}))
	stagePatch := func(id string, tbl *lua.LTable) {
		v, err := luaPlain(tbl, 0)
		if err != nil {
			L.RaiseError("operations[%d] patch %q: %s", len(r.Staged)+1, id, err)
		}
		props, ok := v.(map[string]any)
		if !ok {
			L.RaiseError("operations[%d] patch %q: properties must be object", len(r.Staged)+1, id)
		}
		for _, key := range []string{"items", "spans", "columns", "rows", "frames", "keys", "highlights", "suggestions", "allowed_extensions"} {
			if m, ok := props[key].(map[string]any); ok && len(m) == 0 {
				props[key] = []any{}
			}
		}
		r.Staged = append(r.Staged, Operation{ID: id, Props: props})
		if len(r.Staged) > maxNodes {
			L.RaiseError("too many operations")
		}
	}
	patchFn := L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() == 1 {
			tbl := L.CheckTable(1)
			var iterErr error
			tbl.ForEach(func(k, v lua.LValue) {
				if iterErr != nil {
					return
				}
				propsTbl, ok := v.(*lua.LTable)
				if !ok {
					iterErr = fmt.Errorf("operations[%d] batch patch %q: properties must be object", len(r.Staged)+1, k.String())
					return
				}
				stagePatch(k.String(), propsTbl)
			})
			if iterErr != nil {
				L.RaiseError("%s", iterErr)
			}
			return 0
		}
		id := L.CheckString(1)
		stagePatch(id, L.CheckTable(2))
		return 0
	})
	L.SetField(ui, "patch", patchFn)
	L.SetField(ui, "patch_all", patchFn)
	L.SetField(ui, "replace", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		tree, err := parseNode(L.CheckTable(2), 0)
		if err != nil {
			L.RaiseError("operations[%d] replace %q: %s", len(r.Staged)+1, id, err)
		}
		r.Staged = append(r.Staged, Operation{ID: id, Tree: tree, Replace: true})
		if len(r.Staged) > maxNodes {
			L.RaiseError("too many operations")
		}
		return 0
	}))
	L.SetField(ui, "focus", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		r.stagedFocus = &id
		return 0
	}))
	L.SetField(ui, "focused", L.NewFunction(func(L *lua.LState) int {
		focus := r.Focus
		if r.stagedFocus != nil {
			focus = *r.stagedFocus
		}
		if focus == "" {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(focus))
		}
		return 1
	}))
	L.SetField(ui, "focus_direction", L.NewFunction(func(L *lua.LState) int {
		dir := strings.ToLower(L.CheckString(1))
		nr := r.navRuntime()
		nr.ReconcileFocus()
		curr := nr.Nodes[nr.Focus]
		if r.stagedFocus != nil {
			curr = nr.Nodes[*r.stagedFocus]
		}
		if curr == nil {
			L.Push(lua.LNil)
			return 1
		}
		target := nr.findDirectionNode(curr, dir)
		if target != nil {
			r.stagedFocus = &target.ID
			L.Push(lua.LString(target.ID))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))
	L.SetField(ui, "focus_next", L.NewFunction(func(L *lua.LState) int {
		nr := r.navRuntime()
		target := nr.nextFocusable(false)
		if target != "" {
			r.stagedFocus = &target
			L.Push(lua.LString(target))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))
	L.SetField(ui, "focus_prev", L.NewFunction(func(L *lua.LState) int {
		nr := r.navRuntime()
		target := nr.nextFocusable(true)
		if target != "" {
			r.stagedFocus = &target
			L.Push(lua.LString(target))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))
	host := L.NewTable()
	L.SetField(host, "dispatch", L.NewFunction(func(L *lua.LState) int {
		action := L.CheckString(1)
		v, err := luaPlain(L.Get(2), 0)
		if err != nil {
			L.RaiseError("%s", err)
		}
		replace := false
		if L.GetTop() >= 3 {
			if b, ok := L.Get(3).(lua.LBool); ok {
				replace = bool(b)
			} else if tbl, ok := L.Get(3).(*lua.LTable); ok {
				if d := tbl.RawGetString("debounce"); d != lua.LNil && d != lua.LFalse {
					replace = true
				} else if rep := tbl.RawGetString("replace"); rep != lua.LNil && rep != lua.LFalse {
					replace = true
				}
			}
		} else if m, ok := v.(map[string]any); ok {
			if d, ok := m["debounce"]; ok && d != false && d != nil {
				replace = true
			}
		}
		if replace {
			for i := len(r.Events) - 1; i >= 0; i-- {
				if r.Events[i].Action == action {
					r.Events[i].Payload = v
					return 0
				}
			}
		}
		if len(r.Events) >= 256 {
			L.RaiseError("host event queue full")
		}
		r.Events = append(r.Events, HostEvent{action, v})
		return 0
	}))
	L.SetField(ui, "get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		v, err := toLua(L, r.Data[key], "model."+key)
		if err != nil {
			L.RaiseError("%s", err)
		}
		L.Push(v)
		return 1
	}))
	L.SetField(ui, "set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		v, e := luaPlain(L.Get(2), 0)
		if e != nil {
			L.RaiseError("%s", e)
		}
		r.Data[key] = v
		return 0
	}))
	L.SetField(ui, "clipboard_get", L.NewFunction(func(L *lua.LState) int {
		text, err := r.ClipboardAdapter().Read()
		if err != nil {
			L.RaiseError("%s", err)
		}
		L.Push(lua.LString(text))
		return 1
	}))
	L.SetField(ui, "clipboard_set", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		if err := r.ClipboardAdapter().Write(text); err != nil {
			L.RaiseError("%s", err)
		}
		return 0
	}))
	L.SetField(ui, "enter_alt_screen", L.NewFunction(func(_ *lua.LState) int {
		r.AltScreen = true
		r.Inline = false
		if len(r.Events) < 256 {
			r.Events = append(r.Events, HostEvent{Action: "__alt_screen", Payload: true})
		}
		return 0
	}))
	L.SetField(ui, "exit_alt_screen", L.NewFunction(func(_ *lua.LState) int {
		r.AltScreen = false
		r.Inline = true
		if len(r.Events) < 256 {
			r.Events = append(r.Events, HostEvent{Action: "__alt_screen", Payload: false})
		}
		return 0
	}))
	L.SetGlobal("ui", ui)
	L.SetGlobal("host", host)
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int {
		if L.CheckString(1) != "ui" {
			L.RaiseError("only embedded ui module available")
		}
		L.Push(ui)
		return 1
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	L.SetContext(ctx)
	fn, err := L.Load(strings.NewReader(source), chunkName)
	if err != nil {
		return nil, err
	}
	if err = L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return nil, err
	}
	spec, ok := L.Get(-1).(*lua.LTable)
	if !ok {
		return nil, errors.New("script must return table")
	}
	L.Pop(1)
	if version := spec.RawGetString("api_version"); version != lua.LNil && version != lua.LNumber(1) {
		return nil, errors.New("unsupported api_version")
	}
	view := spec.RawGetString("view")
	if f, ok := view.(*lua.LFunction); ok {
		if err = L.CallByParam(lua.P{Fn: f, NRet: 1, Protect: true}); err != nil {
			return nil, err
		}
		view = L.Get(-1)
		L.Pop(1)
	}
	t, ok := view.(*lua.LTable)
	if !ok {
		return nil, errors.New("view must be table or function returning table")
	}
	r.Root, err = parseNode(t, 0)
	if err != nil {
		return nil, err
	}
	r.Nodes, err = validateTree(r.Root)
	if err != nil {
		return nil, err
	}
	r.actions, _ = spec.RawGetString("actions").(*lua.LTable)
	r.keymap = map[string]string{}
	if keys, ok := spec.RawGetString("keymap").(*lua.LTable); ok {
		var keyErr error
		keys.ForEach(func(k, v lua.LValue) {
			ks, kok := k.(lua.LString)
			vs, vok := v.(lua.LString)
			if !kok || !vok {
				keyErr = errors.New("keymap requires string keys and actions")
				return
			}
			r.keymap[string(ks)] = string(vs)
		})
		if keyErr != nil {
			return nil, keyErr
		}
	}
	if err = r.validateActions(); err != nil {
		return nil, err
	}
	if old != nil {
		r.modalID = old.modalID
		r.PreviousFocus = old.PreviousFocus
		r.spatialNav = old.spatialNav
	}
	if sn := spec.RawGetString("spatial_nav"); sn.Type() == lua.LTBool {
		r.spatialNav = bool(sn.(lua.LBool))
	}
	if len(r.Staged) > 0 || r.stagedFocus != nil {
		return nil, errors.New("UI mutation during initialization forbidden")
	}
	if old != nil {
		r.Events = append([]HostEvent(nil), old.Events...)
	} else {
		r.Events = nil
	}
	r.restore(old)
	if err = ValidateModel(r.Data); err != nil {
		return nil, err
	}
	L.RemoveContext()
	success = true
	return r, nil
}

const maxLuaValues = maxNodes * 64

type luaBudget struct {
	nodes, values, text int
}

func (b *luaBudget) take(v lua.LValue) error {
	if b.values >= maxLuaValues {
		return errors.New("expanded values exceed limit")
	}
	b.values++
	if s, ok := v.(lua.LString); ok {
		if len(s) > maxText-b.text {
			return errors.New("text exceeds 1 MiB")
		}
		b.text += len(s)
	}
	return nil
}

func luaPlain(v lua.LValue, depth int) (any, error) {
	return luaPlainBudget(v, depth, &luaBudget{})
}

func luaPlainBudget(v lua.LValue, depth int, budget *luaBudget) (any, error) {
	if err := budget.take(v); err != nil {
		return nil, err
	}
	if depth > 32 {
		return nil, errors.New("value nesting exceeds 32")
	}
	switch x := v.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(x), nil
	case lua.LString:
		return string(x), nil
	case lua.LNumber:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return nil, errors.New("nonfinite number")
		}
		return float64(x), nil
	case *lua.LTable:
		if x.Len() > maxNodes {
			return nil, errors.New("table too large")
		}
		if x.Len() > 0 {
			count := 0
			var err error
			x.ForEach(func(k, _ lua.LValue) {
				if err == nil {
					err = budget.take(k)
				}
				count++
			})
			if err != nil {
				return nil, err
			}
			if count != x.Len() {
				return nil, errors.New("mixed or sparse table")
			}
			if x.Len() > maxLuaValues-budget.values {
				return nil, errors.New("expanded values exceed limit")
			}
			a := make([]any, x.Len())
			for i := range a {
				v, e := luaPlainBudget(x.RawGetInt(i+1), depth+1, budget)
				if e != nil {
					return nil, e
				}
				a[i] = v
			}
			return a, nil
		}
		m := map[string]any{}
		var err error
		x.ForEach(func(k, v lua.LValue) {
			if err != nil {
				return
			}
			s, ok := k.(lua.LString)
			if !ok {
				err = errors.New("string keys required")
				return
			}
			if len(m) >= maxNodes {
				err = errors.New("table too large")
				return
			}
			if err = budget.take(k); err != nil {
				return
			}
			var value any
			value, err = luaPlainBudget(v, depth+1, budget)
			if err == nil {
				m[string(s)] = value
			}
		})
		return m, err
	default:
		return nil, fmt.Errorf("unsupported value %s", v.Type())
	}
}

func parseNode(t *lua.LTable, depth int) (*Node, error) {
	return parseNodeBudget(t, depth, &luaBudget{})
}

func parseNodeBudget(t *lua.LTable, depth int, budget *luaBudget) (*Node, error) {
	if depth > 64 {
		return nil, errors.New("tree depth exceeds 64")
	}
	if budget.nodes >= maxNodes {
		return nil, errors.New("tree exceeds 4096 nodes")
	}
	budget.nodes++
	if err := budget.take(t); err != nil {
		return nil, err
	}
	n := &Node{Props: map[string]any{}}
	kind, ok := t.RawGetString("type").(lua.LString)
	if !ok {
		return nil, fmt.Errorf("node id=%q: type must be string", t.RawGetString("id"))
	}
	if err := budget.take(kind); err != nil {
		return nil, err
	}
	n.Type = string(kind)
	if id := t.RawGetString("id"); id != lua.LNil {
		s, ok := id.(lua.LString)
		if !ok {
			return nil, fmt.Errorf("node type=%q: id must be string", n.Type)
		}
		if err := budget.take(s); err != nil {
			return nil, err
		}
		n.ID = string(s)
	}
	var err error
	t.ForEach(func(k, v lua.LValue) {
		if err != nil {
			return
		}
		if err = budget.take(k); err != nil {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			return
		}
		switch string(key) {
		case "type", "id", "children":
		default:
			if key == "items" || key == "spans" || key == "columns" || key == "rows" || key == "frames" || key == "keys" || key == "highlights" || key == "suggestions" || key == "allowed_extensions" {
				if table, ok := v.(*lua.LTable); ok && table.Len() == 0 {
					empty := true
					table.ForEach(func(lua.LValue, lua.LValue) { empty = false })
					if empty {
						if err = budget.take(table); err == nil {
							n.Props[string(key)] = []any{}
						}
						return
					}
				}
			}
			n.Props[string(key)], err = luaPlainBudget(v, 0, budget)
			if err != nil {
				err = fmt.Errorf("node id=%q type=%q property %q: %w", n.ID, n.Type, key, err)
			}
		}
	})
	if err != nil {
		return nil, err
	}
	children := t
	if value := t.RawGetString("children"); value != lua.LNil {
		c, ok := value.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("node id=%q type=%q property children: must be table", n.ID, n.Type)
		}
		if err = budget.take(c); err != nil {
			return nil, err
		}
		children = c
		c.ForEach(func(key, _ lua.LValue) {
			if err != nil {
				return
			}
			if err = budget.take(key); err != nil {
				return
			}
			i, ok := key.(lua.LNumber)
			if !ok || i < 1 || i > lua.LNumber(c.Len()) || math.Trunc(float64(i)) != float64(i) {
				err = fmt.Errorf("node id=%q type=%q property children: expected dense array, invalid key %s", n.ID, n.Type, key)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	if children.Len() > maxNodes {
		return nil, fmt.Errorf("node id=%q type=%q: too many children", n.ID, n.Type)
	}
	if children.Len() > maxNodes-budget.nodes {
		return nil, errors.New("tree exceeds 4096 nodes")
	}
	for i := 1; i <= children.Len(); i++ {
		c, ok := children.RawGetInt(i).(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("node id=%q type=%q children[%d]: child must be node table", n.ID, n.Type, i)
		}
		node, e := parseNodeBudget(c, depth+1, budget)
		if e != nil {
			return nil, fmt.Errorf("node id=%q type=%q children[%d]: %w", n.ID, n.Type, i, e)
		}
		n.Children = append(n.Children, node)
	}
	return n, nil
}

func toLua(L *lua.LState, v any, path string) (lua.LValue, error) {
	return toLuaDepth(L, v, path, 0)
}

func toLuaDepth(L *lua.LState, v any, path string, depth int) (lua.LValue, error) {
	if depth > 32 {
		return nil, fmt.Errorf("%s: value nesting exceeds 32", path)
	}
	switch x := v.(type) {
	case nil:
		return lua.LNil, nil
	case string:
		return lua.LString(x), nil
	case bool:
		return lua.LBool(x), nil
	case int:
		return toLuaDepth(L, int64(x), path, depth)
	case int64:
		if x < -(1<<53) || x > 1<<53 {
			return nil, fmt.Errorf("%s: integer exceeds exact Lua range", path)
		}
		return lua.LNumber(x), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("%s: nonfinite number", path)
		}
		return lua.LNumber(x), nil
	case map[string]any:
		t := L.NewTable()
		for k, v := range x {
			value, err := toLuaDepth(L, v, path+"."+k, depth+1)
			if err != nil {
				return nil, err
			}
			t.RawSetString(k, value)
		}
		return t, nil
	case []any:
		t := L.NewTable()
		for i, v := range x {
			value, err := toLuaDepth(L, v, fmt.Sprintf("%s[%d]", path, i+1), depth+1)
			if err != nil {
				return nil, err
			}
			t.RawSetInt(i+1, value)
		}
		return t, nil
	case []string:
		t := L.NewTable()
		for i, v := range x {
			t.RawSetInt(i+1, lua.LString(v))
		}
		return t, nil
	default:
		return nil, fmt.Errorf("%s: unsupported Go type %T", path, v)
	}
}
