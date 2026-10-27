package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type Node struct {
	Type, ID            string
	Props               map[string]any
	Children            []*Node
	Rect, Clip          Rect
	layoutText          string
	layoutBound         bool
	intrinsicCache      [2]float64
	intrinsicValid      bool
	measuredWidth       int
	measuredHeightCache float64
	measuredValid       bool
}

type nodeError struct {
	Node     *Node
	Property string
	Err      error
}

func (e *nodeError) Error() string {
	context := "node <nil>"
	if e.Node != nil {
		context = fmt.Sprintf("node %q (type %q)", e.Node.ID, e.Node.Type)
	}
	if e.Property != "" {
		context += fmt.Sprintf(" property %q", e.Property)
	}
	return context + ": " + e.Err.Error()
}

func (e *nodeError) Unwrap() error { return e.Err }

type WidgetState struct {
	Value                            string
	Cursor, Anchor, Scroll, Selected int
	Undo, Redo                       []string
	collection                       *collectionState
	primitive                        *primitiveState
	indexedValue                     string
	indexedWidth                     int
	indexedMultiline                 bool
	indexed                          bool
	clusters                         []string
	positions                        [][2]int
}

type Operation struct {
	ID      string
	Props   map[string]any
	Tree    *Node
	Replace bool
}

func str(n *Node, key, fallback string) string {
	if v, ok := n.Props[key].(string); ok {
		return v
	}
	return fallback
}

func num(n *Node, key string, fallback float64) float64 {
	if v, ok := n.Props[key].(float64); ok {
		return v
	}
	return fallback
}

func flag(n *Node, key string) bool { v, _ := n.Props[key].(bool); return v }

func focusable(n *Node) bool {
	return !flag(n, "disabled") && !flag(n, "hidden") && (n.Type == "input" || n.Type == "textarea" || n.Type == "button" || n.Type == "list" || n.Type == "scroll" || n.Type == "table" || n.Type == "viewport" || n.Type == "tree" || n.Type == "filepicker" || n.Type == "paginator" || n.Type == "timer" || n.Type == "stopwatch" || n.Type == "help" || n.Type == "cursor")
}

func cloneValue(v any, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("value nesting exceeds 32")
	}
	switch value := v.(type) {
	case map[string]any:
		copy := make(map[string]any, len(value))
		for key, item := range value {
			cloned, err := cloneValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			copy[key] = cloned
		}
		return copy, nil
	case []any:
		copy := make([]any, len(value))
		for i, item := range value {
			cloned, err := cloneValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			copy[i] = cloned
		}
		return copy, nil
	case []string:
		return append([]string(nil), value...), nil
	case nil, string, bool, int, int64, float64:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported Go type %T", v)
	}
}

func cloneNode(n *Node) *Node {
	c := *n
	c.Props = maps.Clone(n.Props)
	if c.Props == nil {
		c.Props = make(map[string]any)
	}
	c.Children = make([]*Node, len(n.Children))
	for i, ch := range n.Children {
		c.Children[i] = cloneNode(ch)
	}
	return &c
}

func findNode(root *Node, id string) *Node {
	if root == nil || id == "" {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, c := range root.Children {
		if found := findNode(c, id); found != nil {
			return found
		}
	}
	return nil
}

func ordered(n *Node) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		var walk func(*Node) bool
		walk = func(n *Node) bool {
			if !yield(n) {
				return false
			}
			for _, child := range n.Children {
				if !walk(child) {
					return false
				}
			}
			return true
		}
		walk(n)
	}
}

const MaxNodes = 4096
const maxNodes = MaxNodes

const MaxText = 1 << 20
const maxText = MaxText

func validateTree(root *Node) (map[string]*Node, error) {
	ids := map[string]*Node{}
	count, total := 0, 0
	var walk func(*Node, int) error
	walk = func(n *Node, d int) (err error) {
		property := ""
		defer func() {
			var contextual *nodeError
			if err != nil && !errors.As(err, &contextual) {
				err = &nodeError{Node: n, Property: property, Err: err}
			}
		}()
		if n == nil {
			return errors.New("nil node")
		}
		count++
		if count > maxNodes || d > 64 {
			return errors.New("tree exceeds limits")
		}
		switch n.Type {
		case "box", "row", "column", "panel", "text", "input", "textarea", "button", "list", "scroll", "overlay", "table", "viewport", "tree", "filepicker", "spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor":
		default:
			return fmt.Errorf("unknown component %q", n.Type)
		}
		if n.ID != "" {
			if len(n.ID) > 128 {
				return errors.New("id exceeds 128 bytes")
			}
			if ids[n.ID] != nil {
				return fmt.Errorf("duplicate id %q", n.ID)
			}
			ids[n.ID] = n
		}
		if (focusable(n) || flag(n, "modal") || n.Type == "scroll" || n.Type == "spinner" || n.Type == "progress" || n.Type == "cursor") && n.ID == "" {
			return fmt.Errorf("%s requires id", n.Type)
		}
		for k, v := range n.Props {
			property = k
			switch k {
			case "width", "height", "basis", "min_width", "max_width", "min_height", "max_height", "top", "right", "bottom", "left":
				if s, ok := v.(string); ok {
					if s != "auto" {
						if !strings.HasSuffix(s, "%") {
							return fmt.Errorf("%s: invalid %s", n.ID, k)
						}
						f, e := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
						if e != nil || !finite(f) || f < 0 || f > 10000 {
							return fmt.Errorf("invalid percentage %s", s)
						}
					}
				} else if f, ok := v.(float64); !ok || !finite(f) || f < 0 || f > 1e6 {
					return fmt.Errorf("invalid %s", k)
				}
			case "grow", "shrink", "padding", "margin", "gap":
				if f, ok := v.(float64); !ok || !finite(f) || f < 0 || f > 1e6 {
					return fmt.Errorf("invalid %s", k)
				}
			case "direction":
				if v != "row" && v != "column" {
					return errors.New("invalid direction")
				}
			case "align", "align_self":
				if v != "start" && v != "center" && v != "end" && v != "stretch" {
					return errors.New("invalid alignment")
				}
			case "justify":
				if v != "start" && v != "center" && v != "end" && v != "space-between" && v != "space-around" {
					return errors.New("invalid justify")
				}
			case "position":
				if v != "absolute" && v != "relative" {
					return errors.New("invalid position")
				}
			case "text", "title", "bind", "source", "on_submit", "on_change", "on_select", "on_click", "fg", "bg", "wrap":
				if _, ok := v.(string); !ok {
					return fmt.Errorf("%s must be string", k)
				}
			case "border", "hidden", "disabled", "bold", "modal":
				if _, ok := v.(bool); !ok {
					return fmt.Errorf("%s must be boolean", k)
				}
			case "z_index":
				if f, ok := v.(float64); !ok || !finite(f) || math.Abs(f) > 1e6 {
					return errors.New("invalid z_index")
				}
			case "spans":
				a, ok := v.([]any)
				if !ok {
					return errors.New("spans must be array")
				}
				for _, v := range a {
					m, ok := v.(map[string]any)
					if !ok {
						return errors.New("span must be object")
					}
					for key, value := range m {
						switch key {
						case "text", "fg", "bg":
							s, ok := value.(string)
							if !ok {
								return errors.New("span string required")
							}
							total += len(s)
						case "bold":
							if _, ok := value.(bool); !ok {
								return errors.New("span bold must be boolean")
							}
						default:
							return fmt.Errorf("unknown span property %s", key)
						}
					}
				}
			case "items":
				if _, ok := v.([]any); !ok {
					return errors.New("items must be array")
				}
			case "columns", "rows", "show_header", "soft_wrap", "highlights", "filterable", "show_title", "show_status", "show_help", "show_pagination", "page_size", "query", "on_navigate", "allowed_extensions", "show_hidden", "file_allowed", "dir_allowed", "frames", "preset", "interval", "running", "percent", "animated", "duration", "gradient", "show_percentage", "fill", "empty", "page", "per_page", "total", "mode", "active_dot", "inactive_dot", "reset_key", "on_timeout", "keys", "full", "separator", "echo", "echo_character", "placeholder", "prompt", "char_limit", "suggestions", "show_suggestions", "validation_pattern", "validation_error", "line_numbers", "cursor_mode", "cursor_blink_speed", "char", "blink_speed", "show_size", "show_permissions", "show_line_numbers", "percent_format", "open_char", "close_char", "cursor_char", "branch_indent", "subsecond", "format":
			default:
				return fmt.Errorf("unknown property %q", k)
			}
			if s, ok := v.(string); ok {
				total += len(s)
			}
		}
		property = ""
		for _, validate := range []func(*Node) error{validateCollection, validatePrimitive, validateEditor} {
			if err := validate(n); err != nil {
				return err
			}
		}
		if total > maxText {
			return errors.New("tree text exceeds 1 MiB")
		}
		for _, c := range n.Children {
			if e := walk(c, d+1); e != nil {
				return e
			}
		}
		return nil
	}
	return ids, walk(root, 0)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func ValidateModel(data map[string]any) error {
	if len(data) > 1024 {
		return errors.New("model exceeds 1024 keys")
	}
	b, e := json.Marshal(data)
	if e != nil {
		return e
	}
	if len(b) > maxText {
		return errors.New("model exceeds 1 MiB")
	}
	return nil
}

func (r *Runtime) validateActions() error {
	for n := range ordered(r.Root) {
		for _, key := range []string{"on_click", "on_select", "on_submit", "on_change", "on_navigate", "on_timeout"} {
			if action := str(n, key, ""); action != "" {
				if r.actions == nil || r.actions.RawGetString(action).Type() != lua.LTFunction {
					return &nodeError{Node: n, Property: key, Err: errors.New("unknown action: " + action)}
				}
			}
		}
		if n.Type == "help" {
			keys, _ := n.Props["keys"].([]any)
			for _, value := range keys {
				binding, _ := value.(map[string]any)
				action, _ := binding["action"].(string)
				if action != "" && (r.actions == nil || r.actions.RawGetString(action).Type() != lua.LTFunction) {
					return &nodeError{Node: n, Property: "keys", Err: errors.New("unknown action: " + action)}
				}
			}
		}
	}
	for key, action := range r.keymap {
		if r.actions == nil || r.actions.RawGetString(action).Type() != lua.LTFunction {
			return fmt.Errorf("unknown keymap action: %s (key %q)", action, key)
		}
	}
	return nil
}
