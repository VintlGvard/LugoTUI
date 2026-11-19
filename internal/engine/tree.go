package engine

import (
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/VintlGvard/LugoTUI/internal/core"
)

type Node = core.Node
type nodeError = core.NodeError
type WidgetState = core.WidgetState
type Operation = core.Operation

const (
	MaxNodes = core.MaxNodes
	maxNodes = core.MaxNodes
	MaxText  = core.MaxText
	maxText  = core.MaxText
)

func str(n *Node, key, fallback string) string          { return core.Str(n, key, fallback) }
func floatVal(v any) (float64, bool)                    { return core.FloatVal(v) }
func num(n *Node, key string, fallback float64) float64 { return core.Num(n, key, fallback) }
func flag(n *Node, key string) bool                     { return core.Flag(n, key) }
func focusable(n *Node) bool                            { return core.Focusable(n) }
func cloneValue(v any, depth int) (any, error)          { return core.CloneValue(v) }
func CopyValue(v any) any                               { return core.CopyValue(v) }
func cloneNode(n *Node) *Node                           { return core.CloneNode(n) }
func findNode(root *Node, id string) *Node              { return core.FindNode(root, id) }
func ordered(n *Node) iter.Seq[*Node]                   { return core.Ordered(n) }
func finite(v float64) bool                             { return core.Finite(v) }
func ValidateModel(data map[string]any) error           { return core.ValidateModel(data) }
func AutoAssignIDs(root *Node)                          { core.AutoAssignIDs(root) }

func validateTree(root *Node) (map[string]*Node, error) {
	return core.ValidateTree(root, validateCollection, validatePrimitive, validateEditor)
}

func (r *Runtime) validateActions() error {
	for n := range ordered(r.Root) {
		for _, key := range []string{"on_click", "on_select", "on_submit", "on_change", "on_navigate", "on_timeout", "on_key"} {
			if action := str(n, key, ""); action != "" {
				if !r.hasAction(action) {
					return &nodeError{Node: n, Property: key, Err: errors.New("unknown action: " + action)}
				}
			}
		}
		if km, ok := n.Props["keymap"].(map[string]any); ok {
			for key, action := range km {
				if s, ok := action.(string); ok && s != "" {
					if !r.hasAction(s) {
						return &nodeError{Node: n, Property: "keymap", Err: fmt.Errorf("unknown action: %s (key %q)", s, key)}
					}
				}
			}
		}
		if n.Type == "help" {
			keys, _ := n.Props["keys"].([]any)
			for _, value := range keys {
				binding, _ := value.(map[string]any)
				action, _ := binding["action"].(string)
				if action != "" && !r.hasAction(action) {
					return &nodeError{Node: n, Property: "keys", Err: errors.New("unknown action: " + action)}
				}
			}
		}
	}
	for key, action := range r.keymap {
		if !r.hasAction(action) {
			return fmt.Errorf("unknown keymap action: %s (key %q)", action, key)
		}
	}
	return nil
}

func wrapInlineAction(raw any) (GoActionFunc, bool) {
	switch fn := raw.(type) {
	case GoActionFunc:
		if fn == nil {
			return nil, true
		}
		return fn, true
	case func() error:
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, _ map[string]any) error { return fn() }, true
	case func():
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, _ map[string]any) error { fn(); return nil }, true
	case func(map[string]any) error:
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, payload map[string]any) error { return fn(payload) }, true
	case func(any) error:
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, payload map[string]any) error { return fn(payload) }, true
	case func(string):
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, payload map[string]any) error {
			fn(core.ExtractPayloadText(payload))
			return nil
		}, true
	case func(string) error:
		if fn == nil {
			return nil, true
		}
		return func(_ *Runtime, payload map[string]any) error { return fn(core.ExtractPayloadText(payload)) }, true
	default:
		return nil, false
	}
}

func (r *Runtime) bindInlineActions(root *Node) error {
	if root == nil {
		return nil
	}
	if r.GoActions == nil {
		r.GoActions = map[string]GoActionFunc{}
	}
	seen := map[*Node]bool{}
	var walk func(*Node, int) error
	walk = func(n *Node, depth int) error {
		if n == nil || depth > core.MaxDepth || seen[n] {
			return nil
		}
		seen[n] = true
		if len(n.InlineActions) > 0 {
			if n.Props == nil {
				n.Props = map[string]any{}
			}
			for prop, raw := range n.InlineActions {
				if raw == nil {
					continue
				}
				fn, supported := wrapInlineAction(raw)
				if !supported {
					return &nodeError{Node: n, Property: prop, Err: fmt.Errorf("unsupported inline action type %T", raw)}
				}
				if fn == nil {
					continue
				}
				if strings.HasPrefix(prop, "keymap:") {
					key := strings.TrimPrefix(prop, "keymap:")
					actName := "__inline:" + n.ID + ":key:" + key
					km, ok := n.Props["keymap"].(map[string]any)
					if !ok || km == nil {
						km = map[string]any{}
						n.Props["keymap"] = km
					}
					km[key] = actName
					r.GoActions[actName] = fn
				} else {
					actName := "__inline:" + n.ID + ":" + prop
					n.Props[prop] = actName
					r.GoActions[actName] = fn
				}
			}
		}
		for _, c := range n.Children {
			if err := walk(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root, 0)
}
